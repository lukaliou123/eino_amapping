package amap

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// POIData 表示提取的POI数据
type POIData struct {
	ID          string            `json:"id"`          // POI唯一标识
	Name        string            `json:"name"`        // POI名称
	Address     string            `json:"address"`     // 地址
	Description string            `json:"description"` // 描述信息
	Type        string            `json:"type"`        // POI类型
	Location    string            `json:"location"`    // 经纬度
	ToolName    string            `json:"tool_name"`   // 来源工具名称
	Query       string            `json:"query"`       // 原始查询
	City        string            `json:"city"`        // 城市
	Metadata    map[string]string `json:"metadata"`    // 其他元数据
	RawData     string            `json:"raw_data"`    // 原始JSON数据
}

// InterceptorTool 是一个拦截器工具，用于捕获API响应
type InterceptorTool struct {
	originalTool tool.BaseTool
}

// NewInterceptorTool 创建一个新的拦截器工具
func NewInterceptorTool(original tool.BaseTool) *InterceptorTool {
	return &InterceptorTool{
		originalTool: original,
	}
}

// Info 实现Tool接口，返回原始工具的信息
func (it *InterceptorTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return it.originalTool.Info(ctx)
}

// InvokableRun 实现InvokableTool接口，拦截响应并提取POI数据
func (it *InterceptorTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 获取工具名称
	toolInfo, err := it.originalTool.Info(ctx)
	if err != nil {
		return "", err
	}

	// 解析参数，提取查询内容
	var args map[string]interface{}
	var query string
	var city string

	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err == nil {
		// 提取查询关键词
		if keywords, ok := args["keywords"]; ok {
			query = fmt.Sprintf("%v", keywords)
		} else if location, ok := args["location"]; ok {
			query = fmt.Sprintf("%v", location)
		} else if address, ok := args["address"]; ok {
			query = fmt.Sprintf("%v", address)
		}

		// 提取城市信息
		if cityVal, ok := args["city"]; ok {
			city = fmt.Sprintf("%v", cityVal)
		}
	}

	// 调用原始工具
	invokableTool, ok := it.originalTool.(tool.InvokableTool)
	if !ok {
		return "", fmt.Errorf("工具不是可调用的")
	}

	result, err := invokableTool.InvokableRun(ctx, argumentsInJSON, opts...)
	if err != nil {
		return result, err
	}

	// 异步处理，不阻塞主流程
	go func() {
		log.Printf("工具 %s 返回结果，准备提取数据", toolInfo.Name)

		// 首先尝试解析响应JSON
		var responseData map[string]interface{}
		var responseContent []struct {
			Text string `json:"text"`
			Type string `json:"type"`
		}

		if err := json.Unmarshal([]byte(result), &struct {
			Content *[]struct {
				Text string `json:"text"`
				Type string `json:"type"`
			} `json:"content"`
		}{&responseContent}); err == nil && len(responseContent) > 0 {
			// 找到文本内容
			for _, content := range responseContent {
				if content.Type == "text" {
					// 尝试解析JSON文本
					if err := json.Unmarshal([]byte(content.Text), &responseData); err == nil {
						// 成功解析JSON
						log.Printf("成功解析工具 %s 的响应数据", toolInfo.Name)

						// 向量化处理
						if IsVectorizationEnabled() {
							log.Printf("准备向量化工具 %s 的响应数据", toolInfo.Name)
							go func() {
								_, err := VectorizeData(context.Background(), toolInfo.Name, responseData)
								if err != nil {
									log.Printf("向量化工具 %s 的响应数据失败: %v", toolInfo.Name, err)
								}
							}()
						}

						break
					}
				}
			}
		}

		// 根据工具类型处理结果
		switch {
		case strings.Contains(toolInfo.Name, "maps_text_search") ||
			strings.Contains(toolInfo.Name, "maps_around_search"):
			// 处理搜索类API
			poiList, err := extractPOIFromSearch(result, toolInfo.Name, query, city)
			if err != nil {
				log.Printf("提取搜索POI数据失败: %v", err)
				return
			}

			log.Printf("成功从 %s 提取 %d 个POI数据", toolInfo.Name, len(poiList))
			for _, poi := range poiList {
				log.Printf("POI: %s, 地址: %s, ID: %s", poi.Name, poi.Address, poi.ID)
			}

			// 这里可以添加后续处理逻辑，如存储到数据库

		case strings.Contains(toolInfo.Name, "maps_search_detail"):
			// 处理POI详情API
			poi, err := extractPOIFromDetail(result, toolInfo.Name, query)
			if err != nil {
				log.Printf("提取POI详情失败: %v", err)
				return
			}

			log.Printf("成功提取POI详情: %s, 地址: %s, ID: %s", poi.Name, poi.Address, poi.ID)

			// 这里可以添加后续处理逻辑，如存储到数据库

		case strings.Contains(toolInfo.Name, "maps_weather"):
			// 处理天气API
			weatherData, err := extractWeatherData(result, toolInfo.Name, query, city)
			if err != nil {
				log.Printf("提取天气数据失败: %v", err)
				return
			}

			log.Printf("成功提取天气数据: %s, 城市: %s", weatherData.Description, weatherData.City)

		default:
			log.Printf("工具 %s 暂不支持额外的数据提取", toolInfo.Name)
		}
	}()

	// 返回原始结果
	return result, nil
}

