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
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/model"
)

// CurlLogger 是一个 HTTP 中间件，用于将请求转换为 curl 命令并打印出来
type CurlLogger struct {
	// 下一个处理器
	next http.RoundTripper
	// 日志输出函数
	logf func(format string, v ...interface{})
}

// NewCurlLogger 创建一个新的 CurlLogger 中间件
func NewCurlLogger(next http.RoundTripper, logf func(format string, v ...interface{})) *CurlLogger {
	if logf == nil {
		logf = func(format string, v ...interface{}) {
			fmt.Printf(format, v...)
		}
	}
	return &CurlLogger{
		next: next,
		logf: logf,
	}
}

// RoundTrip 实现 http.RoundTripper 接口
func (c *CurlLogger) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := c.next.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, fmt.Errorf("error reading response body: %v", err)
	}
	c.logf("%s", string(body))
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp, nil
}

// generateCurlCommand 将 HTTP 请求转换为等效的 curl 命令
func generateCurlCommand(req *http.Request) string {
	var command strings.Builder
	// 基础 curl 命令
	command.WriteString("curl -X " + req.Method)
	// 添加 URL
	command.WriteString(" '" + req.URL.String() + "'")
	// 添加请求头
	for key, values := range req.Header {
		for _, value := range values {
			command.WriteString(fmt.Sprintf(" -H '%s: %s'", key, value))
		}
	}
	// 添加请求体
	if req.Body != nil && (req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH") {
		var bodyBytes []byte
		// 保存原始请求体
		if req.GetBody != nil {
			bodyReadCloser, err := req.GetBody()
			if err == nil {
				bodyBytes, _ = io.ReadAll(bodyReadCloser)
				bodyReadCloser.Close()
			}
		} else if req.Body != nil {
			// 如果没有 GetBody，则尝试读取 Body，但这会消耗 Body
			bodyBytes, _ = io.ReadAll(req.Body)
			req.Body.Close()
			// 重新设置 Body 以便后续处理
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
		if len(bodyBytes) > 0 {
			bodyStr := string(bodyBytes)
			// 转义单引号
			bodyStr = strings.ReplaceAll(bodyStr, "'", "'\\''")
			command.WriteString(" -d '" + bodyStr + "'")
		}
	}
	return command.String()
}

func newChatModel(ctx context.Context) (cm model.ChatModel, err error) {
	// 配置带有日志记录功能的HTTP客户端
	httpClient := &http.Client{
		Transport: NewCurlLogger(http.DefaultTransport, log.Printf),
	}

	// 模型配置
	config := &ark.ChatModelConfig{
		Model:      os.Getenv("ARK_CHAT_MODEL"),
		APIKey:     os.Getenv("ARK_API_KEY"),
		HTTPClient: httpClient, // 设置HTTP客户端
	}

	log.Println("正在初始化带有CURL日志的聊天模型...")
	cm, err = ark.NewChatModel(ctx, config)
	if err != nil {
		return nil, err
	}
	log.Println("聊天模型初始化成功，所有HTTP请求将记录为CURL命令")

	return cm, nil
}
