package amap

import (
	"context"
	"testing"
	"time"
)

func TestMCPClientConnection(t *testing.T) {
	// 创建一个带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 尝试创建一个默认的MCP客户端
	client, err := NewMCPClient(ctx, "")
	if err != nil {
		t.Fatalf("创建MCP客户端失败: %v", err)
	}
	defer client.Close()

	// 如果能走到这一步，说明连接和初始化成功
	t.Log("MCP客户端连接和初始化成功")

	// 验证客户端不为空
	if client.GetClient() == nil {
		t.Fatal("MCP客户端实例为空")
	}

	t.Log("MCP客户端测试通过")
}
