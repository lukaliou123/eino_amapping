package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino-examples/quickstart/eino_assistant/pkg/tool/amap"
	"github.com/cloudwego/eino/components/tool"
)

// TestInterceptor 测试高德API拦截器功能
func TestInterceptor(t *testing.T) {
	// 设置上下文
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 启用POI数据拦截
	os.Setenv("ENABLE_POI_INTERCEPTOR", "true")

	// 获取带拦截器的高德地图工具
	fmt.Println("正在获取带拦截器的高德地图工具...")
	tools, err := amap.GetInterceptedAmapTools(ctx)
	if err != nil {
		fmt.Printf("获取工具失败: %v\n", err)
		return
	}

	fmt.Printf("成功获取 %d 个高德地图工具\n", len(tools))

	// 选择文本搜索工具进行测试
	var searchTool tool.InvokableTool
	for _, t := range tools {
		info, _ := t.Info(ctx)
		if info.Name == "maps_text_search" {
			if invokable, ok := t.(tool.InvokableTool); ok {
				searchTool = invokable
				break
			}
		}
	}

	if searchTool == nil {
		fmt.Println("未找到文本搜索工具，无法继续测试")
		return
	}

	// 执行搜索测试
	fmt.Println("\n开始测试POI搜索功能...")
	searchQuery := `{"keywords":"北京西站","city":"北京"}`

	fmt.Printf("执行搜索: %s\n", searchQuery)
	result, err := searchTool.InvokableRun(ctx, searchQuery)
	if err != nil {
		fmt.Printf("搜索失败: %v\n", err)
		return
	}

	// 打印返回结果的前200个字符
	if len(result) > 200 {
		fmt.Printf("结果预览: %s...\n", result[:200])
	} else {
		fmt.Printf("结果: %s\n", result)
	}

	fmt.Println("搜索成功! 拦截器已在后台提取POI数据")
	fmt.Println("等待5秒钟，观察拦截器日志...")
	time.Sleep(5 * time.Second)

	fmt.Println("\n测试完成!")
}