// extractPOIFromSearch 从搜索结果中提取POI数据
func extractPOIFromSearch(resultJSON string, toolName string, query string, city string) ([]POIData, error) {
	var result struct {
		Content []struct {
			Text string `json:"text"`
			Type string `json:"type"`
		} `json:"content"`
	}

	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %w", err)
	}

	var poiList []POIData

	// 提取POI内容
	for _, content := range result.Content {
		if content.Type == "text" {
			var poiData struct {
				Pois []struct {
					ID         string   `json:"id"`
					Name       string   `json:"name"`
					Address    string   `json:"address"`
					Location   string   `json:"location"`
					TypeCode   string   `json:"typecode"`
					Type       string   `json:"type"`
					CityName   string   `json:"cityname"`
					Tags       []string `json:"tags,omitempty"`
					Tel        string   `json:"tel,omitempty"`
					Distance   string   `json:"distance,omitempty"`
					Importance string   `json:"importance,omitempty"`
				} `json:"pois"`
			}

			if err := json.Unmarshal([]byte(content.Text), &poiData); err != nil {
				continue
			}

			// 处理每个POI
			for _, poi := range poiData.Pois {
				// 如果没有ID，生成一个唯一ID
				poiID := poi.ID
				if poiID == "" {
					poiID = uuid.New().String()
				}

				// 构建描述信息
				description := ""
				if len(poi.Tags) > 0 {
					description = strings.Join(poi.Tags, ", ")
				}
				if poi.Tel != "" {
					if description != "" {
						description += "; "
					}
					description += "电话: " + poi.Tel
				}

				// 设置城市
				poiCity := city
				if poiCity == "" && poi.CityName != "" {
					poiCity = poi.CityName
				}

				// 创建元数据
				metadata := make(map[string]string)
				if poi.Distance != "" {
					metadata["distance"] = poi.Distance
				}
				if poi.Importance != "" {
					metadata["importance"] = poi.Importance
				}

				// 创建POI数据对象
				poiObj := POIData{
					ID:          poiID,
					Name:        poi.Name,
					Address:     poi.Address,
					Description: description,
					Type:        poi.Type,
					Location:    poi.Location,
					ToolName:    toolName,
					Query:       query,
					City:        poiCity,
					Metadata:    metadata,
					RawData:     content.Text,
				}

				poiList = append(poiList, poiObj)
			}
		}
	}

	return poiList, nil
}

// extractPOIFromDetail 从详情结果中提取POI数据
func extractPOIFromDetail(resultJSON string, toolName string, query string) (*POIData, error) {
	var result struct {
		Content []struct {
			Text string `json:"text"`
			Type string `json:"type"`
		} `json:"content"`
	}

	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %w", err)
	}

	// 提取POI详情
	for _, content := range result.Content {
		if content.Type == "text" {
			var poiDetail struct {
				ID         string   `json:"id"`
				Name       string   `json:"name"`
				Address    string   `json:"address"`
				Location   string   `json:"location"`
				TypeCode   string   `json:"typecode"`
				Type       string   `json:"type"`
				CityName   string   `json:"cityname"`
				Tags       []string `json:"tags,omitempty"`
				Tel        string   `json:"tel,omitempty"`
				Distance   string   `json:"distance,omitempty"`
				Importance string   `json:"importance,omitempty"`
			}

			if err := json.Unmarshal([]byte(content.Text), &poiDetail); err != nil {
				continue
			}

			// 如果没有ID，生成一个唯一ID
			poiID := poiDetail.ID
			if poiID == "" {
				poiID = uuid.New().String()
			}

			// 构建描述信息
			description := ""
			if len(poiDetail.Tags) > 0 {
				description = strings.Join(poiDetail.Tags, ", ")
			}
			if poiDetail.Tel != "" {
				if description != "" {
					description += "; "
				}
				description += "电话: " + poiDetail.Tel
			}

			// 设置城市
			poiCity := ""
			if poiDetail.CityName != "" {
				poiCity = poiDetail.CityName
			}

			// 创建元数据
			metadata := make(map[string]string)
			if poiDetail.Distance != "" {
				metadata["distance"] = poiDetail.Distance
			}
			if poiDetail.Importance != "" {
				metadata["importance"] = poiDetail.Importance
			}

			// 创建POI数据对象
			poiObj := POIData{
				ID:          poiID,
				Name:        poiDetail.Name,
				Address:     poiDetail.Address,
				Description: description,
				Type:        poiDetail.Type,
				Location:    poiDetail.Location,
				ToolName:    toolName,
				Query:       query,
				City:        poiCity,
				Metadata:    metadata,
				RawData:     content.Text,
			}

			return &poiObj, nil
		}
	}

	return nil, fmt.Errorf("未找到POI详情")
}

// extractWeatherData 从天气结果中提取数据
func extractWeatherData(resultJSON string, toolName string, query string, city string) (*POIData, error) {
	var result struct {
		Content []struct {
			Text string `json:"text"`
			Type string `json:"type"`
		} `json:"content"`
	}

	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %w", err)
	}

	// 提取天气数据
	for _, content := range result.Content {
		if content.Type == "text" {
			// 生成唯一ID (基于查询和城市)
			weatherID := uuid.New().String()

			// 创建天气数据对象
			return &POIData{
				ID:          weatherID,
				Name:        city + "天气信息",
				Description: content.Text,
				Type:        "weather",
				ToolName:    toolName,
				Query:       query,
				City:        city,
				RawData:     content.Text,
			}, nil
		}
	}

	return nil, fmt.Errorf("未找到天气数据")
}

// WrapWithInterceptor 将原始工具包装为拦截器工具
func WrapWithInterceptor(original tool.BaseTool) tool.BaseTool {
	return NewInterceptorTool(original)
}
