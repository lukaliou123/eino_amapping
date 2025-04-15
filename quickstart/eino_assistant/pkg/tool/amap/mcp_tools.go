package amap

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
)

// 我们想要获取的MCP工具名称列表
// 注意：这些名称可能需要根据实际可用的工具进行调整
var ToolNamesToFetch = []string{
	"maps_text_search",                  // 文本搜索
	"maps_around_search",                // 周边搜索
	"maps_search_detail",                // POI详情
	"maps_geo",                          // 地理编码
	"maps_regeocode",                    // 逆地理编码
	"maps_weather",                      // 天气查询
	"maps_direction_driving",            // 驾车路径规划
	"maps_direction_walking",            // 步行路径规划
	"maps_direction_bicycling",          // 骑行路径规划
	"maps_direction_transit_integrated", // 公交路径规划
	"maps_distance",                     // 距离测量
	"maps_ip_location",                  // IP定位
}

// GetMCPTools 获取MCP工具
func GetMCPTools(ctx context.Context, mcpClient *MCPClient) ([]tool.BaseTool, error) {
	if mcpClient == nil {
		// 如果没有提供客户端，则创建一个新的客户端
		var err error
		mcpClient, err = NewMCPClient(ctx, DefaultMCPSSEURL)
		if err != nil {
			return nil, fmt.Errorf("创建MCP客户端失败: %w", err)
		}
	}

	// 获取底层客户端
	cli := mcpClient.GetClient()

	// 获取工具列表
	fmt.Println("正在获取MCP工具列表...")
	tools, err := mcp.GetTools(ctx, &mcp.Config{
		Cli:          cli,
		ToolNameList: ToolNamesToFetch,
	})
	if err != nil {
		return nil, fmt.Errorf("获取MCP工具失败: %w", err)
	}

	fmt.Printf("成功获取 %d 个MCP工具\n", len(tools))

	// 打印获取到的工具名称
	for i, t := range tools {
		info, _ := t.Info(ctx)
		fmt.Printf("%d. %s - %s\n", i+1, info.Name, info.Desc)
	}

	return tools, nil
}
