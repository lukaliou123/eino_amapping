package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudwego/eino-examples/quickstart/eino_assistant/eino/einoagent"
	"github.com/cloudwego/eino-examples/quickstart/eino_assistant/pkg/tool/amap"
	"github.com/cloudwego/eino/components/tool"
)

// TestAllTools tests retrieving all tools including Amap tools
func TestAllTools() {
	fmt.Println("开始测试获取所有工具，包括高德地图工具...")

	// 获取所有工具
	ctx := context.Background()
	tools, err := einoagent.GetTools(ctx)
	if err != nil {
		fmt.Printf("获取工具失败: %v\n", err)
		return
	}

	fmt.Printf("\n成功获取了 %d 个工具\n\n", len(tools))

	// 打印工具信息
	fmt.Println("工具列表:")
	for i, tool := range tools {
		info, err := tool.Info(ctx)
		if err != nil {
			fmt.Printf("获取工具 #%d 信息失败: %v\n", i+1, err)
			continue
		}
		fmt.Printf("%3d. %-30s - %s\n", i+1, info.Name, info.Desc)
	}

	// 检查是否包含高德地图工具
	hasAmapTools := false
	for _, tool := range tools {
		info, _ := tool.Info(ctx)
		if info.Name == "maps_direction_bicycling" ||
			info.Name == "maps_direction_driving" ||
			info.Name == "maps_geo" {
			hasAmapTools = true
			break
		}
	}

	if hasAmapTools {
		fmt.Println("\n✓ 成功集成高德地图工具！")
	} else {
		fmt.Println("\n❌ 未找到高德地图工具，请检查MCP客户端配置。")
	}

	fmt.Println("\n测试完成！")
}

func RunAllTests() {
	// 创建一个可取消的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 设置信号处理，以便在收到中断信号时优雅地关闭
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signalChan
		fmt.Println("\n收到中断信号，正在关闭...")
		cancel()
		os.Exit(0)
	}()

	// 使用上下文来执行可能需要超时的操作
	select {
	case <-ctx.Done():
		fmt.Println("操作超时或被取消")
	default:
		// 运行测试函数
		TestAllTools()
	}
}

// RunAllToolsTests 可以从主程序调用的入口函数
func RunAllToolsTests() {
	fmt.Println("\n==== 开始测试所有高德地图工具 ====")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 创建MCP客户端
	client, err := amap.NewMCPClient(ctx, "")
	if err != nil {
		fmt.Printf("创建MCP客户端失败: %v\n", err)
		return
	}
	defer client.Close()

	// 获取高德地图工具
	tools, err := amap.GetMCPTools(ctx, client)
	if err != nil {
		fmt.Printf("获取高德地图工具失败: %v\n", err)
		return
	}

	fmt.Printf("成功获取 %d 个高德地图工具，开始测试各个工具...\n\n", len(tools))

	// 测试地理编码
	testGeocodeTool(ctx, tools)

	// 测试逆地理编码
	testRegeocodeTool(ctx, tools)

	// 测试天气查询
	testWeatherTool(ctx, tools)

	// 测试关键词搜索
	testTextSearchTool(ctx, tools)

	// 测试周边搜索
	testAroundSearchTool(ctx, tools)

	// 测试IP定位
	testIPLocationTool(ctx, tools)

	// 测试距离计算
	testDistanceTool(ctx, tools)

	// 测试驾车规划
	testDrivingTool(ctx, tools)

	fmt.Println("\n==== 所有高德地图工具测试完成 ====")
}

// 查找特定工具
func findTool(ctx context.Context, tools []tool.BaseTool, name string) tool.InvokableTool {
	for _, t := range tools {
		info, _ := t.Info(ctx)
		if info.Name == name {
			if invokable, ok := t.(tool.InvokableTool); ok {
				return invokable
			}
		}
	}
	return nil
}

// 测试地理编码
func testGeocodeTool(ctx context.Context, tools []tool.BaseTool) {
	fmt.Println("测试地理编码工具...")
	geocodeTool := findTool(ctx, tools, "maps_geo")
	if geocodeTool == nil {
		fmt.Println("未找到地理编码工具，跳过测试")
		return
	}

	query := `{"address":"北京市海淀区上地十街10号"}`
	fmt.Printf("执行地理编码查询: %s\n", query)

	result, err := geocodeTool.InvokableRun(ctx, query)
	if err != nil {
		fmt.Printf("地理编码失败: %v\n", err)
	} else if len(result) > 200 {
		fmt.Printf("结果预览: %s...\n", result[:200])
	} else {
		fmt.Printf("结果: %s\n", result)
	}
	fmt.Println()
}

// 测试逆地理编码
func testRegeocodeTool(ctx context.Context, tools []tool.BaseTool) {
	fmt.Println("测试逆地理编码工具...")
	regeocodeTool := findTool(ctx, tools, "maps_regeocode")
	if regeocodeTool == nil {
		fmt.Println("未找到逆地理编码工具，跳过测试")
		return
	}

	query := `{"location":"116.307590,40.058440"}`
	fmt.Printf("执行逆地理编码查询: %s\n", query)

	result, err := regeocodeTool.InvokableRun(ctx, query)
	if err != nil {
		fmt.Printf("逆地理编码失败: %v\n", err)
	} else if len(result) > 200 {
		fmt.Printf("结果预览: %s...\n", result[:200])
	} else {
		fmt.Printf("结果: %s\n", result)
	}
	fmt.Println()
}

