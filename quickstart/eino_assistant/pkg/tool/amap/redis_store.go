package amap

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

// RedisVectorStoreConfig Redis向量存储配置
type RedisVectorStoreConfig struct {
	// Redis连接地址，格式如：localhost:6379
	Address string
	// Redis密码
	Password string
	// 使用的数据库编号
	DB int
	// 索引名称前缀
	IndexPrefix string
	// 向量维度
	VectorDimension int
	// 距离度量方式："cosine"、"l2"、"ip"
	DistanceMetric string
}

// 默认配置
var DefaultRedisConfig = RedisVectorStoreConfig{
	Address:         "localhost:6379",
	Password:        "",
	DB:              0,
	IndexPrefix:     "amap",
	VectorDimension: 1536, // 根据使用的嵌入模型调整
	DistanceMetric:  "cosine",
}

// AmapRedisStore 高德地图数据Redis向量存储
type AmapRedisStore struct {
	client         *redis.Client
	config         RedisVectorStoreConfig
	indexesCreated map[string]bool
}

// NewAmapRedisStore 创建新的高德地图数据Redis向量存储
func NewAmapRedisStore(ctx context.Context, config *RedisVectorStoreConfig) (*AmapRedisStore, error) {
	if config == nil {
		cfg := DefaultRedisConfig
		config = &cfg
	}

	client := redis.NewClient(&redis.Options{
		Addr:     config.Address,
		Password: config.Password,
		DB:       config.DB,
	})

	// 测试连接
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接Redis失败: %w", err)
	}

	return &AmapRedisStore{
		client:         client,
		config:         *config,
		indexesCreated: make(map[string]bool),
	}, nil
}

// Close 关闭Redis连接
func (s *AmapRedisStore) Close() error {
	return s.client.Close()
}

// ensureIndex 确保指定类型的索引存在
func (s *AmapRedisStore) ensureIndex(ctx context.Context, dataType string) error {
	indexName := fmt.Sprintf("%s:%s", s.config.IndexPrefix, dataType)

	// 如果索引已创建，直接返回
	if s.indexesCreated[indexName] {
		return nil
	}

	// 检查索引是否已存在
	exists := false
	indices, err := s.client.Do(ctx, "FT._LIST").Result()
	if err == nil {
		indicesList, ok := indices.([]interface{})
		if ok {
			for _, idx := range indicesList {
				if idxStr, ok := idx.(string); ok && idxStr == indexName {
					exists = true
					break
				}
			}
		}
	}

	// 如果索引不存在，创建它
	if !exists {
		// 创建向量索引
		// 注意：这里使用RedisSearch命令创建向量索引
		// 根据Redis版本和模块可能需要调整命令
		createCmd := []interface{}{
			"FT.CREATE", indexName,
			"ON", "HASH",
			"PREFIX", "1", fmt.Sprintf("%s:%s:", s.config.IndexPrefix, dataType),
			"SCHEMA",
			"tool_name", "TEXT", "SORTABLE",
			"data_type", "TEXT", "SORTABLE",
			"geo_info", "TEXT", "SORTABLE",
			"content_summary", "TEXT", "SORTABLE",
			"timestamp", "NUMERIC", "SORTABLE",
			"confidence", "NUMERIC", "SORTABLE",
			"vector", "VECTOR", "FLAT",
			"6",
			"TYPE", "FLOAT32",
			"DIM", s.config.VectorDimension,
			"DISTANCE_METRIC", s.config.DistanceMetric,
		}

		_, err := s.client.Do(ctx, createCmd...).Result()
		if err != nil && !strings.Contains(err.Error(), "Index already exists") {
			return fmt.Errorf("创建索引失败: %w", err)
		}

		log.Printf("为数据类型 %s 创建了索引 %s", dataType, indexName)
	}

	// 标记索引已创建
	s.indexesCreated[indexName] = true
	return nil
}

