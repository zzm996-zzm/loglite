# LogLite - 开发者友好的日志平台

📋 项目文档 v1.0

---

## 一、项目初衷与核心理念

### 为什么创建 LogLite？

作为一名 Go 开发者，我经历了这些痛点：

- 调试时只能 `tail -f` 看日志，难以过滤和搜索
- Elasticsearch 查询语法复杂，记不住 DSL
- 现有方案太重，个人项目用不起
- 接入麻烦，配置复杂

### 设计原则（永恒不变）

1. **查询零学习成本**：像用 Google 一样简单
2. **轻量级但功能完整**：一台服务器就能跑
3. **Go 开发者优先**：从 Go 生态开始，完美集成
4. **渐进式扩展**：个人项目→小团队→大厂都能用
5. **保持简单**：API 简单，配置简单，部署简单

### 不做什么（避免范围蔓延）

- ❌ 不成为另一个 Elasticsearch/Kibana
- ❌ 不追求处理 PB 级数据（初期）
- ❌ 不要求集群部署（单机优先）
- ❌ 不依赖复杂的外部组件

---

## 二、架构设计（核心记忆）

### 整体架构

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│    App      │    │   App       │    │    App      │
│   (Go)      │    │  (其他语言)  │    │   (Go)      │
└─────┬───────┘    └──────┬──────┘    └──────┬──────┘
      │HTTP/GRPC          │HTTP              │SDK直连
      │                   │                  │
      ▼                   ▼                  ▼
┌───────────────────────────────────────────────────┐
│              LogLite Core（单进程）                │
│                                                   │
│  ┌─────────┐  ┌─────────┐  ┌─────────────────┐  │
│  │接收器    │  │处理器    │  │查询引擎         │  │
│  │ - HTTP  │  │ - 解析   │  │ - 自然语言      │  │
│  │ - SDK   │  │ - 索引   │  │ - 结构化        │  │
│  └─────────┘  └─────────┘  └─────────────────┘  │
│                                                   │
│  ┌─────────────────────────────────────────────┐  │
│  │              存储层                          │  │
│  │  - BadgerDB（热数据）                        │  │
│  │  - SQLite（温数据）                          │  │
│  │  - 文件系统（冷数据/归档）                   │  │
│  └─────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────┘
```

### 技术选型（坚持轻量）

- **语言**：Go（我们最熟悉的）
- **存储**：BadgerDB + SQLite（零外部依赖）
- **索引**：自己实现倒排 + Bleve（可选）
- **Web UI**：Go Template 或 Vue.js（轻量版）
- **CLI**：Cobra

---

## 三、API 设计规范

### 1. 日志接收 API（永远保持简单）

```http
POST /api/v1/logs
Content-Type: application/json

{
  "message": "用户登录成功",
  "level": "info",
  "service": "auth-service",
  "timestamp": "2024-01-01T10:00:00Z",
  // 其他任意字段
  "user_id": 123,
  "ip": "192.168.1.1"
}
```

### 2. 查询 API（自然语言优先）

**方式1：自然语言查询（主推）**

```http
GET /api/v1/query?q=今天下午的错误日志&service=payment
```

**方式2：结构化查询（高级用户）**

```http
POST /api/v1/query
Content-Type: application/json

