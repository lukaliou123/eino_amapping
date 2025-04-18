package amap

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

// VectorConfig 存储向量配置信息
type VectorConfig struct {
	// 向量化模型API URL
	ModelEndpoint string
	// API密钥(如果需要)
	APIKey string
	// 数据存储路径
	StoragePath string
	// 是否启用向量化
	Enabled bool
	// Redis存储配置，如果为nil则不存储到Redis
	RedisConfig *RedisVectorStoreConfig
	// 是否启用Redis存储
	EnableRedisStore bool
	// 数据过期时间（天），设置为RedisConfig.TTLDays，0表示使用默认值
	TTLDays int
}

// DataVector 表示各种高德数据向量化后的结构
type DataVector struct {
	// 原始数据
	OriginalData map[string]interface{} `json:"original_data"`
	// 向量表示
	Vector []float32 `json:"vector"`
	// 元数据信息
	Metadata struct {
		// 工具名称(如maps_text_search)
		ToolName string `json:"tool_name"`
		// 数据类型(如POI、route、weather)
		DataType string `json:"data_type"`
		// 地理范围(城市、区域等)
		GeoInfo string `json:"geo_info"`
		// 数据获取时间
		Timestamp int64 `json:"timestamp"`
		// 数据可信度
		Confidence float32 `json:"confidence"`
		// 内容摘要
		ContentSummary string `json:"content_summary"`
		// 数据ID
		ID string `json:"id"`
		// 额外属性
		Attributes map[string]interface{} `json:"attributes"`
	} `json:"metadata"`
}

// DataVectorStoreManager 数据向量存储管理器接口
type DataVectorStoreManager interface {
	// 存储数据向量
	StoreDataVector(ctx context.Context, dataVector *DataVector) error
	// 关闭存储
	Close() error
}

var (
	// 默认向量配置
	DefaultVectorConfig = VectorConfig{
		ModelEndpoint:    os.Getenv("ARK_EMBEDDING_MODEL"),
		APIKey:           os.Getenv("ARK_API_KEY"),
		StoragePath:      "data/amap_vectors",
		Enabled:          false,
		EnableRedisStore: false,
		TTLDays:          3, // 默认3天过期
	}

	// 全局向量配置
	vectorConfig = DefaultVectorConfig

	// Redis存储器
	redisStore *AmapRedisStore
)

// 数据类型常量
const (
	DataTypePOI        = "poi"
	DataTypeWeather    = "weather"
	DataTypeRoute      = "route"
	DataTypeGeo        = "geo"
	DataTypeRegeocode  = "regeocode"
	DataTypeIPLocation = "ip_location"
	DataTypeDistance   = "distance"
)

// 工具名称到数据类型映射
var toolToDataTypeMap = map[string]string{
	"maps_text_search":                  DataTypePOI,
	"maps_around_search":                DataTypePOI,
	"maps_search_detail":                DataTypePOI,
	"maps_geo":                          DataTypeGeo,
	"maps_regeocode":                    DataTypeRegeocode,
	"maps_weather":                      DataTypeWeather,
	"maps_direction_driving":            DataTypeRoute,
	"maps_direction_walking":            DataTypeRoute,
	"maps_direction_bicycling":          DataTypeRoute,
	"maps_direction_transit_integrated": DataTypeRoute,
	"maps_distance":                     DataTypeDistance,
	"maps_ip_location":                  DataTypeIPLocation,
}

// SetVectorConfig 设置向量化配置
func SetVectorConfig(config VectorConfig) {
	vectorConfig = config
	// 确保存储目录存在
	if vectorConfig.Enabled && vectorConfig.StoragePath != "" {
		os.MkdirAll(vectorConfig.StoragePath, 0755)
	}

	// 初始化Redis存储
	if vectorConfig.Enabled && vectorConfig.EnableRedisStore && vectorConfig.RedisConfig != nil {
		// 如果配置中没有指定TTL，使用VectorConfig中的TTLDays
		if vectorConfig.RedisConfig.TTLDays <= 0 && vectorConfig.TTLDays > 0 {
			vectorConfig.RedisConfig.TTLDays = vectorConfig.TTLDays
		} else if vectorConfig.RedisConfig.TTLDays <= 0 {
			vectorConfig.RedisConfig.TTLDays = 3 // 默认3天
		}

		var err error
		redisStore, err = NewAmapRedisStore(context.Background(), vectorConfig.RedisConfig)
		if err != nil {
			log.Printf("初始化Redis向量存储失败: %v", err)
		} else {
			log.Printf("已初始化Redis向量存储，TTL设置为%d天", vectorConfig.RedisConfig.TTLDays)
		}
	}
}

