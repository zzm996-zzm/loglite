// storage/config.go
package storage

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Config 配置结构（支持任意字段）
type Config map[string]interface{}

// Get 获取原始值
func (c Config) Get(key string) interface{} {
	return c[key]
}

// GetString 获取字符串（带默认值）
func (c Config) GetString(key string, def ...string) string {
	if val := c[key]; val != nil {
		switch v := val.(type) {
		case string:
			return v
		case []byte:
			return string(v)
		case fmt.Stringer:
			return v.String()
		default:
			return fmt.Sprintf("%v", v)
		}
	}
	if len(def) > 0 {
		return def[0]
	}
	return ""
}

// GetInt 获取整数
func (c Config) GetInt(key string, def ...int) int {
	if val := c[key]; val != nil {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case json.Number:
			if i, err := v.Int64(); err == nil {
				return int(i)
			}
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	if len(def) > 0 {
		return def[0]
	}
	return 0
}

// GetDuration 获取时间间隔
func (c Config) GetDuration(key string, def ...time.Duration) time.Duration {
	if val := c[key]; val != nil {
		switch v := val.(type) {
		case time.Duration:
			return v
		case string:
			if d, err := time.ParseDuration(v); err == nil {
				return d
			}
		case int:
			return time.Duration(v) * time.Second
		case int64:
			return time.Duration(v) * time.Second
		case float64:
			return time.Duration(v) * time.Second
		}
	}
	if len(def) > 0 {
		return def[0]
	}
	return 0
}

// GetBool 获取布尔值
func (c Config) GetBool(key string, def ...bool) bool {
	if val := c[key]; val != nil {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return strings.ToLower(v) == "true"
		case int:
			return v != 0
		}
	}
	if len(def) > 0 {
		return def[0]
	}
	return false
}

// GetStringSlice 获取字符串数组
func (c Config) GetStringSlice(key string, def ...[]string) []string {
	if val := c[key]; val != nil {
		switch v := val.(type) {
		case []string:
			return v
		case []interface{}:
			var result []string
			for _, item := range v {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		case string:
			return strings.Split(v, ",")
		}
	}
	if len(def) > 0 {
		return def[0]
	}
	return nil
}

// GetSubConfig 获取子配置
func (c Config) GetSubConfig(key string) Config {
	if val := c[key]; val != nil {
		switch v := val.(type) {
		case map[string]interface{}:
			return Config(v)
		case Config:
			return v
		}
	}
	return Config{}
}

// Merge 合并配置（后面的覆盖前面的）
func (c Config) Merge(other Config) Config {
	result := Config{}

	// 复制当前配置
	for k, v := range c {
		result[k] = v
	}

	// 覆盖/添加其他配置
	for k, v := range other {
		result[k] = v
	}

	return result
}

// ToJSON 转换为JSON
func (c Config) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// FromJSON 从JSON解析
func FromJSON(data []byte) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
