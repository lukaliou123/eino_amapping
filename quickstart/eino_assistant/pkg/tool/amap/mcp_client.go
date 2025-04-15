package amap

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	mcplib "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPClient 封装了与高德MCP通信的功能
type MCPClient struct {
	cli   mcplib.MCPClient     // 使用具体的MCPClient类型
	tools []tool.InvokableTool // 存储可调用的工具
}

// DefaultMCPSSEURL 默认的高德地图MCP SSE URL
const DefaultMCPSSEURL = "https://mcp.amap.com/sse?key=f145e25c5ac6ba84fe217036979181d2"

// NewMCPClient 创建一个新的MCP客户端
func NewMCPClient(ctx context.Context, baseURL string) (*MCPClient, error) {
	if baseURL == "" {
		baseURL = DefaultMCPSSEURL
	}

	fmt.Printf("正在连接MCP服务: %s\n", baseURL)

	// 创建SSE客户端
	cli, err := mcplib.NewSSEMCPClient(baseURL)
	if err != nil {
		return nil, fmt.Errorf("创建MCP SSE客户端失败: %w", err)
	}

	// 启动客户端，建立连接
	fmt.Println("正在启动MCP客户端...")
	err = cli.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("启动MCP客户端失败: %w", err)
	}
	fmt.Println("MCP客户端连接成功")

	// 初始化客户端
	fmt.Println("正在初始化MCP客户端...")
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "eino-amap-client",
		Version: "1.0.0",
	}
	initResponse, err := cli.Initialize(ctx, initRequest)
	if err != nil {
		return nil, fmt.Errorf("初始化MCP客户端失败: %w", err)
	}
	fmt.Printf("MCP客户端初始化成功，服务器名称: %s，协议版本: %s\n",
		initResponse.ServerInfo.Name,
		initResponse.ProtocolVersion)

	return &MCPClient{
		cli:   cli,
		tools: []tool.InvokableTool{},
	}, nil
}

// GetClient 返回底层的MCP客户端
func (m *MCPClient) GetClient() mcplib.MCPClient {
	return m.cli
}

// Close 关闭MCP客户端连接
func (m *MCPClient) Close() error {
	if cli, ok := m.cli.(interface{ Close() error }); ok {
		return cli.Close()
	}
	return nil
}
