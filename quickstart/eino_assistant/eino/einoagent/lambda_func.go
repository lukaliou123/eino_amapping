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
	"log"
	"time"

	"github.com/cloudwego/eino/schema"
)

// newLambda component initialization function of node 'InputToQuery' in graph 'EinoAgent'
func newLambda(ctx context.Context, input *UserMessage, opts ...any) (output string, err error) {
	return input.Query, nil
}

// newLambda2 component initialization function of node 'InputToHistory' in graph 'EinoAgent'
func newLambda2(ctx context.Context, input *UserMessage, opts ...any) (output map[string]any, err error) {
	// 尝试从上下文获取历史记录管理器
	historyManager, ok := ctx.Value(HistoryKey).(HistoryManager)

	// 准备返回数据
	result := map[string]any{
		"content": input.Query,
		"date":    time.Now().Format("2006-01-02 15:04:05"),
	}

	// 如果没有历史记录管理或会话ID为空，使用内存中的历史记录
	if !ok || input.ID == "" {
		result["history"] = input.History
		return result, nil
	}

	// 从历史记录管理中获取用户聊天记录
	chatHistory, err := historyManager.GetHistory(input.ID, 100)
	if err != nil {
		log.Printf("获取历史记录失败: %v", err)
		// 如果获取失败，使用内存中的历史记录
		result["history"] = input.History
		return result, nil
	}

	// 保存用户消息到历史记录
	err = historyManager.SaveMessage(&schema.Message{
		Role:    schema.User,
		Content: input.Query,
	}, input.ID)
	if err != nil {
		log.Printf("保存用户消息失败: %v", err)
	}

	// 使用获取到的历史记录
	result["history"] = chatHistory
	return result, nil
}

// SaveAssistantMessage 保存助手的回复到历史记录中
func SaveAssistantMessage(ctx context.Context, message *schema.Message, conversationID string) error {
	// 尝试从上下文获取历史记录管理器
	historyManager, ok := ctx.Value(HistoryKey).(HistoryManager)
	if !ok || conversationID == "" {
		log.Printf("跳过保存助手回复: 历史记录管理器不存在或会话ID为空")
		return nil
	}

	// 保存助手回复到历史记录
	err := historyManager.SaveMessage(message, conversationID)
	if err != nil {
		log.Printf("保存助手回复失败: %v", err)
		return err
	}

	log.Printf("成功保存助手回复到历史记录, ID: %s", conversationID)
	return nil
}