{
  "filters": [
    {"field": "level", "op": "=", "value": "error"},
    {"field": "time", "op": ">", "value": "2024-01-01"}
  ],
  "limit": 100
}
```

### 3. 实时流 API（用于调试）

```http
GET /api/v1/tail?service=auth&level=error
Connection: keep-alive
Transfer-Encoding: chunked
```

---

## 四、数据模型（固定格式）

```go
// 核心日志结构（不可变）
type LogEntry struct {
    // 必填字段
    ID        string    `json:"id"`         // UUID
    Timestamp time.Time `json:"timestamp"`  // 时间戳
    Message   string    `json:"message"`    // 日志消息
    Level     string    `json:"level"`      // info/warn/error/debug
    Service   string    `json:"service"`    // 服务名称
    
    // 推荐字段（SDK自动填充）
    TraceID   string `json:"trace_id,omitempty"`  // 追踪ID
    SpanID    string `json:"span_id,omitempty"`   // 跨度ID
    UserID    string `json:"user_id,omitempty"`   // 用户ID
    RequestID string `json:"request_id,omitempty"`// 请求ID
    IP        string `json:"ip,omitempty"`        // IP地址
    
    // 🆕 代码位置（SDK自动采集，解决「上下文不足」痛点）
    Caller     string `json:"caller,omitempty"`     // 调用位置 "main.go:42"
    Function   string `json:"function,omitempty"`   // 函数名 "main.HandleOrder"
    Package    string `json:"package,omitempty"`    // 包名 "github.com/xxx/service"
    StackTrace string `json:"stack_trace,omitempty"`// 错误堆栈（仅 error 级别）
    StackHash  string `json:"stack_hash,omitempty"` // 堆栈指纹（用于错误聚合）
    
    // 任意字段
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
    
    // 系统字段（内部使用）
    ReceivedAt time.Time `json:"-"`           // 接收时间
    StoredAt   time.Time `json:"-"`           // 存储时间
}
```

---

## 五、SDK 设计（Go 优先）

### 基础 API

```go
// 永远保持这个简单的 API
package loglite

// 初始化 - 一行代码
func Init(serviceName string) Logger

// 日志方法 - 像 fmt.Println 一样简单
func (l Logger) Info(message string, keyvals ...interface{})
func (l Logger) Error(message string, keyvals ...interface{})
func (l Logger) Warn(message string, keyvals ...interface{})
func (l Logger) Debug(message string, keyvals ...interface{})

// 链式调用
func (l Logger) With(fields map[string]interface{}) Logger

// 从 Context 获取（自动传递 trace_id、user_id 等）
func (l Logger) WithContext(ctx context.Context) Logger

// 调试模式（针对 IDE 调试痛点）
func (l Logger) EnableDebug()  // 在浏览器实时查看日志
```

### 自动上下文采集（解决「上下文不足」痛点）

```go
// 用户只需要这样写
logger.Error("支付失败", "error", err, "order_id", orderID)

// SDK 自动采集并附加以下信息：
// {
//   "message": "支付失败",
//   "level": "error",
//   "error": "insufficient balance",
//   "order_id": "ORD123456",
//   
//   // 🆕 以下字段自动采集
//   "caller": "payment/handler.go:156",
//   "function": "HandlePayment",
//   "package": "github.com/xxx/payment",
//   "stack_trace": "goroutine 42 [running]:\n...",  // 仅 error 级别
//   "stack_hash": "a1b2c3d4",  // 用于错误聚合
//   
//   // 从 Context 自动获取
//   "trace_id": "abc-123-xyz",
//   "request_id": "req-456",
//   "user_id": "user-789"
// }
```

### 配置自动采集

```go
logger := loglite.Init("my-service", 
    loglite.WithCaller(true),           // 采集调用位置（默认开启）
    loglite.WithStackTrace(loglite.ErrorLevel), // error 级别采集堆栈
    loglite.WithContextKeys("user_id", "tenant_id"), // 从 context 提取的 key
)

---

## 六、配置系统（约定优于配置）

```yaml
# config.yaml - 默认配置就能工作
server:
  port: 8080           # Web 端口
  data_dir: ./data     # 数据目录
  
storage:
  max_size: 10GB       # 最大存储
  retention: 7d        # 保留时间
  
query:
  natural_language: true  # 启用自然语言查询
  default_limit: 100      # 默认返回条数

# 🆕 告警配置（可选）
alerts:
  enabled: false
  channels:
    wecom:
      type: wecom
      webhook: ""
    feishu:
      type: feishu
      webhook: ""
  rules: []
  
# 可选功能（默认关闭）
kafka:
  enabled: false
  
redis:
  enabled: false
  
ui:
  enabled: true        # 内置 Web UI
```

---

## 七、开发路线图

### Phase 1: MVP（核心功能）- 2周

- [ ] HTTP 接收日志
- [ ] BadgerDB 存储
- [ ] 简单查询（按时间、级别、服务）
- [ ] 基础 Web 界面
- [ ] Go SDK 基础版（含自动上下文采集）

### Phase 2: 可用版本 - 3周

