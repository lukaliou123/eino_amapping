package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudwego/eino-examples/quickstart/eino_assistant/pkg/tool/amap"
)

func TestPrintToolNames(t *testing.T) {
	// 创建上下文
	ctx := context.Background()

	// 获取高德地图工具
	tools, err := amap.GetInterceptedAmapTools(ctx)
	if err != nil {
		t.Fatalf("获取高德地图工具失败: %v", err)
	}

	fmt.Printf("成功获取 %d 个高德地图工具\n", len(tools))

	// 打印工具名称
	for i, tool := range tools {
		info, err := tool.Info(ctx)
		if err != nil {
			t.Fatalf("获取工具 #%d 信息失败: %v", i+1, err)
		}
		fmt.Printf("%d. %s - %s\n", i+1, info.Name, info.Desc)
	}
}
