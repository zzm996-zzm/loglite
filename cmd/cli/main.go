package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bytedance/sonic"
)

var (
	version  = "0.1.0"
	endpoint = "http://localhost:8081"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	// 检查环境变量
	if e := os.Getenv("LOGLITE_ENDPOINT"); e != "" {
		endpoint = e
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "query", "q":
		handleQuery(args)
	case "tail", "t":
		handleTail(args)
	case "stats", "s":
		handleStats(args)
	case "send":
		handleSend(args)
	case "clear":
		handleClear(args)
	case "version", "-v", "--version":
		fmt.Printf("loglite-cli %s\n", version)
	case "help", "-h", "--help":
		printHelp()
	default:
		// 默认当作自然语言查询
		handleQuery(os.Args[1:])
	}
}

func printHelp() {
	fmt.Println(`LogLite CLI - 轻量级日志平台命令行工具

用法:
  loglite-cli <命令> [参数]

命令:
  query, q    查询日志
  tail, t     实时查看日志
  stats, s    查看统计信息
  send        发送日志
  clear       清空数据库（危险操作！）
  version     显示版本
  help        显示帮助

查询示例:
  loglite-cli query "今天的错误日志"
  loglite-cli q --service=payment --level=error
  loglite-cli q --start="2024-01-01" --end="2024-01-02"

实时日志:
  loglite-cli tail
  loglite-cli t --service=auth --level=error

统计信息:
  loglite-cli stats errors
  loglite-cli s errors --group-by=function --top=20

发送日志:
  loglite-cli send --service=test --level=info --message="测试日志"

环境变量:
  LOGLITE_ENDPOINT  服务端地址 (默认: http://localhost:8081)`)
}

func handleQuery(args []string) {
	params := url.Values{}

	// 解析参数
	var query string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--service=") {
			params.Set("service", strings.TrimPrefix(arg, "--service="))
		} else if strings.HasPrefix(arg, "--level=") {
			params.Set("level", strings.TrimPrefix(arg, "--level="))
		} else if strings.HasPrefix(arg, "--start=") {
			params.Set("start", strings.TrimPrefix(arg, "--start="))
		} else if strings.HasPrefix(arg, "--end=") {
			params.Set("end", strings.TrimPrefix(arg, "--end="))
		} else if strings.HasPrefix(arg, "--limit=") {
			params.Set("limit", strings.TrimPrefix(arg, "--limit="))
		} else if strings.HasPrefix(arg, "-n") {
			if i+1 < len(args) {
				params.Set("limit", args[i+1])
				i++
			}
		} else if !strings.HasPrefix(arg, "-") {
			query += arg + " "
		}
	}

	if query != "" {
		params.Set("q", strings.TrimSpace(query))
	}

	// 发起请求
	url := endpoint + "/api/v1/query?" + params.Encode()
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "请求失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&result)

	if code, ok := result["code"].(float64); ok && code != 200 {
		fmt.Fprintf(os.Stderr, "查询失败: %v\n", result["message"])
		os.Exit(1)
	}

	data := result["data"].(map[string]interface{})
	logs := data["logs"].([]interface{})
	total := int(data["total"].(float64))

	fmt.Printf("找到 %d 条日志\n\n", total)

	for _, l := range logs {
		log := l.(map[string]interface{})
		printLog(log)
	}
}

func handleTail(args []string) {
	params := url.Values{}

	for _, arg := range args {
		if strings.HasPrefix(arg, "--service=") {
			params.Set("service", strings.TrimPrefix(arg, "--service="))
		} else if strings.HasPrefix(arg, "--level=") {
			params.Set("level", strings.TrimPrefix(arg, "--level="))
		}
	}

	url := endpoint + "/api/v1/tail?" + params.Encode()

	fmt.Println("连接到日志流...")
	fmt.Printf("服务: %s, 级别: %s\n", params.Get("service"), params.Get("level"))
	fmt.Println("按 Ctrl+C 退出\n")

	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "连接失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("连接断开")
				break
			}
			continue
		}

		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)

			var log map[string]interface{}
			if err := sonic.Unmarshal([]byte(data), &log); err == nil {
				printLog(log)
			}
		}
	}
}