// IsVectorizationEnabled 检查向量化功能是否已启用
func IsVectorizationEnabled() bool {
	return vectorConfig.Enabled &&
		vectorConfig.ModelEndpoint != "" &&
		vectorConfig.StoragePath != ""
}

// extractDataType 从工具名称提取数据类型
func extractDataType(toolName string) string {
	if dataType, ok := toolToDataTypeMap[toolName]; ok {
		return dataType
	}
	return "unknown"
}

// extractGeoInfo 从响应数据中提取地理信息
func extractGeoInfo(dataType string, data map[string]interface{}) string {
	switch dataType {
	case DataTypePOI:
		if city, ok := data["city"].(string); ok && city != "" {
			return city
		}
		if pois, ok := data["pois"].([]interface{}); ok && len(pois) > 0 {
			if poi, ok := pois[0].(map[string]interface{}); ok {
				if city, ok := poi["cityname"].(string); ok && city != "" {
					return city
				}
				if address, ok := poi["address"].(string); ok && address != "" {
					return address
				}
			}
		}
	case DataTypeWeather:
		if city, ok := data["city"].(string); ok {
			return city
		}
	case DataTypeGeo, DataTypeRegeocode:
		if province, ok := data["province"].(string); ok {
			if city, ok := data["city"].(string); ok && city != "" {
				return fmt.Sprintf("%s %s", province, city)
			}
			return province
		}
	case DataTypeRoute:
		if origin, ok := data["origin"].(string); ok {
			if destination, ok := data["destination"].(string); ok {
				return fmt.Sprintf("%s -> %s", origin, destination)
			}
			return origin
		}
	}
	return ""
}

// extractContentSummary 从响应数据中提取内容摘要
func extractContentSummary(dataType string, data map[string]interface{}) string {
	switch dataType {
	case DataTypePOI:
		if pois, ok := data["pois"].([]interface{}); ok && len(pois) > 0 {
			count := len(pois)
			if count == 1 {
				if poi, ok := pois[0].(map[string]interface{}); ok {
					if name, ok := poi["name"].(string); ok {
						return fmt.Sprintf("POI: %s", name)
					}
				}
			} else {
				return fmt.Sprintf("包含%d个POI的搜索结果", count)
			}
		}
	case DataTypeWeather:
		if city, ok := data["city"].(string); ok {
			return fmt.Sprintf("%s的天气信息", city)
		}
	case DataTypeRoute:
		if paths, ok := data["paths"].([]interface{}); ok && len(paths) > 0 {
			if path, ok := paths[0].(map[string]interface{}); ok {
				if distance, ok := path["distance"].(string); ok {
					if duration, ok := path["duration"].(string); ok {
						return fmt.Sprintf("路线长度:%s米, 预计用时:%s秒", distance, duration)
					}
					return fmt.Sprintf("路线长度:%s米", distance)
				}
			}
		}
	case DataTypeDistance:
		if results, ok := data["results"].([]interface{}); ok && len(results) > 0 {
			if result, ok := results[0].(map[string]interface{}); ok {
				if distance, ok := result["distance"].(string); ok {
					return fmt.Sprintf("距离:%s米", distance)
				}
			}
		}
	}
	return "高德地图数据"
}

// extractAttributes 从响应数据中提取特定类型的属性
func extractAttributes(dataType string, data map[string]interface{}) map[string]interface{} {
	attributes := make(map[string]interface{})

	switch dataType {
	case DataTypePOI:
		if pois, ok := data["pois"].([]interface{}); ok && len(pois) > 0 {
			if poi, ok := pois[0].(map[string]interface{}); ok {
				for k, v := range poi {
					if k == "name" || k == "address" || k == "tel" || k == "type" || k == "typecode" {
						attributes[k] = v
					}
				}
			}
		}
	case DataTypeWeather:
		if forecasts, ok := data["forecasts"].([]interface{}); ok && len(forecasts) > 0 {
			if forecast, ok := forecasts[0].(map[string]interface{}); ok {
				attributes["date"] = forecast["date"]
				attributes["dayweather"] = forecast["dayweather"]
				attributes["nightweather"] = forecast["nightweather"]
				attributes["daytemp"] = forecast["daytemp"]
				attributes["nighttemp"] = forecast["nighttemp"]
			}
		}
	case DataTypeRoute:
		if paths, ok := data["paths"].([]interface{}); ok && len(paths) > 0 {
			if path, ok := paths[0].(map[string]interface{}); ok {
				attributes["distance"] = path["distance"]
				attributes["duration"] = path["duration"]
				if steps, ok := path["steps"].([]interface{}); ok {
					attributes["steps_count"] = len(steps)
				}
			}
		}
	}

	return attributes
}

