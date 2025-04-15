package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

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

	// 运行详细的高德地图工具测试
	RunAllToolsTests()

	// 运行拦截器测试
	RunInterceptorTestMain()

	fmt.Println("测试完成，按Ctrl+C退出程序")

	// 保持程序运行，直到收到中断信号
	<-ctx.Done()
}