- [ ] 自然语言查询
- [ ] 全文索引
- [ ] 命令行工具
- [ ] 存储策略（TTL）
- [ ] 实时 tail 功能
- [ ] **🆕 错误聚合 API**（哪个函数报错多）
- [ ] **🆕 Gin/Fiber 中间件**

### Phase 3: 生产版本 - 4周

- [ ] 权限控制
- [ ] **🆕 告警系统**（企业微信/飞书/钉钉）
- [ ] **🆕 错误分析看板**
- [ ] 性能优化
- [ ] 数据导出
- [ ] 监控仪表板
- [ ] **🆕 GORM/sqlx Hook**
- [ ] **🆕 zerolog/zap 适配器**

### Phase 4: 企业版本 - 持续迭代

- [ ] 分布式支持
- [ ] 高可用
- [ ] 审计日志
- [ ] 多租户
- [ ] **🆕 多语言 SDK**（Python、Java、Node.js）
- [ ] **🆕 日志采样/限流**
- [ ] **🆕 日志脱敏**

---

## 八、性能目标（实事求是）

| 场景 | 目标 |
|------|------|
| 单机日志接收 | 5,000-10,000 logs/sec |
| 查询响应时间 | < 200ms（最近1小时数据） |
| 内存占用 | < 512MB（默认配置） |
| 启动时间 | < 2秒 |

---

## 九、项目结构参考

```
loglite/
├── cmd/
│   ├── server/          # 主服务
│   ├── cli/             # 命令行工具
│   └── bench/           # 性能测试
├── internal/
│   ├── core/            # 核心引擎
│   │   ├── storage/     # 存储实现
│   │   ├── query/       # 查询引擎
│   │   └── index/       # 索引引擎
│   ├── api/             # HTTP API
│   └── config/          # 配置管理
├── pkg/
│   ├── sdk/go/          # Go SDK
│   ├── protocol/        # 协议定义
│   └── utils/           # 工具函数
├── web/
│   ├── ui/              # 前端界面
│   └── templates/       # HTML模板
└── examples/            # 使用示例
```

---

## 十、开发者体验目标

### 从零到一

```bash
# 1. 下载
$ wget https://github.com/loglite/loglite/releases/latest/loglite

# 2. 运行
$ ./loglite

# 3. 发送日志
$ curl -X POST http://localhost:8080/logs \
  -d '{"message":"test", "level":"info", "service":"test"}'

# 4. 查询
$ curl "http://localhost:8080/query?q=最近的日志"
```

### 集成到 Go 项目

```go
import "github.com/loglite/sdk-go"

func main() {
    logger := loglite.Init("my-service")
    logger.Info("服务启动", "port", 8080)
}
```

---

## 十一、质量保证

### 测试策略

- **单元测试**：核心功能必须测试
- **集成测试**：API 和存储的集成
- **性能测试**：保证性能目标
- **端到端测试**：完整流程测试

### 监控

- **内置健康检查**：`/health`
- **内置性能指标**：`/metrics`
- **内置日志**：LogLite 自己的日志

---

## 十二、兼容性承诺

### 永远保持兼容

- **日志格式**：v1.0 的日志格式永不改变
- **核心 API**：`/api/v1/logs` 和 `/api/v1/query` 保持稳定
- **Go SDK**：主要 API 方法不破坏性变更

### 可以改变的

- 内部存储格式
- 索引算法
- 配置项（向后兼容）

---

## 十三、框架集成（即插即用）

> 目标：支持市面常用框架，一行代码接入

### Web 框架中间件

| 框架 | 包 | 功能 |
|------|-----|------|
| **Gin** | `sdk-go/middleware/gin` | 请求日志、Panic 恢复、TraceID 注入 |
| **Fiber** | `sdk-go/middleware/fiber` | 请求日志、性能追踪 |
| **Echo** | `sdk-go/middleware/echo` | 请求日志 |
| **Chi** | `sdk-go/middleware/chi` | 请求日志 |
| **Hertz** | `sdk-go/middleware/hertz` | 请求日志（字节跳动框架） |

**Gin 集成示例：**

