package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/cloudwego/eino-examples/quickstart/eino_assistant/pkg/tool/amap"
)

func main() {
	// 设置日志
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("开始测试高德地图数据向量化和Redis存储...")

	// 获取环境变量
	embeddingModel := os.Getenv("EMBEDDING_MODEL_ENDPOINT")
	embeddingAPIKey := os.Getenv("EMBEDDING_API_KEY")

	if embeddingModel == "" || embeddingAPIKey == "" {
		log.Println("警告: EMBEDDING_MODEL_ENDPOINT 或 EMBEDDING_API_KEY 环境变量未设置")
		log.Println("将使用默认测试数据...")
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
			IndexPrefix:     "amap_test",
			VectorDimension: 1536,
			DistanceMetric:  "cosine",
		},
	})

	ctx := context.Background()

	// 测试POI数据
	testPOIData := map[string]interface{}{
		"pois": []map[string]interface{}{
			{
				"id":       "B000A83M61",
				"name":     "北京西站",
				"address":  "莲花池东路118号",
				"location": "116.322056,39.894861",
				"typecode": "150200",
				"type":     "交通设施服务",
				"cityname": "北京市",
			},
			{
				"id":       "B000A7BD6N",
				"name":     "北京站",
				"address":  "毛家湾胡同甲13号",
				"location": "116.427013,39.902942",
				"typecode": "150200",
				"type":     "交通设施服务",
				"cityname": "北京市",
			},
		},
	}

	// 测试天气数据
	testWeatherData := map[string]interface{}{
		"city": "北京市",
		"forecasts": []map[string]interface{}{
			{
				"date":         "2025-04-15",
				"week":         "2",
				"dayweather":   "晴",
				"nightweather": "多云",
				"daytemp":      "32",
				"nighttemp":    "18",
			},
		},
	}

	// 测试路线数据
	testRouteData := map[string]interface{}{
		"origin":      "116.307590,40.058440",
		"destination": "116.397428,39.909187",
		"paths": []map[string]interface{}{
			{
				"distance": "23324",
				"duration": "2587",
				"steps": []map[string]interface{}{
					{
						"instruction": "向东行驶",
						"road_name":   "上地十街",
						"distance":    "500",
						"duration":    "120",
					},
				},
			},
		},
	}

	// 向量化测试数据
	log.Println("开始向量化POI测试数据...")
	poiVector, err := amap.VectorizeData(ctx, "maps_text_search", testPOIData)
	if err != nil {
		log.Fatalf("向量化POI数据失败: %v", err)
	}
	log.Printf("POI数据向量化成功，维度: %d, ID: %s", len(poiVector.Vector), poiVector.Metadata.ID)

	log.Println("开始向量化天气测试数据...")
	weatherVector, err := amap.VectorizeData(ctx, "maps_weather", testWeatherData)
	if err != nil {
		log.Fatalf("向量化天气数据失败: %v", err)
	}
	log.Printf("天气数据向量化成功，维度: %d, ID: %s", len(weatherVector.Vector), weatherVector.Metadata.ID)

	log.Println("开始向量化路线测试数据...")
	routeVector, err := amap.VectorizeData(ctx, "maps_direction_driving", testRouteData)
	if err != nil {
		log.Fatalf("向量化路线数据失败: %v", err)
	}
	log.Printf("路线数据向量化成功，维度: %d, ID: %s", len(routeVector.Vector), routeVector.Metadata.ID)

	// 测试搜索功能
	log.Println("开始测试向量搜索功能...")

	// 按数据类型搜索
	log.Println("测试搜索POI数据...")
	poiResults, err := amap.SearchSimilarData(ctx, "北京火车站在哪里", amap.DataTypePOI, 5)
	if err != nil {
		log.Printf("搜索POI数据失败: %v", err)
	} else {
		log.Printf("找到 %d 条相似POI数据", len(poiResults))
		for i, doc := range poiResults {
			// 打印搜索结果
			log.Printf("结果 %d: ID=%s, 内容=%s", i+1, doc.ID, doc.Content)

			// 将元数据转换为JSON并打印
			metaJSON, _ := json.MarshalIndent(doc.MetaData, "", "  ")
			log.Printf("元数据: %s", string(metaJSON))
		}
	}

	// 按城市过滤搜索
	log.Println("测试按城市过滤搜索...")
	filters := map[string]interface{}{
		"geo_info": "北京市",
	}

	filteredResults, err := amap.SearchSimilarDataFiltered(ctx, "出行路线", "", filters, 5)
	if err != nil {
		log.Printf("按城市过滤搜索失败: %v", err)
	} else {
		log.Printf("找到 %d 条符合条件的数据", len(filteredResults))
		for i, doc := range filteredResults {
			log.Printf("过滤结果 %d: ID=%s, 内容=%s, 类型=%s",
				i+1, doc.ID, doc.Content, doc.MetaData["data_type"])
		}
	}

	// 清理资源
	amap.CloseVectorization()
	log.Println("测试完成!")
}
