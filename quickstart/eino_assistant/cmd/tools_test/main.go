package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cloudwego/eino-examples/quickstart/eino_assistant/eino/einoagent"
)

// 测试获取所有工具，包括高德地图工具
func main() {
	// 创建一个带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("正在获取所有Eino工具...")

	// 获取所有工具
	tools, err := einoagent.GetTools(ctx)
	if err != nil {
		fmt.Printf("获取工具失败: %v\n", err)
		os.Exit(1)
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
}