// StoreDataVector 将高德数据向量存储到Redis
func (s *AmapRedisStore) StoreDataVector(ctx context.Context, dataVector *DataVector) error {
	// 确保索引存在
	if err := s.ensureIndex(ctx, dataVector.Metadata.DataType); err != nil {
		return err
	}

	// 准备Redis存储的键
	key := fmt.Sprintf("%s:%s:%s",
		s.config.IndexPrefix,
		dataVector.Metadata.DataType,
		dataVector.Metadata.ID)

	// 将向量数据转换为文档格式
	doc := schema.Document{
		ID:      dataVector.Metadata.ID,
		Content: dataVector.Metadata.ContentSummary,
		MetaData: map[string]interface{}{
			"tool_name":       dataVector.Metadata.ToolName,
			"data_type":       dataVector.Metadata.DataType,
			"geo_info":        dataVector.Metadata.GeoInfo,
			"timestamp":       dataVector.Metadata.Timestamp,
			"confidence":      dataVector.Metadata.Confidence,
			"content_summary": dataVector.Metadata.ContentSummary,
			"attributes":      dataVector.Metadata.Attributes,
			"original_data":   dataVector.OriginalData,
			"vector":          dataVector.Vector, // 将向量放入元数据
		},
	}

	// 将文档转换为Redis哈希字段
	fields := make(map[string]interface{})

	// 添加元数据字段
	fields["id"] = doc.ID
	fields["content"] = doc.Content
	fields["tool_name"] = dataVector.Metadata.ToolName
	fields["data_type"] = dataVector.Metadata.DataType
	fields["geo_info"] = dataVector.Metadata.GeoInfo
	fields["timestamp"] = dataVector.Metadata.Timestamp
	fields["confidence"] = dataVector.Metadata.Confidence
	fields["content_summary"] = dataVector.Metadata.ContentSummary

	// 将向量转换为适合Redis存储的格式
	vectorBytes, err := vectorToBytes(dataVector.Vector)
	if err != nil {
		return fmt.Errorf("向量序列化失败: %w", err)
	}
	fields["vector"] = vectorBytes

	// 将原始数据和属性序列化为JSON
	if len(dataVector.Metadata.Attributes) > 0 {
		attrJSON, err := json.Marshal(dataVector.Metadata.Attributes)
		if err != nil {
			return fmt.Errorf("序列化属性失败: %w", err)
		}
		fields["attributes"] = string(attrJSON)
	}

	origDataJSON, err := json.Marshal(dataVector.OriginalData)
	if err != nil {
		return fmt.Errorf("序列化原始数据失败: %w", err)
	}
	fields["original_data"] = string(origDataJSON)

	// 存储到Redis
	if err := s.client.HSet(ctx, key, fields).Err(); err != nil {
		return fmt.Errorf("存储到Redis失败: %w", err)
	}

	log.Printf("向量数据已存储到Redis, key: %s", key)
	return nil
}

// vectorToBytes 将向量转换为字节数组(适用于Redis存储)
func vectorToBytes(vector []float32) ([]byte, error) {
	// 这里根据Redis向量存储的实际要求进行转换
	// 对于RedisSearch，通常需要特定格式的字节数组
	// 这是一个简化版本，实际使用可能需要调整
	return json.Marshal(vector)
}

// SearchSimilar 搜索相似的高德数据
func (s *AmapRedisStore) SearchSimilar(ctx context.Context, queryVector []float32, dataType string, limit int) ([]*schema.Document, error) {
	if limit <= 0 {
		limit = 10 // 默认返回10条
	}

	// 确保索引存在
	if err := s.ensureIndex(ctx, dataType); err != nil {
		return nil, err
	}

	// 索引名称
	indexName := fmt.Sprintf("%s:%s", s.config.IndexPrefix, dataType)

	// 将查询向量转换为字节数组
	vectorBytes, err := vectorToBytes(queryVector)
	if err != nil {
		return nil, fmt.Errorf("向量序列化失败: %w", err)
	}

	// 构建向量搜索查询
	// 注意：根据Redis版本和模块可能需要调整命令
	searchCmd := []interface{}{
		"FT.SEARCH", indexName,
		"*=>[KNN", fmt.Sprintf("%d", limit), "@vector", "$query_vector", "AS", "score", "]",
		"SORTBY", "score",
		"PARAMS", "2", "query_vector", vectorBytes,
		"LIMIT", "0", fmt.Sprintf("%d", limit),
		"RETURN", "8", "id", "content", "tool_name", "data_type", "geo_info", "timestamp", "content_summary", "original_data",
	}

	// 执行搜索
	result, err := s.client.Do(ctx, searchCmd...).Result()
	if err != nil {
		return nil, fmt.Errorf("向量搜索失败: %w", err)
	}

	// 解析结果
	searchResults, ok := result.([]interface{})
	if !ok || len(searchResults) < 1 {
		return nil, nil // 没有找到结果
	}

	// 转换结果为Document数组
	totalResults, ok := searchResults[0].(int64)
	if !ok || totalResults == 0 {
		return nil, nil
	}

	var docs []*schema.Document
	for i := 1; i < len(searchResults); i += 2 {
		docID, ok := searchResults[i].(string)
		if !ok {
			continue
		}

		fieldsArray, ok := searchResults[i+1].([]interface{})
		if !ok || len(fieldsArray) < 16 { // 8个字段，每个字段有名称和值
			continue
		}

		// 提取字段
		doc := &schema.Document{
			ID:       docID,
			MetaData: make(map[string]interface{}),
		}

		for j := 0; j < len(fieldsArray); j += 2 {
			fieldName, ok := fieldsArray[j].(string)
			if !ok {
				continue
			}
			fieldValue, ok := fieldsArray[j+1].(string)
			if !ok {
				continue
			}

			switch fieldName {
			case "id":
				doc.ID = fieldValue
			case "content":
				doc.Content = fieldValue
			case "content_summary":
				doc.MetaData["content_summary"] = fieldValue
			case "tool_name":
				doc.MetaData["tool_name"] = fieldValue
			case "data_type":
				doc.MetaData["data_type"] = fieldValue
			case "geo_info":
				doc.MetaData["geo_info"] = fieldValue
			case "timestamp":
				doc.MetaData["timestamp"] = fieldValue
			case "original_data":
				var originalData map[string]interface{}
				if err := json.Unmarshal([]byte(fieldValue), &originalData); err == nil {
					doc.MetaData["original_data"] = originalData
				}
			}
		}

		docs = append(docs, doc)
	}

	return docs, nil
}