```go
import (
    "github.com/gin-gonic/gin"
    loglitemw "github.com/loglite/sdk-go/middleware/gin"
)

func main() {
    r := gin.New()
    
    // 一行代码接入
    r.Use(loglitemw.Logger())   // 自动记录请求日志
    r.Use(loglitemw.Recovery()) // Panic 恢复并记录错误
    
    r.GET("/api/users", func(c *gin.Context) {
        // 从 context 获取 logger（已注入 trace_id、request_id）
        logger := loglite.FromContext(c.Request.Context())
        logger.Info("处理用户请求", "user_id", c.Query("id"))
    })
}
```

**自动记录的请求日志：**

```json
{
  "message": "HTTP Request",
  "level": "info",
  "method": "GET",
  "path": "/api/users",
  "status": 200,
  "latency_ms": 23,
  "client_ip": "192.168.1.1",
  "user_agent": "Mozilla/5.0...",
  "trace_id": "abc-123",
  "request_id": "req-456"
}
```

### ORM/数据库 Hook

| 框架 | 包 | 功能 |
|------|-----|------|
| **GORM** | `sdk-go/hooks/gorm` | 慢查询日志、错误日志、SQL 追踪 |
| **sqlx** | `sdk-go/hooks/sqlx` | SQL 日志 |
| **ent** | `sdk-go/hooks/ent` | ORM 操作日志 |

**GORM 集成示例：**

```go
import (
    "gorm.io/gorm"
    loglitegorm "github.com/loglite/sdk-go/hooks/gorm"
)

db, _ := gorm.Open(mysql.Open(dsn), &gorm.Config{
    Logger: loglitegorm.NewLogger(
        loglitegorm.WithSlowThreshold(200 * time.Millisecond), // 慢查询阈值
        loglitegorm.WithLogLevel(loglite.WarnLevel),           // 日志级别
    ),
})
```

**自动记录的慢查询日志：**

```json
{
  "message": "Slow SQL",
  "level": "warn",
  "sql": "SELECT * FROM users WHERE id = ?",
  "args": [123],
  "latency_ms": 356,
  "rows_affected": 1,
  "caller": "user/repo.go:42"
}
```

### 日志库适配器

| 库 | 包 | 说明 |
|-----|-----|------|
| **zerolog** | `sdk-go/adapters/zerolog` | 作为 zerolog 的 Writer |
| **zap** | `sdk-go/adapters/zap` | 作为 zap 的 Core |
| **logrus** | `sdk-go/adapters/logrus` | 作为 logrus 的 Hook |
| **slog** | `sdk-go/adapters/slog` | Go 1.21+ 标准库 |

**已有项目迁移示例（zerolog）：**

```go
import (
    "github.com/rs/zerolog"
    logliteadapter "github.com/loglite/sdk-go/adapters/zerolog"
)

// 原有代码不用改，只需要换一个 Writer
logger := zerolog.New(logliteadapter.NewWriter("my-service"))
logger.Info().Str("user", "john").Msg("用户登录")
// 日志自动发送到 LogLite
```

---

## 十四、告警系统（解决「报错发企业微信/飞书」痛点）

### 支持的通知渠道

| 渠道 | 状态 | 说明 |
|------|------|------|
| **企业微信** | ✅ 支持 | Webhook 机器人 |
| **飞书** | ✅ 支持 | Webhook 机器人 |
| **钉钉** | ✅ 支持 | Webhook 机器人 |
| **Slack** | ✅ 支持 | Webhook |
| **Telegram** | ✅ 支持 | Bot API |
| **邮件** | ✅ 支持 | SMTP |
| **Webhook** | ✅ 支持 | 自定义 HTTP 回调 |
| **PagerDuty** | 🔜 计划 | 事件管理平台 |

### 配置示例