func handleStats(args []string) {
	if len(args) == 0 {
		fmt.Println("请指定统计类型: errors")
		return
	}

	subCmd := args[0]
	params := url.Values{}

	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "--group-by=") {
			params.Set("group_by", strings.TrimPrefix(arg, "--group-by="))
		} else if strings.HasPrefix(arg, "--top=") {
			params.Set("top", strings.TrimPrefix(arg, "--top="))
		} else if strings.HasPrefix(arg, "--time-range=") {
			params.Set("time_range", strings.TrimPrefix(arg, "--time-range="))
		} else if strings.HasPrefix(arg, "--service=") {
			params.Set("service", strings.TrimPrefix(arg, "--service="))
		}
	}

	switch subCmd {
	case "errors":
		url := endpoint + "/api/v1/stats/errors?" + params.Encode()
		resp, err := http.Get(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "请求失败: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&result)

		data := result["data"].(map[string]interface{})
		groups := data["groups"].([]interface{})

		fmt.Printf("时间范围: %s\n", data["time_range"])
		fmt.Printf("总错误数: %.0f\n", data["total_errors"])
		fmt.Printf("分组方式: %s\n\n", data["group_by"])

		fmt.Printf("%-40s %10s %10s\n", "名称", "数量", "占比")
		fmt.Println(strings.Repeat("-", 62))

		for _, g := range groups {
			group := g.(map[string]interface{})
			name := group["key"].(string)
			if len(name) > 38 {
				name = name[:38] + ".."
			}
			fmt.Printf("%-40s %10.0f %10s\n",
				name,
				group["count"].(float64),
				group["percentage"].(string),
			)
		}
	default:
		fmt.Printf("未知的统计类型: %s\n", subCmd)
	}
}

func handleSend(args []string) {
	var service, level, message string

	for _, arg := range args {
		if strings.HasPrefix(arg, "--service=") {
			service = strings.TrimPrefix(arg, "--service=")
		} else if strings.HasPrefix(arg, "--level=") {
			level = strings.TrimPrefix(arg, "--level=")
		} else if strings.HasPrefix(arg, "--message=") {
			message = strings.TrimPrefix(arg, "--message=")
		} else if strings.HasPrefix(arg, "-m") {
			// 下一个参数是消息
		} else if !strings.HasPrefix(arg, "-") && message == "" {
			message = arg
		}
	}

	if service == "" {
		service = "cli"
	}
	if level == "" {
		level = "info"
	}
	if message == "" {
		fmt.Println("请指定消息内容: --message=\"你的日志\"")
		return
	}

	body := fmt.Sprintf(`{"service":"%s","level":"%s","message":"%s"}`, service, level, message)
	resp, err := http.Post(endpoint+"/api/v1/logs", "application/json", strings.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "发送失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&result)

	if code, ok := result["code"].(float64); ok && code == 200 {
		data := result["data"].(map[string]interface{})
		fmt.Printf("日志已发送，ID: %s\n", data["id"])
	} else {
		fmt.Fprintf(os.Stderr, "发送失败: %v\n", result["message"])
	}
}

func handleClear(args []string) {
	// 检查是否需要确认
	force := false
	for _, arg := range args {
		if arg == "--force" || arg == "-f" {
			force = true
			break
		}
	}
	
	if !force {
		fmt.Println("⚠️  警告：此操作将清空所有日志数据，不可恢复！")
		fmt.Print("请输入 'yes' 确认: ")
		
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "yes" {
			fmt.Println("已取消")
			return
		}
	}
	
	// 读取配置
	configPath := "config.yaml"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") && args[0] != "--force" && args[0] != "-f" {
		configPath = args[0]
	}
	
	// 加载配置获取数据目录
	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}
	
	dataDir := cfg.Storage.DataDir
	if dataDir == "" {
		dataDir = "./data"
	}
	
	fmt.Printf("正在清空数据库: %s\n", dataDir)
	
	// 检查目录是否存在
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		fmt.Printf("数据目录不存在: %s\n", dataDir)
		return
	}
	
	// 删除数据目录
	if err := os.RemoveAll(dataDir); err != nil {
		fmt.Fprintf(os.Stderr, "清空失败: %v\n", err)
		os.Exit(1)
	}
	
	// 重新创建目录
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建目录失败: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("✅ 数据库已清空")
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var cfg Config
	if err := sonic.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	
	return &cfg, nil
}

type Config struct {
	Storage struct {
		DataDir string `yaml:"data_dir" json:"data_dir"`
	} `yaml:"storage" json:"storage"`
}

func printLog(log map[string]interface{}) {
	// 颜色代码
	colors := map[string]string{
		"error": "\033[31m", // 红
		"warn":  "\033[33m", // 黄
		"info":  "\033[32m", // 绿
		"debug": "\033[35m", // 紫
	}
	reset := "\033[0m"

	level := ""
	if l, ok := log["level"].(string); ok {
		level = l
	}

	timestamp := ""
	if t, ok := log["timestamp"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			timestamp = parsed.Format("01-02 15:04:05")
		} else {
			timestamp = t
		}
	}

	service := ""
	if s, ok := log["service"].(string); ok {
		service = s
	}

	message := ""
	if m, ok := log["message"].(string); ok {
		message = m
	}

	color := colors[level]
	if color == "" {
		color = ""
	}

	fmt.Printf("%s%s%-5s%s [%s] %s: %s\n",
		color,
		timestamp,
		strings.ToUpper(level),
		reset,
		service,
		message,
		"",
	)
}