// FilteredSearch 按条件过滤的向量搜索
func (s *AmapRedisStore) FilteredSearch(ctx context.Context, queryVector []float32, dataType string, filters map[string]interface{}, limit int) ([]*schema.Document, error) {
	if limit <= 0 {
		limit = 10
	}

	// 确保索引存在
	if err := s.ensureIndex(ctx, dataType); err != nil {
		return nil, err
	}

	// 索引名称
	indexName := fmt.Sprintf("%s:%s", s.config.IndexPrefix, dataType)

	// 将查询向量转换为字节数组
	vectorBytes, err := vectorToBytes(queryVector)
	if err != nil {
		return nil, fmt.Errorf("向量序列化失败: %w", err)
	}

	// 构建过滤条件
	var filterParts []string
	if dataType != "" {
		filterParts = append(filterParts, fmt.Sprintf("@data_type:{%s}", dataType))
	}

	// 添加其他过滤条件
	for key, value := range filters {
		switch key {
		case "geo_info":
			if strVal, ok := value.(string); ok && strVal != "" {
				filterParts = append(filterParts, fmt.Sprintf("@geo_info:{%s}", strVal))
			}
		case "tool_name":
			if strVal, ok := value.(string); ok && strVal != "" {
				filterParts = append(filterParts, fmt.Sprintf("@tool_name:{%s}", strVal))
			}
		case "min_confidence":
			if floatVal, ok := value.(float64); ok {
				filterParts = append(filterParts, fmt.Sprintf("@confidence:[%f +inf]", floatVal))
			}
		}
	}

	// 组合过滤条件
	filterQuery := "*"
	if len(filterParts) > 0 {
		filterQuery = strings.Join(filterParts, " ")
	}

	// 构建向量搜索查询
	searchCmd := []interface{}{
		"FT.SEARCH", indexName,
		fmt.Sprintf("(%s)=>[KNN %d @vector $query_vector AS score]", filterQuery, limit),
		"SORTBY", "score",
		"PARAMS", "2", "query_vector", vectorBytes,
		"LIMIT", "0", fmt.Sprintf("%d", limit),
		"RETURN", "8", "id", "content", "tool_name", "data_type", "geo_info", "timestamp", "content_summary", "original_data",
	}

	// 执行搜索
	result, err := s.client.Do(ctx, searchCmd...).Result()
	if err != nil {
		return nil, fmt.Errorf("过滤向量搜索失败: %w", err)
	}

	// 解析结果
	searchResults, ok := result.([]interface{})
	if !ok || len(searchResults) < 1 {
		return nil, nil
	}

	// 转换结果为Document数组
	// 这部分逻辑与SearchSimilar相同，可以提取为一个辅助函数
	totalResults, ok := searchResults[0].(int64)
	if !ok || totalResults == 0 {
		return nil, nil
	}

	var docs []*schema.Document
	for i := 1; i < len(searchResults); i += 2 {
		docID, ok := searchResults[i].(string)
		if !ok {
			continue
		}

		fieldsArray, ok := searchResults[i+1].([]interface{})
		if !ok || len(fieldsArray) < 16 {
			continue
		}

		// 提取字段
		doc := &schema.Document{
			ID:       docID,
			MetaData: make(map[string]interface{}),
		}

		for j := 0; j < len(fieldsArray); j += 2 {
			fieldName, ok := fieldsArray[j].(string)
			if !ok {
				continue
			}
			fieldValue, ok := fieldsArray[j+1].(string)
			if !ok {
				continue
			}

			switch fieldName {
			case "id":
				doc.ID = fieldValue
			case "content":
				doc.Content = fieldValue
			case "content_summary":
				doc.MetaData["content_summary"] = fieldValue
			case "tool_name":
				doc.MetaData["tool_name"] = fieldValue
			case "data_type":
				doc.MetaData["data_type"] = fieldValue
			case "geo_info":
				doc.MetaData["geo_info"] = fieldValue
			case "timestamp":
				doc.MetaData["timestamp"] = fieldValue
			case "original_data":
				var originalData map[string]interface{}
				if err := json.Unmarshal([]byte(fieldValue), &originalData); err == nil {
					doc.MetaData["original_data"] = originalData
				}
			}
		}

		docs = append(docs, doc)
	}

	return docs, nil
}