// generateDataID 为数据生成唯一ID
func generateDataID(toolName string, data map[string]interface{}) string {
	dataType := extractDataType(toolName)

	switch dataType {
	case DataTypePOI:
		if pois, ok := data["pois"].([]interface{}); ok && len(pois) > 0 {
			if poi, ok := pois[0].(map[string]interface{}); ok {
				if id, ok := poi["id"].(string); ok && id != "" {
					return fmt.Sprintf("%s_%s", dataType, id)
				}
			}
		}
	case DataTypeWeather:
		if city, ok := data["city"].(string); ok {
			timestamp := time.Now().Format("20060102")
			return fmt.Sprintf("%s_%s_%s", dataType, city, timestamp)
		}
	case DataTypeRoute:
		if origin, ok := data["origin"].(string); ok {
			if destination, ok := data["destination"].(string); ok {
				hash := fmt.Sprintf("%d", time.Now().Unix())
				return fmt.Sprintf("%s_%s_%s_%s", dataType, strings.Replace(origin, ",", "_", -1),
					strings.Replace(destination, ",", "_", -1), hash)
			}
		}
	}

	// 默认ID生成
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s_%d", dataType, timestamp)
}

// CreateDataTextRepresentation 创建数据的文本表示用于向量化
func CreateDataTextRepresentation(toolName string, data map[string]interface{}) string {
	var parts []string
	dataType := extractDataType(toolName)

	// 添加数据类型信息
	parts = append(parts, fmt.Sprintf("数据类型: %s", dataType))

	// 添加工具名称
	parts = append(parts, fmt.Sprintf("工具: %s", toolName))

	// 根据数据类型添加特定字段
	switch dataType {
	case DataTypePOI:
		if pois, ok := data["pois"].([]interface{}); ok {
			for i, p := range pois {
				if i >= 3 { // 只处理前3个POI
					break
				}
				if poi, ok := p.(map[string]interface{}); ok {
					poiParts := []string{}
					if name, ok := poi["name"].(string); ok && name != "" {
						poiParts = append(poiParts, fmt.Sprintf("名称: %s", name))
					}
					if address, ok := poi["address"].(string); ok && address != "" {
						poiParts = append(poiParts, fmt.Sprintf("地址: %s", address))
					}
					if typecode, ok := poi["typecode"].(string); ok && typecode != "" {
						poiParts = append(poiParts, fmt.Sprintf("类型代码: %s", typecode))
					}
					if len(poiParts) > 0 {
						parts = append(parts, fmt.Sprintf("POI %d: %s", i+1, strings.Join(poiParts, ", ")))
					}
				}
			}
		}
	case DataTypeWeather:
		if city, ok := data["city"].(string); ok && city != "" {
			parts = append(parts, fmt.Sprintf("城市: %s", city))
		}
		if forecasts, ok := data["forecasts"].([]interface{}); ok && len(forecasts) > 0 {
			if forecast, ok := forecasts[0].(map[string]interface{}); ok {
				if date, ok := forecast["date"].(string); ok && date != "" {
					parts = append(parts, fmt.Sprintf("日期: %s", date))
				}
				if dayweather, ok := forecast["dayweather"].(string); ok && dayweather != "" {
					parts = append(parts, fmt.Sprintf("白天天气: %s", dayweather))
				}
				if nightweather, ok := forecast["nightweather"].(string); ok && nightweather != "" {
					parts = append(parts, fmt.Sprintf("夜间天气: %s", nightweather))
				}
				if daytemp, ok := forecast["daytemp"].(string); ok && daytemp != "" {
					if nighttemp, ok := forecast["nighttemp"].(string); ok && nighttemp != "" {
						parts = append(parts, fmt.Sprintf("温度: %s-%s℃", nighttemp, daytemp))
					}
				}
			}
		}
	case DataTypeRoute:
		if origin, ok := data["origin"].(string); ok && origin != "" {
			parts = append(parts, fmt.Sprintf("起点: %s", origin))
		}
		if destination, ok := data["destination"].(string); ok && destination != "" {
			parts = append(parts, fmt.Sprintf("终点: %s", destination))
		}
		if paths, ok := data["paths"].([]interface{}); ok && len(paths) > 0 {
			if path, ok := paths[0].(map[string]interface{}); ok {
				if distance, ok := path["distance"].(string); ok && distance != "" {
					parts = append(parts, fmt.Sprintf("距离: %s米", distance))
				}
				if duration, ok := path["duration"].(string); ok && duration != "" {
					parts = append(parts, fmt.Sprintf("时间: %s秒", duration))
				}
			}
		}
	case DataTypeGeo, DataTypeRegeocode:
		if province, ok := data["province"].(string); ok && province != "" {
			parts = append(parts, fmt.Sprintf("省份: %s", province))
		}
		if city, ok := data["city"].(string); ok && city != "" {
			parts = append(parts, fmt.Sprintf("城市: %s", city))
		}
		if district, ok := data["district"].(string); ok && district != "" {
			parts = append(parts, fmt.Sprintf("区县: %s", district))
		}
	case DataTypeDistance:
		if results, ok := data["results"].([]interface{}); ok && len(results) > 0 {
			for i, r := range results {
				if i >= 3 { // 只处理前3个结果
					break
				}
				if result, ok := r.(map[string]interface{}); ok {
					if distance, ok := result["distance"].(string); ok && distance != "" {
						if duration, ok := result["duration"].(string); ok && duration != "" {
							parts = append(parts, fmt.Sprintf("路线%d: 距离%s米, 用时%s秒", i+1, distance, duration))
						} else {
							parts = append(parts, fmt.Sprintf("路线%d: 距离%s米", i+1, distance))
						}
					}
				}
			}
		}
	case DataTypeIPLocation:
		if province, ok := data["province"].(string); ok && province != "" {
			parts = append(parts, fmt.Sprintf("省份: %s", province))
		}
		if city, ok := data["city"].(string); ok && city != "" {
			parts = append(parts, fmt.Sprintf("城市: %s", city))
		}
		if adcode, ok := data["adcode"].(string); ok && adcode != "" {
			parts = append(parts, fmt.Sprintf("区域编码: %s", adcode))
		}
	}

	// 组合成单个文本字符串
	return strings.Join(parts, ". ")
}

