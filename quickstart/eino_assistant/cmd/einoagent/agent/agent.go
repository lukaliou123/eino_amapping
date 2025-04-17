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

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/cloudwego/eino-ext/callbacks/apmplus"
	"github.com/cloudwego/eino-ext/callbacks/langfuse"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-examples/quickstart/eino_assistant/eino/einoagent"
	"github.com/cloudwego/eino-examples/quickstart/eino_assistant/pkg/mem"
	chathistory "github.com/wangle201210/chat-history/eino"
)

var memory = mem.GetDefaultMemory()

var cbHandler callbacks.Handler

var once sync.Once

// HistoryManager 是一个接口，定义了历史记录管理器需要实现的方法
type HistoryManager interface {
	// GetHistory 获取指定会话的历史记录
	GetHistory(conversationID string, limit int) ([]*schema.Message, error)
	// SaveMessage 保存消息到指定会话
	SaveMessage(message *schema.Message, conversationID string) error
}

// MemoryHistoryManager 实现HistoryManager接口，基于内存存储
type MemoryHistoryManager struct {
	memory *mem.SimpleMemory
}

func NewMemoryHistoryManager(memory *mem.SimpleMemory) *MemoryHistoryManager {
	return &MemoryHistoryManager{memory: memory}
}

func (m *MemoryHistoryManager) GetHistory(conversationID string, limit int) ([]*schema.Message, error) {
	conv := m.memory.GetConversation(conversationID, false)
	if conv == nil {
		return []*schema.Message{}, nil
	}
	messages := conv.GetMessages()
	if limit > 0 && len(messages) > limit {
		return messages[len(messages)-limit:], nil
	}
	return messages, nil
}

func (m *MemoryHistoryManager) SaveMessage(message *schema.Message, conversationID string) error {
	conv := m.memory.GetConversation(conversationID, true)
	conv.Append(message)
	return nil
}

// MySQLHistoryManager 实现HistoryManager接口，基于MySQL存储
type MySQLHistoryManager struct {
	history *chathistory.History
}

func NewMySQLHistoryManager(dsn string) (*MySQLHistoryManager, error) {
	history := chathistory.NewEinoHistory(dsn)
	return &MySQLHistoryManager{
		history: history,
	}, nil
}

func (m *MySQLHistoryManager) GetHistory(conversationID string, limit int) ([]*schema.Message, error) {
	return m.history.GetHistory(conversationID, limit)
}

func (m *MySQLHistoryManager) SaveMessage(message *schema.Message, conversationID string) error {
	return m.history.SaveMessage(message, conversationID)
}

// 全局历史管理器实例
var historyManager HistoryManager = NewMemoryHistoryManager(memory)

func Init() error {
	var err error
	once.Do(func() {
		os.MkdirAll("log", 0755)
		var f *os.File
		f, err = os.OpenFile("log/eino.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return
		}

		cbConfig := &LogCallbackConfig{
			Detail: true,
			Writer: f,
		}
		if os.Getenv("DEBUG") == "true" {
			cbConfig.Debug = true
		}
		// this is for invoke option of WithCallback
		cbHandler = LogCallback(cbConfig)

		// init global callback, for trace and metrics
		callbackHandlers := make([]callbacks.Handler, 0)
		if os.Getenv("APMPLUS_APP_KEY") != "" {
			region := os.Getenv("APMPLUS_REGION")
			if region == "" {
				region = "cn-beijing"
			}
			fmt.Println("[eino agent] INFO: use apmplus as callback, watch at: https://console.volcengine.com/apmplus-server")
			cbh, _, err := apmplus.NewApmplusHandler(&apmplus.Config{
				Host:        fmt.Sprintf("apmplus-%s.volces.com:4317", region),
				AppKey:      os.Getenv("APMPLUS_APP_KEY"),
				ServiceName: "eino-assistant",
				Release:     "release/v0.0.1",
			})
			if err != nil {
				log.Fatal(err)
			}

			callbackHandlers = append(callbackHandlers, cbh)
		}

		if os.Getenv("LANGFUSE_PUBLIC_KEY") != "" && os.Getenv("LANGFUSE_SECRET_KEY") != "" {
			fmt.Println("[eino agent] INFO: use langfuse as callback, watch at: https://cloud.langfuse.com")
			cbh, _ := langfuse.NewLangfuseHandler(&langfuse.Config{
				Host:      "https://cloud.langfuse.com",
				PublicKey: os.Getenv("LANGFUSE_PUBLIC_KEY"),
				SecretKey: os.Getenv("LANGFUSE_SECRET_KEY"),
				Name:      "Eino Assistant",
				Public:    true,
				Release:   "release/v0.0.1",
				UserID:    "eino_god",
				Tags:      []string{"eino", "assistant"},
			})
			callbackHandlers = append(callbackHandlers, cbh)
		}
		if len(callbackHandlers) > 0 {
			callbacks.InitCallbackHandlers(callbackHandlers)
		}

		// 初始化历史记录管理器
		initHistoryManager()
	})
	return err
}

