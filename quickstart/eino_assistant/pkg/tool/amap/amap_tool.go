package amap

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
)

// GetAmapTools 获取所有高德地图MCP工具，供Eino使用
func GetAmapTools(ctx context.Context) ([]tool.BaseTool, error) {
	// 创建MCP客户端
	client, err := NewMCPClient(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("创建MCP客户端失败: %w", err)
	}

	// 获取高德地图工具
	tools, err := GetMCPTools(ctx, client)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("获取高德地图工具失败: %w", err)
	}

	// 注意：理想情况下，我们应该在某个地方保存client的引用
	// 以便程序结束时可以关闭它，而不是让它在垃圾回收中关闭
	// 在实际应用中，可能需要在应用级别维护这个客户端的生命周期

	return tools, nil
}

// GetToolNames 获取所有高德地图工具的名称
func GetToolNames() []string {
	return ToolNamesToFetch
}
