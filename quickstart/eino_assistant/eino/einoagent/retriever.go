/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package einoagent

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"

	redispkg "github.com/cloudwego/eino-examples/quickstart/eino_assistant/pkg/redis"
	"github.com/cloudwego/eino-examples/quickstart/eino_assistant/pkg/tool/amap"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/flow/retriever/multiquery"
	"github.com/cloudwego/eino/schema"
	redisCli "github.com/redis/go-redis/v9"

	"github.com/cloudwego/eino-ext/components/retriever/redis"
)

// newRetriever component initialization function of node 'RedisRetriever' in graph 'EinoAgent'
func newRetriever(ctx context.Context) (rtr retriever.Retriever, err error) {
	// 1. 创建原始的检索器
	originalRetriever, err := createOriginalRetriever(ctx)
	if err != nil {
		return nil, fmt.Errorf("创建原始检索器失败: %w", err)
	}

	// 2. 创建高德地图检索器
	amapRetriever, err := createAmapRetriever(ctx)
	if err != nil {
		// 如果高德检索器创建失败，只使用原始检索器
		return originalRetriever, nil
	}

	// 3. 创建MultiQuery检索器
	multiRetriever, err := createMultiQueryRetriever(ctx, originalRetriever, amapRetriever)
	if err != nil {
		// 如果创建多检索器失败，退回到使用原始检索器
		return originalRetriever, nil
	}

	return multiRetriever, nil
}

// createOriginalRetriever 创建原始的检索器
func createOriginalRetriever(ctx context.Context) (retriever.Retriever, error) {
	redisAddr := os.Getenv("REDIS_ADDR")
	redisClient := redisCli.NewClient(&redisCli.Options{
		Addr:     redisAddr,
		Protocol: 2,
	})
	config := &redis.RetrieverConfig{
		Client:       redisClient,
		Index:        fmt.Sprintf("%s%s", redispkg.RedisPrefix, redispkg.IndexName),
		Dialect:      2,
		ReturnFields: []string{redispkg.ContentField, redispkg.MetadataField, redispkg.DistanceField},
		TopK:         8,
		VectorField:  redispkg.VectorField,
		DocumentConverter: func(ctx context.Context, doc redisCli.Document) (*schema.Document, error) {
			resp := &schema.Document{
				ID:       doc.ID,
				Content:  "",
				MetaData: map[string]any{},
			}
			for field, val := range doc.Fields {
				if field == redispkg.ContentField {
					resp.Content = val
				} else if field == redispkg.MetadataField {
					resp.MetaData[field] = val
				} else if field == redispkg.DistanceField {
					distance, err := strconv.ParseFloat(val, 64)
					if err != nil {
						continue
					}
					resp.WithScore(1 - distance)
				}
			}

			// 添加来源标记，以便我们知道此文档来自原始检索器
			resp.MetaData["source"] = "original"

			return resp, nil
		},
	}
	embeddingIns, err := newEmbedding(ctx)
	if err != nil {
		return nil, err
	}
	config.Embedding = embeddingIns
	return redis.NewRetriever(ctx, config)
}

// createAmapRetriever 创建高德地图检索器
func createAmapRetriever(ctx context.Context) (retriever.Retriever, error) {
	// 使用pkg/tool/amap中定义的检索器
	return amap.NewAmapRedisRetriever(ctx, nil)
}

// createMultiQueryRetriever 创建整合了多个检索器的MultiQuery检索器
func createMultiQueryRetriever(ctx context.Context, originalRetriever, amapRetriever retriever.Retriever) (retriever.Retriever, error) {
	// 定义查询修改函数
	rewriteHandler := func(ctx context.Context, query string) ([]string, error) {
		// 返回原始查询和一个更强调地理位置的查询
		return []string{
			query,               // 原始查询
			query + " 位置 地点 附近", // 强调地理位置的查询
		}, nil
	}

	// 配置MultiQuery检索器
	config := &multiquery.Config{
		RewriteHandler: rewriteHandler,
		MaxQueriesNum:  2,
		OrigRetriever:  originalRetriever, // 原始检索器作为主检索器
		FusionFunc:     customFusionFunc,  // 自定义融合函数
	}

	return multiquery.NewRetriever(ctx, config)
}

// customFusionFunc 自定义文档融合函数，将多个检索器的结果合并
func customFusionFunc(ctx context.Context, allDocs [][]*schema.Document) ([]*schema.Document, error) {
	if len(allDocs) == 0 {
		return []*schema.Document{}, nil
	}

	// 使用map去重
	uniqueDocs := make(map[string]*schema.Document)

	// 先处理第一个检索器的结果（原始检索器）
	for _, doc := range allDocs[0] {
		uniqueDocs[doc.ID] = doc
	}

	// 然后处理第二个检索器的结果（高德检索器）
	// 确保至少保留一些高德检索器的结果
	amapDocsAdded := 0
	if len(allDocs) > 1 {
		for _, doc := range allDocs[1] {
			if doc.MetaData == nil {
				doc.MetaData = make(map[string]interface{})
			}

			// 如果ID不存在或高德检索结果得分更高，则使用高德检索结果
			docScore := doc.Score()
			if existing, ok := uniqueDocs[doc.ID]; !ok || docScore > existing.Score() {
				uniqueDocs[doc.ID] = doc
				amapDocsAdded++
			}

			// 确保至少添加2个高德检索结果
			if amapDocsAdded >= 2 && len(uniqueDocs) >= 10 {
				break
			}
		}
	}

	// 转换回切片
	mergedDocs := make([]*schema.Document, 0, len(uniqueDocs))
	for _, doc := range uniqueDocs {
		mergedDocs = append(mergedDocs, doc)
	}

	// 按相似度分数排序
	sort.Slice(mergedDocs, func(i, j int) bool {
		return mergedDocs[i].Score() > mergedDocs[j].Score()
	})

	// 限制返回数量
	maxResults := 10
	if len(mergedDocs) > maxResults {
		mergedDocs = mergedDocs[:maxResults]
	}

	return mergedDocs, nil
}