// 初始化历史记录管理
func initHistoryManager() {
	mysqlDSN := os.Getenv("MYSQL_DSN")
	if mysqlDSN == "" {
		log.Println("MySQL DSN not set, using memory history manager")
		return
	}

	// 尝试创建MySQL历史管理器
	mysqlManager, err := NewMySQLHistoryManager(mysqlDSN)
	if err != nil {
		log.Printf("Failed to initialize MySQL history manager: %v, fallback to memory", err)
		return
	}

	historyManager = mysqlManager
	log.Printf("Using MySQL history manager with DSN: %s", mysqlDSN)
}

func RunAgent(ctx context.Context, id string, msg string) (*schema.StreamReader[*schema.Message], error) {
	// 构建智能体，传入历史记录管理器
	runner, err := einoagent.BuildEinoAgent(ctx, historyManager)
	if err != nil {
		return nil, fmt.Errorf("failed to build agent graph: %w", err)
	}

	// 从历史管理器获取历史记录
	messages, err := historyManager.GetHistory(id, 20)
	if err != nil {
		log.Printf("Failed to get history: %v, using empty history", err)
		messages = []*schema.Message{}
	}

	userMessage := &einoagent.UserMessage{
		ID:      id,
		Query:   msg,
		History: messages,
	}

	sr, err := runner.Stream(ctx, userMessage, compose.WithCallbacks(cbHandler))
	if err != nil {
		return nil, fmt.Errorf("failed to stream: %w", err)
	}

	srs := sr.Copy(2)

	go func() {
		// for save to memory
		fullMsgs := make([]*schema.Message, 0)

		defer func() {
			// close stream if you used it
			srs[1].Close()

			// 保存用户消息
			userMsg := schema.UserMessage(msg)
			if err := historyManager.SaveMessage(userMsg, id); err != nil {
				log.Printf("Failed to save user message: %v", err)
			}

			fullMsg, err := schema.ConcatMessages(fullMsgs)
			if err != nil {
				fmt.Println("error concatenating messages: ", err.Error())
				return
			}

			// 保存助手回复
			if err := historyManager.SaveMessage(fullMsg, id); err != nil {
				log.Printf("Failed to save assistant message: %v", err)
			}
		}()

	outer:
		for {
			select {
			case <-ctx.Done():
				fmt.Println("context done", ctx.Err())
				return
			default:
				chunk, err := srs[1].Recv()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break outer
					}
				}

				fullMsgs = append(fullMsgs, chunk)
			}
		}
	}()

	return srs[0], nil
}

type LogCallbackConfig struct {
	Detail bool
	Debug  bool
	Writer io.Writer
}

func LogCallback(config *LogCallbackConfig) callbacks.Handler {
	if config == nil {
		config = &LogCallbackConfig{
			Detail: true,
			Writer: os.Stdout,
		}
	}
	if config.Writer == nil {
		config.Writer = os.Stdout
	}
	builder := callbacks.NewHandlerBuilder()
	builder.OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
		fmt.Fprintf(config.Writer, "[view]: start [%s:%s:%s]\n", info.Component, info.Type, info.Name)
		if config.Detail {
			var b []byte
			if config.Debug {
				b, _ = json.MarshalIndent(input, "", "  ")
			} else {
				b, _ = json.Marshal(input)
			}
			fmt.Fprintf(config.Writer, "%s\n", string(b))
		}
		return ctx
	})
	builder.OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
		fmt.Fprintf(config.Writer, "[view]: end [%s:%s:%s]\n", info.Component, info.Type, info.Name)
		return ctx
	})
	return builder.Build()
}