// 测试天气查询
func testWeatherTool(ctx context.Context, tools []tool.BaseTool) {
	fmt.Println("测试天气查询工具...")
	weatherTool := findTool(ctx, tools, "maps_weather")
	if weatherTool == nil {
		fmt.Println("未找到天气查询工具，跳过测试")
		return
	}

	query := `{"city":"北京"}`
	fmt.Printf("执行天气查询: %s\n", query)

	result, err := weatherTool.InvokableRun(ctx, query)
	if err != nil {
		fmt.Printf("天气查询失败: %v\n", err)
	} else if len(result) > 200 {
		fmt.Printf("结果预览: %s...\n", result[:200])
	} else {
		fmt.Printf("结果: %s\n", result)
	}
	fmt.Println()
}

// 测试关键词搜索
func testTextSearchTool(ctx context.Context, tools []tool.BaseTool) {
	fmt.Println("测试关键词搜索工具...")
	searchTool := findTool(ctx, tools, "maps_text_search")
	if searchTool == nil {
		fmt.Println("未找到关键词搜索工具，跳过测试")
		return
	}

	query := `{"keywords":"北京西站","city":"北京"}`
	fmt.Printf("执行关键词搜索: %s\n", query)

	result, err := searchTool.InvokableRun(ctx, query)
	if err != nil {
		fmt.Printf("关键词搜索失败: %v\n", err)
	} else if len(result) > 200 {
		fmt.Printf("结果预览: %s...\n", result[:200])
	} else {
		fmt.Printf("结果: %s\n", result)
	}
	fmt.Println()
}

// 测试周边搜索
func testAroundSearchTool(ctx context.Context, tools []tool.BaseTool) {
	fmt.Println("测试周边搜索工具...")
	aroundSearchTool := findTool(ctx, tools, "maps_around_search")
	if aroundSearchTool == nil {
		fmt.Println("未找到周边搜索工具，跳过测试")
		return
	}

	query := `{"keywords":"咖啡","location":"116.307590,40.058440","radius":"1000"}`
	fmt.Printf("执行周边搜索: %s\n", query)

	result, err := aroundSearchTool.InvokableRun(ctx, query)
	if err != nil {
		fmt.Printf("周边搜索失败: %v\n", err)
	} else if len(result) > 200 {
		fmt.Printf("结果预览: %s...\n", result[:200])
	} else {
		fmt.Printf("结果: %s\n", result)
	}
	fmt.Println()
}

// 测试IP定位
func testIPLocationTool(ctx context.Context, tools []tool.BaseTool) {
	fmt.Println("测试IP定位工具...")
	ipLocationTool := findTool(ctx, tools, "maps_ip_location")
	if ipLocationTool == nil {
		fmt.Println("未找到IP定位工具，跳过测试")
		return
	}

	query := `{"ip":"114.247.50.2"}`
	fmt.Printf("执行IP定位: %s\n", query)

	result, err := ipLocationTool.InvokableRun(ctx, query)
	if err != nil {
		fmt.Printf("IP定位失败: %v\n", err)
	} else if len(result) > 200 {
		fmt.Printf("结果预览: %s...\n", result[:200])
	} else {
		fmt.Printf("结果: %s\n", result)
	}
	fmt.Println()
}

// 测试距离计算
func testDistanceTool(ctx context.Context, tools []tool.BaseTool) {
	fmt.Println("测试距离计算工具...")
	distanceTool := findTool(ctx, tools, "maps_distance")
	if distanceTool == nil {
		fmt.Println("未找到距离计算工具，跳过测试")
		return
	}

	query := `{"origins":"116.307590,40.058440","destination":"116.397428,39.909187"}`
	fmt.Printf("执行距离计算: %s\n", query)

	result, err := distanceTool.InvokableRun(ctx, query)
	if err != nil {
		fmt.Printf("距离计算失败: %v\n", err)
	} else if len(result) > 200 {
		fmt.Printf("结果预览: %s...\n", result[:200])
	} else {
		fmt.Printf("结果: %s\n", result)
	}
	fmt.Println()
}

// 测试驾车规划
func testDrivingTool(ctx context.Context, tools []tool.BaseTool) {
	fmt.Println("测试驾车规划工具...")
	drivingTool := findTool(ctx, tools, "maps_direction_driving")
	if drivingTool == nil {
		fmt.Println("未找到驾车规划工具，跳过测试")
		return
	}

	query := `{"origin":"116.307590,40.058440","destination":"116.397428,39.909187"}`
	fmt.Printf("执行驾车规划: %s\n", query)

	result, err := drivingTool.InvokableRun(ctx, query)
	if err != nil {
		fmt.Printf("驾车规划失败: %v\n", err)
	} else if len(result) > 200 {
		fmt.Printf("结果预览: %s...\n", result[:200])
	} else {
		fmt.Printf("结果: %s\n", result)
	}
	fmt.Println()
}
