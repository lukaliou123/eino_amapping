package amap

import (
	"context"
	"testing"
	"time"
)

func TestGetAmapTools(t *testing.T) {
	// 创建一个带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 获取工具
	tools, err := GetAmapTools(ctx)
	if err != nil {
		t.Fatalf("获取高德地图工具失败: %v", err)
	}

	// 验证工具数量
	expectedCount := len(GetToolNames())
	if len(tools) != expectedCount {
		t.Fatalf("期望获取 %d 个工具，实际获取 %d 个", expectedCount, len(tools))
	}

	// 验证每个工具的类型
	for i, tool := range tools {
		if tool == nil {
			t.Fatalf("第 %d 个工具为空", i+1)
		}

		// 验证工具类型
		info, err := tool.Info(ctx)
		if err != nil {
			t.Fatalf("获取第 %d 个工具信息失败: %v", i+1, err)
		}

		t.Logf("工具 %d: %s - %s", i+1, info.Name, info.Desc)
	}

	t.Log("高德地图工具测试通过")
}
