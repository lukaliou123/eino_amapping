package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cloudwego/eino-examples/quickstart/eino_assistant/pkg/tool/amap"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	// 检查必要的环境变量
	arkModel := os.Getenv("ARK_EMBEDDING_MODEL")
	arkAPIKey := os.Getenv("ARK_API_KEY")
	redisAddr := os.Getenv("REDIS_ADDR")

	if arkModel == "" || arkAPIKey == "" {
		fmt.Println("错误: 环境变量ARK_EMBEDDING_MODEL或ARK_API_KEY未设置")
		fmt.Println("请设置以下环境变量:")
		fmt.Println("  ARK_EMBEDDING_MODEL=<embedding模型名称>")
		fmt.Println("  ARK_API_KEY=<API密钥>")
		os.Exit(1)
	}

	if redisAddr == "" {
		fmt.Println("警告: 环境变量REDIS_ADDR未设置，使用默认地址localhost:6379")
		redisAddr = "localhost:6379"
	}

	// 配置检索器
	config := &amap.AmapRedisRetrieverConfig{
		RedisAddr:    redisAddr,
		IndexPrefix:  "amap:",
		IndexName:    "vector_index",
		TopK:         5, // 返回前5个最相似的结果
		VectorField:  "vector",
		ArkModelName: arkModel,
		ArkAPIKey:    arkAPIKey,
	}

	fmt.Println("正在初始化Redis向量检索器...")
	retriever, err := amap.NewAmapRedisRetriever(ctx, config)
	if err != nil {
		fmt.Printf("创建检索器失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Redis向量检索器初始化成功!")

	// 检索示例
	runTests(ctx, retriever)
}

// 运行一系列检索测试
func runTests(ctx context.Context, retriever retriever.Retriever) {
	// 测试案例
	testCases := []struct {
		name   string
		query  string
		filter string
	}{
		{
			name:  "基本检索",
			query: "附近有什么好玩的地方?",
		},
		{
			name:  "带地域的检索",
			query: "北京有哪些著名景点?",
		},
		{
			name:   "地域过滤检索",
			query:  "有哪些著名景点?",
			filter: "@geo_info:{北京}", // 使用过滤器限定为北京
		},
	}

	// 执行测试
	for _, tc := range testCases {
		fmt.Printf("\n===== 测试: %s =====\n", tc.name)
		fmt.Printf("查询: %s\n", tc.query)

		var docs []*schema.Document
		var err error

		startTime := time.Now()

		// 使用过滤器进行检索
		if tc.filter != "" {
			fmt.Printf("过滤条件: %s\n", tc.filter)
			// 普通检索但打印过滤条件 - 移除WithFilter选项
			docs, err = retriever.Retrieve(ctx, tc.query)
		} else {
			// 普通检索
			docs, err = retriever.Retrieve(ctx, tc.query)
		}

		duration := time.Since(startTime)

		if err != nil {
			fmt.Printf("检索失败: %v\n", err)
			continue
		}

		fmt.Printf("找到 %d 条结果，耗时: %v\n\n", len(docs), duration)

		// 打印结果
		for i, doc := range docs {
			fmt.Printf("结果 #%d:\n", i+1)
			fmt.Printf("  ID: %s\n", doc.ID)
			fmt.Printf("  内容: %s\n", doc.Content)

			// 获取分数
			if score, ok := doc.MetaData["score"].(float64); ok {
				fmt.Printf("  相似度: %.4f\n", score)
			}

			// 打印地理位置信息（如果有）
			if geoInfo, ok := doc.MetaData["geo_info"].(string); ok && geoInfo != "" {
				fmt.Printf("  地理位置: %s\n", geoInfo)
			}

			fmt.Println()
		}

		fmt.Println("================================================")
	}

	fmt.Println("\n所有测试完成!")
}
