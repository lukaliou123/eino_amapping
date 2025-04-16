package amap

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/cloudwego/eino-ext/components/retriever/redis"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	redisCli "github.com/redis/go-redis/v9"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
)

// 默认值常量
const (
	DefaultRedisAddr   = "localhost:6379"
	DefaultTopK        = 10
	DefaultIndexPrefix = "amap:"
	DefaultIndexName   = "vector_index"
	DefaultVectorField = "vector"
)

// AmapRedisRetrieverConfig 是高德地图Redis检索器的配置
type AmapRedisRetrieverConfig struct {
	// Redis配置
	RedisAddr string
	// 向量索引前缀
	IndexPrefix string
	// 向量索引名称
	IndexName string
	// 返回的最大结果数
	TopK int
	// 向量字段名
	VectorField string
	// ARK嵌入模型配置
	ArkModelName string
	ArkAPIKey    string
}

// NewAmapRedisRetriever 创建一个新的高德地图数据Redis检索器
func NewAmapRedisRetriever(ctx context.Context, config *AmapRedisRetrieverConfig) (retriever.Retriever, error) {
	// 如果配置为nil，使用默认配置
	if config == nil {
		config = &AmapRedisRetrieverConfig{
			RedisAddr:   DefaultRedisAddr,
			IndexPrefix: DefaultIndexPrefix,
			IndexName:   DefaultIndexName,
			TopK:        DefaultTopK,
			VectorField: DefaultVectorField,
		}
	}

	// 读取环境变量补充配置
	if config.RedisAddr == "" {
		redisAddr := os.Getenv("REDIS_ADDR")
		if redisAddr != "" {
			config.RedisAddr = redisAddr
		} else {
			config.RedisAddr = DefaultRedisAddr
		}
	}

	if config.IndexPrefix == "" {
		config.IndexPrefix = DefaultIndexPrefix
	}

	if config.IndexName == "" {
		config.IndexName = DefaultIndexName
	}

	if config.TopK <= 0 {
		config.TopK = DefaultTopK
	}

	if config.VectorField == "" {
		config.VectorField = DefaultVectorField
	}

	// 创建Redis客户端
	redisClient := redisCli.NewClient(&redisCli.Options{
		Addr:     config.RedisAddr,
		Protocol: 2, // 使用RESP2协议
	})

	// 测试Redis连接
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接Redis失败: %w", err)
	}

	// 初始化嵌入模型配置
	if config.ArkModelName == "" {
		config.ArkModelName = os.Getenv("ARK_EMBEDDING_MODEL")
		if config.ArkModelName == "" {
			return nil, fmt.Errorf("缺少ARK_EMBEDDING_MODEL配置")
		}
	}

	if config.ArkAPIKey == "" {
		config.ArkAPIKey = os.Getenv("ARK_API_KEY")
		if config.ArkAPIKey == "" {
			return nil, fmt.Errorf("缺少ARK_API_KEY配置")
		}
	}

	// 创建ARK嵌入模型
	embedder, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: config.ArkAPIKey,
		Model:  config.ArkModelName,
		Region: "cn-beijing", // 默认使用北京区域
	})
	if err != nil {
		return nil, fmt.Errorf("初始化ARK嵌入模型失败: %w", err)
	}

	// 索引名称
	indexName := fmt.Sprintf("%s%s", config.IndexPrefix, config.IndexName)

	// 配置Redis检索器
	retrieverConfig := &redis.RetrieverConfig{
		Client:  redisClient,
		Index:   indexName,
		Dialect: 2, // Redis Stack 2.x
		ReturnFields: []string{
			"id", "content", "tool_name", "data_type",
			"geo_info", "timestamp", "content_summary",
			"original_data", "distance",
		},
		TopK:        config.TopK,
		VectorField: config.VectorField,
		Embedding:   embedder,
		// 文档转换器，将Redis文档转换为Eino文档格式
		DocumentConverter: func(ctx context.Context, doc redisCli.Document) (*schema.Document, error) {
			resp := &schema.Document{
				ID:       doc.ID,
				Content:  "",
				MetaData: map[string]any{},
			}

			for field, val := range doc.Fields {
				switch field {
				case "content", "content_summary":
					resp.Content = val
				case "distance":
					// 将距离转换为相似度分数
					distance, err := strconv.ParseFloat(val, 64)
					if err != nil {
						continue
					}
					resp.WithScore(1 - distance) // 余弦距离越小表示越相似，转换为相似度
				default:
					// 其他字段保存到元数据中
					resp.MetaData[field] = val
				}
			}

			return resp, nil
		},
	}

	// 创建Redis检索器
	rtr, err := redis.NewRetriever(ctx, retrieverConfig)
	if err != nil {
		return nil, fmt.Errorf("创建Redis检索器失败: %w", err)
	}

	return rtr, nil
}
