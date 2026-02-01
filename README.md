# LogLite - 轻量级日志平台

🚀 一个面向 Go 开发者的轻量级、易用的日志收集和查询平台。

## ✨ 特性

- **零外部依赖**：单个二进制文件，无需 Elasticsearch、Kafka
- **OpenTelemetry 风格 API**：轻量级 Logger，多服务共享 Provider
- **三种可靠性级别**：BestEffort / Balanced / Reliable
- **自动上下文采集**：自动记录调用位置、堆栈信息
- **美观的 Web UI**：开箱即用的日志查看界面
- **框架集成**：支持 Gin、标准 HTTP、GORM

## 🚀 快速开始

### 1. 启动服务端

```bash
git clone https://github.com/loglite/loglite.git
cd loglite
go mod tidy
go build -o loglite ./cmd/server
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
    "github.com/loglite/loglite/pkg/sdk/sender"
)

func main() {
    // 1. 创建 LoggerProvider（重量级，只创建一次）
    provider := sdk.NewLoggerProvider(
        sdk.WithEndpoint("http://localhost:8080"),
        sdk.WithReliability(sender.Balanced),
    )
    defer provider.Shutdown()

    // 2. 获取 Logger（轻量级，可创建多个）
    userLogger := provider.Logger("user-service")
    paymentLogger := provider.Logger("payment-service")

    // 3. 记录日志
    userLogger.Info("用户登录", "user_id", 12345)
    paymentLogger.Info("支付成功", "order_id", "ORD-001", "amount", 99.99)
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
    // 创建 Provider 和 Logger
    provider := sdk.NewLoggerProvider(
        sdk.WithEndpoint("http://localhost:8080"),
    )
    defer provider.Shutdown()

    logger := provider.Logger("my-service")
    sdk.SetDefaultLogger(logger)

    r := gin.New()
    r.Use(sdk.GinMiddleware())  // 自动记录请求日志
    r.Use(sdk.GinRecovery())    // Panic 恢复并记录

    r.GET("/api/users", func(c *gin.Context) {
        // 从 context 获取 logger（自动带上 trace_id）
        ctxLogger := sdk.FromContext(c.Request.Context())
        if ctxLogger != nil {
            ctxLogger.Info("处理请求", "user_id", c.Query("id"))
        }
    })

    r.Run(":8080")
}
```

## 🔧 可靠性级别

```go
// BestEffort - 高性能（开发/测试环境）
provider := sdk.NewLoggerProvider(
    sdk.WithReliability(sender.BestEffort),
)

// Balanced - 平衡（默认，生产环境）
provider := sdk.NewLoggerProvider(
    sdk.WithReliability(sender.Balanced),
    sdk.WithFallbackFile("./logs/fallback.log"),
)

// Reliable - 零丢失（关键业务）
provider := sdk.NewLoggerProvider(
    sdk.WithReliability(sender.Reliable),
    sdk.WithWALDir("./logs/wal"),
)
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
    "service": "auth-service"
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
│   └── sender/          # 发送器（三种可靠性级别）
├── web/templates/       # Web UI
├── examples/            # 使用示例
└── README.md
```

## 🛠️ SDK 配置选项

```go
provider := sdk.NewLoggerProvider(
    sdk.WithEndpoint("http://localhost:8080"),   // 服务端地址
    sdk.WithReliability(sender.Balanced),        // 可靠性级别
    sdk.WithBatchSize(100),                      // 批量大小
    sdk.WithFlushInterval(100*time.Millisecond), // 刷新间隔
    sdk.WithCaller(true),                        // 采集调用位置
    sdk.WithStackTrace(true),                    // Error 级别采集堆栈
    sdk.WithFallbackFile("./logs/fallback.log"), // 降级文件
    sdk.WithWALDir("./logs/wal"),                // WAL 目录
)
```

## 📝 License

MIT License
