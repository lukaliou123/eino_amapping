package amap

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestNewAmapRedisRetriever(t *testing.T) {
	// 如果没有设置环境变量，跳过测试
	if os.Getenv("ARK_EMBEDDING_MODEL") == "" || os.Getenv("ARK_API_KEY") == "" {
		t.Skip("环境变量ARK_EMBEDDING_MODEL或ARK_API_KEY未设置，跳过测试")
	}

	ctx := context.Background()
	config := &AmapRedisRetrieverConfig{
		RedisAddr:    "localhost:6379",
		IndexPrefix:  "amap:",
		IndexName:    "vector_index",
		TopK:         5,
		VectorField:  "vector",
		ArkModelName: os.Getenv("ARK_EMBEDDING_MODEL"),
		ArkAPIKey:    os.Getenv("ARK_API_KEY"),
	}

	// 创建检索器
	retriever, err := NewAmapRedisRetriever(ctx, config)
	if err != nil {
		t.Fatalf("创建检索器失败: %v", err)
	}

	// 检查检索器是否非空
	assert.NotNil(t, retriever)
}

// 查询示例函数，展示如何使用检索器
func ExampleSearch() {
	ctx := context.Background()
	retriever, err := NewAmapRedisRetriever(ctx, nil) // 使用默认配置
	if err != nil {
		fmt.Printf("创建检索器失败: %v\n", err)
		return
	}

	// 简单查询
	docs, err := retriever.Retrieve(ctx, "附近的景点")
	if err != nil {
		fmt.Printf("检索失败: %v\n", err)
		return
	}

	// 处理结果
	for _, doc := range docs {
		fmt.Printf("文档ID: %s\n", doc.ID)
		fmt.Printf("内容: %s\n", doc.Content)
		fmt.Printf("相似度: %f\n", doc.GetScore())
	}
}

// 展示如何使用地理位置过滤
func ExampleGeoFilter() {
	ctx := context.Background()
	retriever, err := NewAmapRedisRetriever(ctx, nil) // 使用默认配置
	if err != nil {
		fmt.Printf("创建检索器失败: %v\n", err)
		return
	}

	// 用于创建过滤条件的辅助函数
	createGeoFilter := func(location string) string {
		if location == "" {
			return ""
		}
		return fmt.Sprintf("@geo_info:{%s}", location)
	}

	// 北京地区的景点
	filter := createGeoFilter("北京")

	// 使用检索器的WithFilter选项
	// 注意：需要引入对应的选项包，这里仅做示例
	// 实际使用中，需要检查官方文档获取正确的WithFilter选项路径
	/*
		docs, err := retriever.Retrieve(ctx, "有哪些著名的景点?", redis.WithFilter(filter))
		if err != nil {
			fmt.Printf("检索失败: %v\n", err)
			return
		}
	*/

	// 为了演示，我们直接使用无过滤的检索
	docs, err := retriever.Retrieve(ctx, "北京有哪些著名的景点?")
	if err != nil {
		fmt.Printf("检索失败: %v\n", err)
		return
	}

	// 处理结果
	printDocs(docs)
}

// 辅助函数：打印文档结果
func printDocs(docs []*schema.Document) {
	fmt.Printf("找到 %d 条结果\n", len(docs))
	for i, doc := range docs {
		fmt.Printf("结果 #%d:\n", i+1)
		fmt.Printf("  ID: %s\n", doc.ID)
		fmt.Printf("  内容: %s\n", doc.Content)
		fmt.Printf("  相似度: %.4f\n", doc.GetScore())

		// 打印地理位置信息（如果有）
		if geoInfo, ok := doc.MetaData["geo_info"].(string); ok && geoInfo != "" {
			fmt.Printf("  地理位置: %s\n", geoInfo)
		}

		fmt.Println()
	}
}
