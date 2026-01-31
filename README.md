# LogLite - 轻量级日志平台

🚀 一个面向 Go 开发者的轻量级、易用的日志收集和查询平台。

## ✨ 特性

- **零外部依赖**：单个二进制文件，无需 Elasticsearch、Kafka
- **极简 API**：像 `fmt.Println` 一样简单
- **批量异步发送**：不影响业务性能
- **自动上下文采集**：自动记录调用位置、堆栈信息
- **美观的 Web UI**：开箱即用的日志查看界面
- **框架集成**：支持 Gin、标准 HTTP 等

## 🚀 快速开始

### 1. 启动服务端

```bash
# 克隆项目
git clone https://github.com/loglite/loglite.git
cd loglite

# 下载依赖
go mod tidy

# 编译
go build -o loglite ./cmd/server

# 运行
./loglite
```

服务启动后访问：http://localhost:8080

### 2. 集成到你的项目

```bash
go get github.com/loglite/loglite/pkg/sdk
```

```go
package main

import (
    "github.com/loglite/loglite/pkg/sdk"
)

func main() {
    // 一行初始化
    logger := sdk.Init("my-service",
        sdk.WithEndpoint("http://localhost:8080"),
    )
    defer logger.Close()

    // 记录日志
    logger.Info("服务启动", "port", 8080)
    logger.Error("发生错误", "error", err, "user_id", 123)
}
```

### 3. Gin 框架集成

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/loglite/loglite/pkg/sdk"
)

func main() {
    client := sdk.Init("my-service")
    defer client.Close()
    sdk.SetDefault(client)

    r := gin.New()
    r.Use(sdk.GinMiddleware())  // 自动记录请求日志
    r.Use(sdk.GinRecovery())    // Panic 恢复并记录

    r.GET("/api/users", func(c *gin.Context) {
        // 从 context 获取 logger（自动带上 trace_id）
        logger := sdk.FromContext(c.Request.Context())
        logger.Info("处理请求", "user_id", c.Query("id"))
    })

    r.Run(":8080")
}
```

## 📡 API 接口

### 发送日志

```bash
# 单条日志
curl -X POST http://localhost:8080/api/v1/logs \
  -H "Content-Type: application/json" \
  -d '{
    "message": "用户登录成功",
    "level": "info",
    "service": "auth-service",
    "user_id": "123"
  }'

# 批量日志
curl -X POST http://localhost:8080/api/v1/logs/batch \
  -H "Content-Type: application/json" \
  -d '{
    "logs": [
      {"message": "日志1", "level": "info", "service": "test"},
      {"message": "日志2", "level": "warn", "service": "test"}
    ]
  }'
```

### 查询日志

```bash
# 简单查询
curl "http://localhost:8080/api/v1/query?service=auth-service&level=error"

# 关键词搜索
curl "http://localhost:8080/api/v1/query?q=登录&limit=50"

# 时间范围
curl "http://localhost:8080/api/v1/query?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z"
```

## ⚙️ 配置

创建 `config.yaml`：

```yaml
server:
  port: 8080
  host: "0.0.0.0"

storage:
  data_dir: "./data"
  retention: 168h  # 7 天

query:
  default_limit: 100
  max_limit: 1000
```

## 📁 项目结构

```
loglite/
├── cmd/server/          # 服务端入口
├── internal/
│   ├── api/             # HTTP API
│   ├── config/          # 配置管理
│   ├── model/           # 数据模型
│   └── storage/         # 存储层 (BadgerDB)
├── pkg/sdk/             # Go SDK
├── web/templates/       # Web UI
├── examples/            # 使用示例
├── config.yaml          # 配置文件
└── README.md
```

## 🛠️ SDK 配置选项

```go
client := sdk.Init("service-name",
    sdk.WithEndpoint("http://localhost:8080"),  // 服务端地址
    sdk.WithBatchSize(100),                      // 批量大小
    sdk.WithFlushInterval(100*time.Millisecond), // 刷新间隔
    sdk.WithCaller(true),                        // 采集调用位置
    sdk.WithStackTrace(true),                    // Error 级别采集堆栈
    sdk.WithFallbackFile("/var/log/app.log"),   // 降级文件
)
```

## 📝 License

MIT License