```yaml
# config.yaml
alerts:
  enabled: true
  
  # 通知渠道配置
  channels:
    wecom:
      type: wecom
      webhook: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
      
    feishu:
      type: feishu
      webhook: "https://open.feishu.cn/open-apis/bot/v2/hook/xxx"
      
    feishu-oncall:
      type: feishu
      webhook: "https://open.feishu.cn/open-apis/bot/v2/hook/yyy"
      at_all: true  # @所有人
      
    dingtalk:
      type: dingtalk
      webhook: "https://oapi.dingtalk.com/robot/send?access_token=xxx"
      secret: "SECxxx"  # 签名密钥
      
    email:
      type: email
      smtp_host: "smtp.example.com"
      smtp_port: 587
      username: "alerts@example.com"
      password: "xxx"
      to: ["oncall@example.com", "dev@example.com"]
      
    custom:
      type: webhook
      url: "https://your-system.com/alert"
      method: POST
      headers:
        Authorization: "Bearer xxx"
  
  # 告警规则
  rules:
    # 规则1：错误数量激增
    - name: "错误激增告警"
      description: "5分钟内错误超过100条"
      condition: "count(level=error) > 100 in 5m"
      severity: critical
      channels: [wecom, feishu-oncall]
      cooldown: 10m  # 冷却时间，避免重复告警
      
    # 规则2：特定函数报错（解决「哪个函数报错」痛点）
    - name: "支付函数报错"
      description: "HandlePayment 函数 10 分钟内报错超过 5 次"
      condition: "count(function='HandlePayment', level=error) > 5 in 10m"
      severity: high
      channels: [wecom, feishu]
      
    # 规则3：特定服务不可用
    - name: "支付服务异常"
      description: "支付服务错误率过高"
      condition: "rate(level=error, service=payment) > 10% in 5m"
      severity: critical
      channels: [wecom, feishu-oncall, email]
      
    # 规则4：慢查询告警
    - name: "数据库慢查询"
      description: "慢查询数量过多"
      condition: "count(message='Slow SQL') > 50 in 5m"
      severity: medium
      channels: [feishu]
```

### 告警消息模板

**企业微信告警示例：**

```
🚨 LogLite 告警

【告警名称】支付函数报错
【告警级别】🔴 Critical
【触发条件】HandlePayment 函数 10 分钟内报错 8 次
【告警时间】2024-01-15 14:32:00

【最近错误】
- [14:31:45] insufficient balance (order_id: ORD123)
- [14:30:22] timeout connecting to payment gateway
- [14:28:15] invalid card number

【查看详情】https://loglite.example.com/errors?function=HandlePayment
```

### 告警 API

```http
# 手动触发告警
POST /api/v1/alerts/send
{
  "channel": "wecom",
  "title": "自定义告警",
  "content": "这是一条测试告警",
  "severity": "high"
}

# 查询告警历史
GET /api/v1/alerts/history?start=2024-01-01&end=2024-01-15

# 静默告警（维护期间）
POST /api/v1/alerts/silence
{
  "rules": ["错误激增告警"],
  "duration": "2h",
  "reason": "系统升级维护"
}
```

---

## 十五、错误分析（解决「哪个函数报错多」痛点）

### 错误聚合 API

```http
# 按函数聚合错误（Top 10 报错函数）
GET /api/v1/stats/errors?
    group_by=function&
    time_range=1h&
    top=10&
    service=payment
```

**返回示例：**

```json
{
  "code": 200,
  "data": {
    "time_range": "2024-01-15T13:00:00Z ~ 2024-01-15T14:00:00Z",
    "total_errors": 523,
    "groups": [
      {
        "function": "payment.HandlePayment",
        "package": "github.com/xxx/payment",
        "count": 156,
        "percentage": "29.8%",
        "last_seen": "2024-01-15T13:58:32Z",
        "sample_error": "insufficient balance",
        "trend": "+23%"  // 相比上一小时
      },
      {
        "function": "user.ValidateToken",
        "package": "github.com/xxx/auth",
        "count": 89,
        "percentage": "17.0%",
        "last_seen": "2024-01-15T13:57:12Z",
        "sample_error": "token expired",
        "trend": "-5%"
      }
    ]
  }
}
```

### 错误堆栈聚合

```http
# 相似错误自动归类（根据 stack_hash）
GET /api/v1/stats/errors/stacks?
    time_range=24h&
    top=20
```

**返回示例：**

```json
{
  "data": [
    {
      "stack_hash": "a1b2c3d4",
      "count": 234,
      "first_seen": "2024-01-14T10:00:00Z",
      "last_seen": "2024-01-15T13:58:00Z",
      "sample": {
        "message": "connection refused",
        "function": "redis.Get",
        "stack_trace": "goroutine 42 [running]:\nredis.(*Client).Get(...)"
      },
      "affected_services": ["user-service", "order-service"]
    }
  ]
}
```

