package amap

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/components/tool"
)

// MCPDefaultURL 默认的高德地图MCP SSE URL (重命名避免冲突)
const MCPDefaultURL = "https://mcp.amap.com/sse?key=f145e25c5ac6ba84fe217036979181d2"

// 是否启用POI数据拦截
var EnablePOIInterceptor bool

func init() {
	// 从环境变量读取是否启用拦截器
	enableStr := os.Getenv("ENABLE_POI_INTERCEPTOR")
	EnablePOIInterceptor = enableStr == "true" || enableStr == "1"
}

// GetInterceptedAmapTools 获取带有拦截器的高德地图工具 (新函数名避免冲突)
func GetInterceptedAmapTools(ctx context.Context) ([]tool.BaseTool, error) {
	// 创建高德MCP客户端
	log.Println("正在连接高德地图MCP服务...")

	// 获取自定义URL或使用默认URL
	mcpURL := os.Getenv("AMAP_MCP_URL")
	if mcpURL == "" {
		mcpURL = MCPDefaultURL
	}

	// 创建MCP SSE客户端
	cli, err := NewMCPClient(ctx, mcpURL)

	// 获取工具列表
	log.Println("正在获取高德地图工具列表...")
	tools, err := GetMCPTools(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("获取高德地图工具失败: %w", err)
	}

	log.Printf("成功获取 %d 个高德地图工具\n", len(tools))

	// 如果启用了POI拦截器，则包装工具
	if EnablePOIInterceptor {
		log.Println("已启用POI数据拦截，将自动提取API返回数据")
		wrappedTools := make([]tool.BaseTool, len(tools))

		for i, t := range tools {
			// 包装工具添加拦截器
			wrappedTools[i] = WrapWithInterceptor(t)
			info, _ := t.Info(ctx)
			log.Printf("已为工具 %s 添加拦截器", info.Name)
		}

		// 打印工具信息
		for i, t := range wrappedTools {
			info, _ := t.Info(ctx)
			log.Printf("%d. %s - %s (带拦截器)\n", i+1, info.Name, info.Desc)
		}

		return wrappedTools, nil
	}

	// 如果不启用拦截器，则返回原始工具
	// 打印工具信息
	for i, t := range tools {
		info, _ := t.Info(ctx)
		log.Printf("%d. %s - %s\n", i+1, info.Name, info.Desc)
	}

	return tools, nil
}

// 注意: 这里不再覆盖GetAmapTools函数
// 而是提供一个新函数，让调用者自行决定使用哪个版本