// GetEmbedding 使用模型API获取文本的向量表示
func GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	if !IsVectorizationEnabled() {
		return nil, fmt.Errorf("向量化功能未启用")
	}

	// 使用 volcengine-go-sdk 库创建 arkruntime 客户端
	client := arkruntime.NewClientWithApiKey(
		vectorConfig.APIKey,
		arkruntime.WithBaseUrl("https://ark.cn-beijing.volces.com/api/v3"),
		arkruntime.WithRegion("cn-beijing"),
	)

	// 创建 embedding 请求
	req := model.EmbeddingRequestStrings{
		Input: []string{text},
		Model: vectorConfig.ModelEndpoint, // 使用配置的模型 endpoint ID
	}

	// 调用 API
	resp, err := client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用 Embedding API 失败: %w", err)
	}

	// 检查响应
	if len(resp.Data) == 0 || len(resp.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("API 返回的向量数据为空")
	}

	// 返回向量
	return resp.Data[0].Embedding, nil
}

// VectorizeData 将高德数据向量化
func VectorizeData(ctx context.Context, toolName string, data map[string]interface{}) (*DataVector, error) {
	if !IsVectorizationEnabled() {
		return nil, fmt.Errorf("向量化功能未启用")
	}

	// 提取数据类型
	dataType := extractDataType(toolName)

	// 创建文本表示
	text := CreateDataTextRepresentation(toolName, data)
	log.Printf("为工具[%s]数据创建文本表示: %s", toolName, text)

	// 获取向量
	vector, err := GetEmbedding(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("获取数据向量失败: %w", err)
	}

	// 生成数据ID
	dataID := generateDataID(toolName, data)

	// 提取地理信息
	geoInfo := extractGeoInfo(dataType, data)

	// 提取内容摘要
	contentSummary := extractContentSummary(dataType, data)

	// 提取属性
	attributes := extractAttributes(dataType, data)

	// 创建数据向量
	dataVector := &DataVector{
		OriginalData: data,
		Vector:       vector,
	}

	// 填充元数据
	dataVector.Metadata.ToolName = toolName
	dataVector.Metadata.DataType = dataType
	dataVector.Metadata.GeoInfo = geoInfo
	dataVector.Metadata.Timestamp = time.Now().Unix()
	dataVector.Metadata.Confidence = 1.0 // 默认最高可信度
	dataVector.Metadata.ContentSummary = contentSummary
	dataVector.Metadata.ID = dataID
	dataVector.Metadata.Attributes = attributes

	// 保存到本地文件
	if err := SaveDataVector(dataID, dataVector); err != nil {
		log.Printf("保存数据向量到文件失败: %v", err)
	}

	// 存储到Redis
	if redisStore != nil {
		if err := redisStore.StoreDataVector(ctx, dataVector); err != nil {
			log.Printf("保存数据向量到Redis失败: %v", err)
		} else {
			log.Printf("数据向量已存储到Redis, ID: %s", dataID)
		}
	}

	return dataVector, nil
}

