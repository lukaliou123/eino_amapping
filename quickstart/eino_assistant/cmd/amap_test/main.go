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
)

// testMCPClient 测试高德地图MCP客户端
func testMCPClient(ctx context.Context) error {
	fmt.Println("开始测试高德地图MCP客户端连接...")

	// 尝试创建MCP客户端
	client, err := amap.NewMCPClient(ctx, "")
	if err != nil {
		return fmt.Errorf("创建MCP客户端失败: %v", err)
	}
	defer client.Close()

	fmt.Println("MCP客户端连接和初始化成功！")

	// 获取MCP工具
	fmt.Println("\n开始获取高德地图MCP工具...")
	tools, err := amap.GetMCPTools(ctx, client)
	if err != nil {
		return fmt.Errorf("获取MCP工具失败: %v", err)
	}

	fmt.Printf("\n成功获取了 %d 个高德地图MCP工具！\n", len(tools))
	return nil
}

// testAllTools 测试获取所有工具，包括高德地图工具
func testAllTools(ctx context.Context) error {
	fmt.Println("\n正在获取所有Eino工具...")

	// 获取所有工具
	tools, err := einoagent.GetTools(ctx)
	if err != nil {
		return fmt.Errorf("获取工具失败: %v", err)
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

	fmt.Println("\n验证工具集成成功！")
	return nil
}

func main() {
	// 创建一个可以被取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置信号处理，以便在Ctrl+C时优雅退出
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalCh
		fmt.Println("\n收到退出信号，正在关闭...")
		cancel()
		// 给一点时间让程序正常退出
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()

	fmt.Println("开始测试...")

	// 测试MCP客户端
	if err := testMCPClient(ctx); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	// 测试所有工具
	if err := testAllTools(ctx); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	fmt.Println("测试完成，按Ctrl+C退出程序")

	// 保持程序运行，直到收到中断信号
	<-ctx.Done()
}
