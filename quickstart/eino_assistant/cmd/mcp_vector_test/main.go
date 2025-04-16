package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-examples/quickstart/eino_assistant/pkg/tool/amap"
)

func main() {
	// 设置日志
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("开始测试高德地图MCP数据向量化和Redis存储...")

	// 获取环境变量
	embeddingModel := os.Getenv("ARK_EMBEDDING_MODEL")
	embeddingAPIKey := os.Getenv("ARK_API_KEY")

	if embeddingModel == "" || embeddingAPIKey == "" {
		log.Println("警告: ARK_EMBEDDING_MODEL 或 ARK_API_KEY 环境变量未设置")
		log.Println("请在.env文件中设置这些值，然后使用source .env命令加载")
		return
	}

	// 设置向量化配置
	amap.SetVectorConfig(amap.VectorConfig{
		ModelEndpoint:    embeddingModel,
		APIKey:           embeddingAPIKey,
		StoragePath:      "data/amap_vectors",
		Enabled:          true,
		EnableRedisStore: true,
		RedisConfig: &amap.RedisVectorStoreConfig{
			Address:         "localhost:6379",
			Password:        "",
			DB:              0,
			IndexPrefix:     "amap_mcp",
			VectorDimension: 1536,
			DistanceMetric:  "cosine", // 使用余弦相似度
		},
	})

	ctx := context.Background()

	// 模拟MCP SSE天气数据
	weatherData := map[string]interface{}{
		"city":       "上海市",
		"adcode":     "310000",
		"province":   "上海市",
		"reporttime": "2023-04-15 16:00:00",
		"forecasts": []map[string]interface{}{
			{
				"date":         "2023-04-15",
				"week":         "6",
				"dayweather":   "多云",
				"nightweather": "晴",
				"daytemp":      "26",
				"nighttemp":    "18",
				"daywind":      "东南",
				"nightwind":    "东南",
				"daypower":     "4",
				"nightpower":   "3",
			},
			{
				"date":         "2023-04-16",
				"week":         "7",
				"dayweather":   "晴",
				"nightweather": "多云",
				"daytemp":      "28",
				"nighttemp":    "19",
				"daywind":      "南",
				"nightwind":    "南",
				"daypower":     "4",
				"nightpower":   "3",
			},
		},
	}

	// 模拟MCP SSE POI搜索数据
	poiData := map[string]interface{}{
		"pois": []map[string]interface{}{
			{
				"id":       "B0FFFZ7A7A",
				"name":     "上海迪士尼乐园",
				"address":  "上海市浦东新区申迪西路753号",
				"location": "121.668248,31.143694",
				"typecode": "110207",
				"type":     "旅游景点",
				"cityname": "上海市",
			},
			{
				"id":       "B00160D494",
				"name":     "上海科技馆",
				"address":  "上海市浦东新区世纪大道2000号",
				"location": "121.550988,31.217297",
				"typecode": "140600",
				"type":     "教育文化服务",
				"cityname": "上海市",
			},
		},
	}

	// 模拟MCP SSE路线规划数据
	routeData := map[string]interface{}{
		"origin":      "121.473701,31.230416",
		"destination": "121.550988,31.217297",
		"paths": []map[string]interface{}{
			{
				"distance": "12345",
				"duration": "2400",
				"steps": []map[string]interface{}{
					{
						"instruction": "沿南京东路步行",
						"distance":    "500",
						"duration":    "360",
					},
					{
						"instruction": "乘坐地铁2号线",
						"distance":    "8000",
						"duration":    "1200",
					},
					{
						"instruction": "步行至上海科技馆",
						"distance":    "300",
						"duration":    "240",
					},
				},
			},
		},
	}

	// 模拟MCP SSE附近搜索数据
	nearbyData := map[string]interface{}{
		"pois": []map[string]interface{}{
			{
				"id":       "B0FFIP70ZP",
				"name":     "上海东方明珠广播电视塔",
				"address":  "上海市浦东新区世纪大道1号",
				"location": "121.499644,31.239254",
				"typecode": "110207",
				"type":     "旅游景点",
				"cityname": "上海市",
				"distance": "1200",
			},
			{
				"id":       "B001537F4N",
				"name":     "上海海洋水族馆",
				"address":  "上海市浦东新区陆家嘴环路1388号",
				"location": "121.501528,31.240593",
				"typecode": "110202",
				"type":     "旅游景点",
				"cityname": "上海市",
				"distance": "1500",
			},
		},
	}

	// 向量化测试数据
	log.Println("====== 向量化MCP数据 ======")

	log.Println("向量化天气数据...")
	weatherVector, err := amap.VectorizeData(ctx, "maps_weather", weatherData)
	if err != nil {
		log.Fatalf("向量化天气数据失败: %v", err)
	}
	log.Printf("天气数据向量化成功，维度: %d, ID: %s", len(weatherVector.Vector), weatherVector.Metadata.ID)

	log.Println("向量化POI数据...")
	poiVector, err := amap.VectorizeData(ctx, "maps_text_search", poiData)
	if err != nil {
		log.Fatalf("向量化POI数据失败: %v", err)
	}
	log.Printf("POI数据向量化成功，维度: %d, ID: %s", len(poiVector.Vector), poiVector.Metadata.ID)

	log.Println("向量化路线数据...")
	routeVector, err := amap.VectorizeData(ctx, "maps_direction_driving", routeData)
	if err != nil {
		log.Fatalf("向量化路线数据失败: %v", err)
	}
	log.Printf("路线数据向量化成功，维度: %d, ID: %s", len(routeVector.Vector), routeVector.Metadata.ID)

	log.Println("向量化附近搜索数据...")
	nearbyVector, err := amap.VectorizeData(ctx, "maps_around_search", nearbyData)
	if err != nil {
		log.Fatalf("向量化附近搜索数据失败: %v", err)
	}
	log.Printf("附近搜索数据向量化成功，维度: %d, ID: %s", len(nearbyVector.Vector), nearbyVector.Metadata.ID)

	// 给向量存储一些时间同步
	time.Sleep(2 * time.Second)

	// 测试向量搜索
	log.Println("\n====== 测试向量搜索 ======")

	// 测试1: 根据天气查询
	log.Println("测试1: 查询上海天气...")
	weatherResults, err := amap.SearchSimilarData(ctx, "上海最近天气如何？是晴天还是雨天？", amap.DataTypeWeather, 3)
	if err != nil {
		log.Printf("搜索天气数据失败: %v", err)
	} else {
		log.Printf("找到 %d 条相关天气数据", len(weatherResults))
		for i, doc := range weatherResults {
			// 打印搜索结果
			log.Printf("结果 %d: ID=%s, 内容=%s", i+1, doc.ID, doc.Content)

			// 简单展示原始数据的一部分
			if originalData, ok := doc.MetaData["original_data"].(map[string]interface{}); ok {
				if city, ok := originalData["city"].(string); ok {
					log.Printf("  城市: %s", city)
				}
				if forecasts, ok := originalData["forecasts"].([]interface{}); ok && len(forecasts) > 0 {
					if forecast, ok := forecasts[0].(map[string]interface{}); ok {
						if dayweather, ok := forecast["dayweather"].(string); ok {
							log.Printf("  天气: %s", dayweather)
						}
						if daytemp, ok := forecast["daytemp"].(string); ok {
							log.Printf("  温度: %s°C", daytemp)
						}
					}
				}
			}
		}
	}

	// 测试2: 根据POI查询
	log.Println("\n测试2: 查询上海旅游景点...")
	poiResults, err := amap.SearchSimilarData(ctx, "上海有什么好玩的地方推荐？", amap.DataTypePOI, 3)
	if err != nil {
		log.Printf("搜索POI数据失败: %v", err)
	} else {
		log.Printf("找到 %d 条相关POI数据", len(poiResults))
		for i, doc := range poiResults {
			log.Printf("结果 %d: ID=%s, 内容=%s", i+1, doc.ID, doc.Content)

			// 尝试打印POI名称和地址
			if originalData, ok := doc.MetaData["original_data"].(map[string]interface{}); ok {
				if pois, ok := originalData["pois"].([]interface{}); ok && len(pois) > 0 {
					poiData := pois[0].(map[string]interface{})
					if name, ok := poiData["name"].(string); ok {
						log.Printf("  名称: %s", name)
					}
					if address, ok := poiData["address"].(string); ok {
						log.Printf("  地址: %s", address)
					}
				}
			}
		}
	}

	// 测试3: 按地理位置过滤搜索
	log.Println("\n测试3: 按地理位置过滤搜索...")
	filters := map[string]interface{}{
		"geo_info": "上海市",
	}

	filteredResults, err := amap.SearchSimilarDataFiltered(ctx, "附近有什么景点？", "", filters, 5)
	if err != nil {
		log.Printf("按地理位置过滤搜索失败: %v", err)
	} else {
		log.Printf("找到 %d 条符合条件的数据", len(filteredResults))
		for i, doc := range filteredResults {
			dataType := "未知"
			if dt, ok := doc.MetaData["data_type"].(string); ok {
				dataType = dt
			}

			log.Printf("过滤结果 %d: ID=%s, 类型=%s, 内容=%s",
				i+1, doc.ID, dataType, doc.Content)
		}
	}

	// 测试4: 跨类型搜索
	log.Println("\n测试4: 跨类型搜索...")
	allResults, err := amap.SearchSimilarData(ctx, "从人民广场到上海科技馆怎么走？", "", 5)
	if err != nil {
		log.Printf("跨类型搜索失败: %v", err)
	} else {
		log.Printf("找到 %d 条相关数据", len(allResults))
		for i, doc := range allResults {
			dataType := "未知"
			if dt, ok := doc.MetaData["data_type"].(string); ok {
				dataType = dt
			}

			log.Printf("结果 %d: ID=%s, 类型=%s, 内容=%s",
				i+1, doc.ID, dataType, doc.Content)
		}
	}

	// 清理资源
	amap.CloseVectorization()
	log.Println("\n测试完成!")
	fmt.Println("按任意键退出...")
	fmt.Scanln()
}
