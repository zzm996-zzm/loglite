package storage

import (
	"context"
	"fmt"
)

// tieredCreator 实现 Creator 接口
type tieredCreator struct{}

// Create 创建分级存储实例
func (c *tieredCreator) Create(ctx context.Context, cfg Config) (Store, error) {
	return NewTieredStore(cfg)
}

// Validate 验证配置
func (c *tieredCreator) Validate(cfg Config) error {
	path := cfg.GetString(ConfigKeyPath)
	if path == "" {
		return fmt.Errorf("tiered storage requires '%s' config", ConfigKeyPath)
	}

	// 验证时间阈值的合理性
	hotThreshold := cfg.GetDuration(ConfigKeyHotThreshold, DefaultHotThreshold)
	warmThreshold := cfg.GetDuration(ConfigKeyWarmThreshold, DefaultWarmThreshold)

	if hotThreshold >= warmThreshold {
		return fmt.Errorf("%s (%v) must be less than %s (%v)",
			ConfigKeyHotThreshold, hotThreshold, ConfigKeyWarmThreshold, warmThreshold)
	}

	return nil
}

// DefaultConfig 返回默认配置
func (c *tieredCreator) DefaultConfig() Config {
	return Config{
		"type":        TypeTiered,
		ConfigKeyPath: "./data/tiered",

		// 时间阈值
		ConfigKeyHotThreshold:  "2h",  // Hot → Warm 边界: 2小时
		ConfigKeyWarmThreshold: "24h", // Warm → Cold 边界: 24小时

		// 各层 TTL
		ConfigKeyHotTTL:  "4h",   // Hot 层数据保留 4 小时
		ConfigKeyWarmTTL: "48h",  // Warm 层数据保留 48 小时
		ConfigKeyColdTTL: "168h", // Cold 层数据保留 7 天

		// 迁移器配置
		ConfigKeyMoverInterval:  "5m", // 迁移检查间隔
		ConfigKeyMoverBatchSize: 1000, // 每批迁移数量
	}
}
