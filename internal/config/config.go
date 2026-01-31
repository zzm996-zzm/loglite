package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
	Query   QueryConfig   `yaml:"query"`
	Log     LogConfig     `yaml:"log"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	DataDir   string        `yaml:"data_dir"`
	MaxSize   string        `yaml:"max_size"`
	Retention time.Duration `yaml:"retention"`
}

// QueryConfig 查询配置
type QueryConfig struct {
	DefaultLimit int `yaml:"default_limit"`
	MaxLimit     int `yaml:"max_limit"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level string `yaml:"level"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8080,
			Host: "0.0.0.0",
		},
		Storage: StorageConfig{
			DataDir:   "./data",
			MaxSize:   "10GB",
			Retention: 168 * time.Hour, // 7 days
		},
		Query: QueryConfig{
			DefaultLimit: 100,
			MaxLimit:     1000,
		},
		Log: LogConfig{
			Level: "info",
		},
	}
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // 使用默认配置
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
