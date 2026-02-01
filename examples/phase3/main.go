package main

import (
	"context"
	"fmt"
	"time"

	"github.com/loglite/loglite/internal/alert"
	"github.com/loglite/loglite/internal/auth"
	"github.com/loglite/loglite/internal/export"
	"github.com/loglite/loglite/internal/monitor"
	"github.com/loglite/loglite/pkg/sdk"
	loglitegorm "github.com/loglite/loglite/pkg/sdk/hooks/gorm"
)

// ============================================================
// Phase 3 功能框架使用示例
// ============================================================

func main() {
	fmt.Println("=== LogLite Phase 3 功能演示 ===")
	fmt.Println()

	// 1. 监控系统
	demoMonitor()

	// 2. 告警系统
	demoAlert()

	// 3. 权限控制
	demoAuth()

	// 4. 数据导出
	demoExport()

	// 5. GORM Hook
	demoGORMHook()
}

// ============================================================
// 1. 监控系统演示
// ============================================================

func demoMonitor() {
	fmt.Println("【1】监控系统")

	mon := monitor.NewMonitor()

	mon.IncrLogReceived(100)
	mon.IncrLogStored(98)
	mon.RecordIngestLatency(5 * time.Millisecond)
	mon.RecordQueryLatency(50 * time.Millisecond)

	metrics := mon.GetMetrics()
	fmt.Printf("  接收日志: %d\n", metrics.LogsReceived)
	fmt.Printf("  存储日志: %d\n", metrics.LogsStored)
	fmt.Printf("  平均接收延迟: %v\n", metrics.AvgIngestLatency)
	fmt.Printf("  平均查询延迟: %v\n", metrics.AvgQueryLatency)

	checker := monitor.NewHealthChecker()
	health := checker.Check()
	fmt.Printf("  健康状态: %s\n", health.Status)
	fmt.Printf("  运行时间: %v\n", health.Uptime)

	fmt.Println()
}

// ============================================================
// 2. 告警系统演示
// ============================================================

func demoAlert() {
	fmt.Println("【2】告警系统")

	ctx := context.Background()

	alertMgr := alert.NewAlertManager(&alert.Config{
		Enabled: true,
	})

	// 注册告警渠道
	alertMgr.RegisterChannel("wecom", alert.NewWeComChannel("https://qyapi.weixin.qq.com/..."))
	alertMgr.RegisterChannel("feishu", alert.NewFeishuChannel("https://open.feishu.cn/...", true))
	alertMgr.RegisterChannel("dingtalk", alert.NewDingTalkChannel("https://oapi.dingtalk.com/...", "SEC..."))

	fmt.Println("  ✓ 注册企业微信、飞书、钉钉渠道")

	// 添加告警规则
	rule := &alert.Rule{
		Name:        "错误激增告警",
		Description: "5分钟内错误超过100条",
		Condition:   "count(level=error) > 100 in 5m",
		Severity:    alert.SeverityCritical,
		Channels:    []string{"wecom", "feishu"},
		Cooldown:    10 * time.Minute,
		Enabled:     true,
	}
	alertMgr.AddRule(rule)
	fmt.Printf("  ✓ 添加规则: %s\n", rule.Name)

	// 模拟触发告警
	alertEvent := &alert.Alert{
		RuleName:     "错误激增告警",
		Severity:     alert.SeverityCritical,
		Title:        "LogLite 告警",
		Message:      "payment-service 5分钟内错误超过100条",
		Timestamp:    time.Now(),
		DashboardURL: "http://localhost:8080/errors?service=payment-service",
	}

	_ = alertMgr.Trigger(ctx, rule, alertEvent)
	fmt.Println("  ✓ 触发告警（演示）")

	fmt.Println()
}

// ============================================================
// 3. 权限控制演示
// ============================================================

func demoAuth() {
	fmt.Println("【3】权限控制")

	ctx := context.Background()
	authMgr := auth.NewAuthManager()

	adminUser := &auth.User{
		ID:       "user-001",
		Username: "admin",
		Roles:    []string{"admin"},
		TenantID: "tenant-001",
	}

	viewerUser := &auth.User{
		ID:       "user-002",
		Username: "viewer",
		Roles:    []string{"viewer"},
		TenantID: "tenant-001",
	}

	fmt.Println("  Admin 用户:")
	if err := authMgr.Authorize(ctx, adminUser, auth.PermissionViewLogs); err == nil {
		fmt.Println("    ✓ 有查看日志权限")
	}
	if err := authMgr.Authorize(ctx, adminUser, auth.PermissionManageUsers); err == nil {
		fmt.Println("    ✓ 有管理用户权限")
	}

	fmt.Println("  Viewer 用户:")
	if err := authMgr.Authorize(ctx, viewerUser, auth.PermissionViewLogs); err == nil {
		fmt.Println("    ✓ 有查看日志权限")
	}
	if err := authMgr.Authorize(ctx, viewerUser, auth.PermissionManageUsers); err != nil {
		fmt.Println("    ✗ 无管理用户权限")
	}

	fmt.Println()
}

// ============================================================
// 4. 数据导出演示
// ============================================================

func demoExport() {
	fmt.Println("【4】数据导出")

	exportMgr := export.NewExportManager()

	exportMgr.RegisterExporter(export.FormatJSON, &export.JSONExporter{})
	exportMgr.RegisterExporter(export.FormatCSV, &export.CSVExporter{})
	exportMgr.RegisterExporter(export.FormatText, &export.TextExporter{})
	exportMgr.RegisterExporter(export.FormatExcel, &export.ExcelExporter{})

	fmt.Println("  ✓ 注册 JSON/CSV/Text/Excel 导出器")

	req := &export.ExportRequest{
		Service:   "payment-service",
		Level:     "error",
		Keyword:   "timeout",
		StartTime: time.Now().Add(-24 * time.Hour),
		EndTime:   time.Now(),
		Format:    export.FormatCSV,
		Limit:     10000,
	}

	fmt.Printf("  导出请求: %s 格式, service=%s, level=%s\n",
		req.Format, req.Service, req.Level)

	fmt.Println()
}

// ============================================================
// 5. GORM Hook 演示
// ============================================================

func demoGORMHook() {
	fmt.Println("【5】GORM Hook")

	// 使用新的 OpenTelemetry 风格 API
	provider := sdk.NewLoggerProvider(
		sdk.WithEndpoint("http://localhost:8081"),
	)
	defer provider.Shutdown()

	logger := provider.Logger("my-service")

	// 创建 GORM Logger
	gormLogger := loglitegorm.NewLogger(logger,
		loglitegorm.WithSlowThreshold(200*time.Millisecond),
	)

	fmt.Println("  ✓ 创建 GORM Logger")
	fmt.Println("  ✓ 慢查询阈值: 200ms")

	// 使用示例：
	// db, _ := gorm.Open(mysql.Open(dsn), &gorm.Config{
	//     Logger: gormLogger,
	// })

	_ = gormLogger

	fmt.Println()
}