### 错误趋势 API

```http
# 错误趋势（按小时）
GET /api/v1/stats/errors/trend?
    time_range=24h&
    interval=1h&
    service=payment
```

### Web UI 错误看板

LogLite 内置错误分析看板，包含：

- 📊 **错误趋势图**：按时间查看错误数量变化
- 🏆 **Top 10 报错函数**：快速定位问题代码
- 🔗 **错误堆栈聚合**：相似错误自动合并，避免重复排查
- 🔍 **错误详情**：点击查看完整上下文和堆栈
- 📈 **错误率监控**：错误率 = 错误数 / 总日志数

---

## 十六、日志最佳实践（解决「废话日志」痛点）

### ❌ 反模式：废话日志

```go
// 这些日志没有价值，不要这样写！
func HandleOrder(ctx context.Context, order *Order) error {
    logger.Info("进入函数")           // ❌ 废话
    logger.Info("开始处理")           // ❌ 废话
    logger.Debug("order: %+v", order) // ❌ 敏感信息泄露风险
    
    // ... 业务逻辑
    
    logger.Info("处理完成")           // ❌ 废话
    return nil
}
```

### ✅ 正确示范：有价值的日志

```go
func HandleOrder(ctx context.Context, order *Order) error {
    logger := loglite.FromContext(ctx)
    start := time.Now()
    
    // 验证
    if err := order.Validate(); err != nil {
        logger.Warn("订单验证失败",
            "order_id", order.ID,
            "error", err,
            "user_id", order.UserID,
        )
        return err
    }
    
    // 处理支付
    if err := processPayment(ctx, order); err != nil {
        logger.Error("支付处理失败",
            "order_id", order.ID,
            "amount", order.Amount,
            "payment_method", order.PaymentMethod,
            "error", err,
        )
        return err
    }
    
    // 成功 - 记录关键业务指标
    logger.Info("订单处理成功",
        "order_id", order.ID,
        "user_id", order.UserID,
        "amount", order.Amount,
        "latency_ms", time.Since(start).Milliseconds(),
    )
    
    return nil
}
```

### 日志级别规范

| 级别 | 使用场景 | 示例 |
|------|----------|------|
| **Debug** | 开发调试，生产环境关闭 | 变量值、SQL 语句 |
| **Info** | 关键业务事件 | 订单创建、用户登录、支付成功 |
| **Warn** | 可恢复的异常 | 重试成功、降级处理、参数校验失败 |
| **Error** | 需要关注的错误 | 支付失败、数据库错误、外部服务超时 |

### 日志内容规范

```go
// ✅ 好的日志应该包含：
logger.Error("支付失败",
    // 1. 唯一标识（方便搜索）
    "order_id", order.ID,
    "user_id", user.ID,
    "trace_id", ctx.Value("trace_id"),
    
    // 2. 业务上下文（理解发生了什么）
    "amount", order.Amount,
    "payment_method", "alipay",
    "retry_count", 3,
    
    // 3. 错误信息（定位问题）
    "error", err,
    
    // 4. 性能数据（可选）
    "latency_ms", elapsed,
)
```

### SDK 内置的日志规范检查

```go
logger := loglite.Init("my-service",
    loglite.WithLintRules(
        loglite.NoEmptyMessage(),      // 禁止空消息
        loglite.RequireFields("order_id", "user_id"), // 必须包含字段
        loglite.MaxFieldCount(20),     // 字段数量限制
    ),
)

// 违反规则会打印警告
logger.Info("")  // ⚠️ Warning: empty log message
```

---

## 附录

### 相关资源

- GitHub: [loglite/loglite](https://github.com/loglite/loglite)
- 文档: [docs.loglite.io](https://docs.loglite.io)
- 社区: [Discord](https://discord.gg/loglite)

### 贡献指南

欢迎贡献代码、文档和想法！请查看 [CONTRIBUTING.md](../CONTRIBUTING.md)

---

**最后更新**: 2024-01-01  
**版本**: v1.0
