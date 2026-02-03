package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/loglite/loglite/internal/api"
	"github.com/loglite/loglite/internal/config"
	"github.com/loglite/loglite/internal/storage"
)

var (
	version   = "0.1.0"
	buildTime = "unknown"
)

func main() {
	// 命令行参数
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	showVersion := flag.Bool("version", false, "显示版本信息")
	flag.Parse()

	if *showVersion {
		fmt.Printf("LogLite %s (built %s)\n", version, buildTime)
		return
	}

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 确保数据目录存在
	if err := os.MkdirAll(cfg.Storage.DataDir, 0755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	// 初始化存储（使用新的重构后的 API）
	store, err := storage.New(
		storage.WithContext(context.Background()),
		storage.WithType("badger"),
		storage.WithPath(cfg.Storage.DataDir),
	)
	if err != nil {
		log.Fatalf("初始化存储失败: %v", err)
	}
	defer store.Close()

	// 设置路由
	router := api.SetupRouter(store)

	// 启动服务器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("正在关闭服务...")
		store.Close()
		os.Exit(0)
	}()

	log.Printf(`
╦  ╔═╗╔═╗╦  ╦╔╦╗╔═╗
║  ║ ║║ ╦║  ║ ║ ║╣ 
╩═╝╚═╝╚═╝╩═╝╩ ╩ ╚═╝  v%s

服务启动: http://%s
Web UI:   http://%s
API:      http://%s/api/v1

按 Ctrl+C 停止服务
`, version, addr, addr, addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
}
