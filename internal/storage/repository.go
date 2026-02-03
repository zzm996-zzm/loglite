// storage/registry.go
package storage

import (
	"fmt"
	"sync"
)

// 存储类型常量
const (
	TypeBadger = "badger"
	TypeSQLite = "sqlite"
	TypeMemory = "memory"
	TypeTiered = "tiered" // 分级存储（默认）
	TypeCustom = "custom" // 自定义存储
)

// 全局注册表
var (
	creatorsMu sync.RWMutex
	creators   = make(map[string]Creator)

	// 默认配置
	defaultConfig = Config{
		"type": TypeBadger,
		"path": "./data/loglite",
		"ttl":  "168h",
	}
)

// Register 注册存储引擎
func Register(name string, creator Creator) error {
	creatorsMu.Lock()
	defer creatorsMu.Unlock()

	if _, exists := creators[name]; exists {
		return fmt.Errorf("storage engine already registered: %s", name)
	}

	creators[name] = creator
	return nil
}

// Unregister 取消注册
func Unregister(name string) {
	creatorsMu.Lock()
	defer creatorsMu.Unlock()
	delete(creators, name)
}

// GetCreator 获取创建器
func GetCreator(name string) (Creator, bool) {
	creatorsMu.RLock()
	defer creatorsMu.RUnlock()

	creator, exists := creators[name]
	return creator, exists
}

// ListCreators 列出所有注册的引擎
func ListCreators() []string {
	creatorsMu.RLock()
	defer creatorsMu.RUnlock()

	names := make([]string, 0, len(creators))
	for name := range creators {
		names = append(names, name)
	}
	return names
}

// init 初始化，注册内置引擎
func init() {
	// 注册内置引擎

	// 注册 Badger 引擎（单层存储）
	Register(TypeBadger, &badgerCreator{})

	// 注册分级存储引擎（默认推荐）
	Register(TypeTiered, &tieredCreator{})

	// default 指向分级存储
	Register("default", &tieredCreator{})
}