// SaveDataVector 保存数据向量到文件
func SaveDataVector(dataID string, dataVector *DataVector) error {
	if !IsVectorizationEnabled() {
		return fmt.Errorf("向量化功能未启用")
	}

	// 根据数据类型创建子目录
	dataTypeDir := filepath.Join(vectorConfig.StoragePath, dataVector.Metadata.DataType)
	if err := os.MkdirAll(dataTypeDir, 0755); err != nil {
		return fmt.Errorf("创建数据类型目录失败: %w", err)
	}

	// 创建文件路径
	filePath := filepath.Join(dataTypeDir, fmt.Sprintf("%s.json", dataID))

	// 编码为JSON
	data, err := json.MarshalIndent(dataVector, "", "  ")
	if err != nil {
		return fmt.Errorf("编码数据向量失败: %w", err)
	}

	// 写入文件
	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入数据向量文件失败: %w", err)
	}

	log.Printf("数据向量已保存到: %s", filePath)
	return nil
}

// LoadDataVector 从文件加载数据向量
func LoadDataVector(dataType, dataID string) (*DataVector, error) {
	if !IsVectorizationEnabled() {
		return nil, fmt.Errorf("向量化功能未启用")
	}

	// 创建文件路径
	filePath := filepath.Join(vectorConfig.StoragePath, dataType, fmt.Sprintf("%s.json", dataID))

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("数据向量文件不存在: %s", filePath)
	}

	// 读取文件
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取数据向量文件失败: %w", err)
	}

	// 解码JSON
	var dataVector DataVector
	if err := json.Unmarshal(data, &dataVector); err != nil {
		return nil, fmt.Errorf("解码数据向量失败: %w", err)
	}

	return &dataVector, nil
}

// ProcessBatch 批量处理数据向量化
func ProcessBatch(ctx context.Context, items []map[string]interface{}, toolName string) {
	if !IsVectorizationEnabled() {
		log.Println("向量化功能未启用，跳过批量处理")
		return
	}

	log.Printf("开始批量处理工具[%s]的%d个数据项向量化", toolName, len(items))

	for i, item := range items {
		log.Printf("处理第%d个数据项", i+1)

		dataVector, err := VectorizeData(ctx, toolName, item)
		if err != nil {
			log.Printf("向量化数据失败: %v", err)
			continue
		}

		log.Printf("数据向量化成功，类型:%s，ID:%s", dataVector.Metadata.DataType, dataVector.Metadata.ID)
	}

	log.Printf("完成批量处理工具[%s]的%d个数据项向量化", toolName, len(items))
}

// SearchSimilarData 搜索相似的数据
func SearchSimilarData(ctx context.Context, queryText string, dataType string, limit int) ([]*schema.Document, error) {
	if !IsVectorizationEnabled() || redisStore == nil {
		return nil, fmt.Errorf("向量化功能或Redis存储未启用")
	}

	// 获取查询文本的向量
	queryVector, err := GetEmbedding(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("获取查询向量失败: %w", err)
	}

	// 搜索相似数据
	return redisStore.SearchSimilar(ctx, queryVector, dataType, limit)
}

// SearchSimilarDataFiltered 按条件过滤搜索相似的数据
func SearchSimilarDataFiltered(ctx context.Context, queryText string, dataType string, filters map[string]interface{}, limit int) ([]*schema.Document, error) {
	if !IsVectorizationEnabled() || redisStore == nil {
		return nil, fmt.Errorf("向量化功能或Redis存储未启用")
	}

	// 获取查询文本的向量
	queryVector, err := GetEmbedding(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("获取查询向量失败: %w", err)
	}

	// 按条件过滤搜索相似数据
	return redisStore.FilteredSearch(ctx, queryVector, dataType, filters, limit)
}

// CloseVectorization 关闭向量化相关资源
func CloseVectorization() {
	if redisStore != nil {
		if err := redisStore.Close(); err != nil {
			log.Printf("关闭Redis向量存储失败: %v", err)
		}
		redisStore = nil
	}
}
