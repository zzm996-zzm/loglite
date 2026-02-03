package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"
)

// Option 配置选项
type Option func(*options)

// options 内部选项
type options struct {
	config Config
	ctx    context.Context
}

// buildOptions 构建选项
func buildOptions(opts ...Option) *options {
	o := &options{
		config: defaultConfig,
		ctx:    context.Background(),
	}

	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithType 设置存储类型
func WithType(typ string) Option {
	return func(o *options) {
		o.config["type"] = typ
	}
}

// WithPath 设置存储路径
func WithPath(path string) Option {
	return func(o *options) {
		o.config["path"] = path
	}
}

// WithTTL 设置TTL
func WithTTL(hot, warm, cold time.Duration) Option {
	return func(o *options) {
		o.config[ConfigKeyHotTTL] = hot
		o.config[ConfigKeyWarmTTL] = warm
		o.config[ConfigKeyColdTTL] = cold
	}
}

// WithConfig 设置完整配置
func WithConfig(cfg Config) Option {
	return func(o *options) {
		// 合并配置
		for k, v := range cfg {
			o.config[k] = v
		}
	}
}

// WithContext 设置上下文
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		o.ctx = ctx
	}
}

// New 创建存储实例（主入口）
func New(opts ...Option) (Store, error) {
	o := buildOptions(opts...)

	typ := o.config.GetString("type", TypeTiered)

	// 获取创建器
	creator, exists := GetCreator(typ)
	if !exists {
		return nil, fmt.Errorf("storage engine not found: %s", typ)
	}

	// 验证配置
	if err := creator.Validate(o.config); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	// 创建存储实例
	return creator.Create(o.ctx, o.config)
}

// NewTiered 创建分级存储（快捷方式）
func NewTiered(path string, opts ...Option) (Store, error) {
	opts = append([]Option{WithType(TypeTiered)}, opts...)
	if path != "" {
		opts = append(opts, WithPath(path))
	}
	return New(opts...)
}

// NewBadger 创建Badger存储（快捷方式）
func NewBadger(path string, opts ...Option) (Store, error) {
	opts = append([]Option{WithType(TypeBadger)}, opts...)
	if path != "" {
		opts = append(opts, WithPath(path))
	}
	return New(opts...)
}

// NewSQLite 创建SQLite存储（快捷方式）
func NewSQLite(path string, opts ...Option) (Store, error) {
	opts = append([]Option{WithType(TypeSQLite)}, opts...)
	if path != "" {
		opts = append(opts, WithPath(path))
	}
	return New(opts...)
}

// NewMemory 创建内存存储（快捷方式，用于测试）
func NewMemory(opts ...Option) (Store, error) {
	opts = append([]Option{WithType(TypeMemory)}, opts...)
	return New(opts...)
}

// NewFromFile 从配置文件创建
func NewFromFile(path string, opts ...Option) (Store, error) {
	// 读取配置文件
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// 解析配置
	fileCfg, err := FromJSON(data)
	if err != nil {
		// 尝试YAML
		fileCfg, err = loadYAML(data)
		if err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	// 合并配置
	allOpts := append([]Option{WithConfig(fileCfg)}, opts...)

	// 如果配置文件有路径，但配置中没有指定，使用配置文件所在目录
	if fileCfg.GetString("path") == "" {
		configDir := filepath.Dir(path)
		allOpts = append(allOpts, WithPath(configDir))
	}

	return New(allOpts...)
}

// NewFromConfig 从配置结构创建
func NewFromConfig(cfg Config, opts ...Option) (Store, error) {
	allOpts := append([]Option{WithConfig(cfg)}, opts...)
	return New(allOpts...)
}

// loadYAML 加载YAML配置（简化实现）
func loadYAML(data []byte) (Config, error) {
	// 实际应该使用yaml.v3
	// 这里简化处理
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return Config(cfg), nil
}
