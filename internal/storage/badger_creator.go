// storage/badger_creator.go
package storage

import (
	"context"
	"fmt"
)

// badgerCreator 实现Creator接口
type badgerCreator struct{}

func (c *badgerCreator) Create(ctx context.Context, cfg Config) (Store, error) {
	return NewBadgerStore(cfg)
}

func (c *badgerCreator) Validate(cfg Config) error {
	path := cfg.GetString("path")
	if path == "" {
		return fmt.Errorf("badger storage requires 'path' config")
	}
	return nil
}

func (c *badgerCreator) DefaultConfig() Config {
	return Config{
		"type":     "badger",
		"path":     "./data/badger",
		"ttl":      "168h", // 7天
		"cache_mb": 64,
	}
}
