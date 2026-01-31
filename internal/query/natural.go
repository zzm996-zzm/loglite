package query

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/loglite/loglite/internal/storage"
)

// NaturalQueryParser 自然语言查询解析器
type NaturalQueryParser struct{}

// NewNaturalQueryParser 创建解析器
func NewNaturalQueryParser() *NaturalQueryParser {
	return &NaturalQueryParser{}
}

// Parse 解析自然语言查询
func (p *NaturalQueryParser) Parse(query string) storage.QueryParams {
	params := storage.QueryParams{}
	query = strings.TrimSpace(query)

	// 解析时间范围
	params.StartTime, params.EndTime = p.parseTimeRange(query)

	// 解析日志级别
	params.Level = p.parseLevel(query)

	// 解析服务名
	params.Service = p.parseService(query)

	// 剩余部分作为关键词
	keyword := p.extractKeyword(query)
	// 如果关键词太短（小于2个字符），认为是噪音，忽略
	// 如果关键词和服务名相同，也忽略
	if len([]rune(keyword)) >= 2 && keyword != params.Service {
		params.Keyword = keyword
	}

	return params
}

// parseTimeRange 解析时间范围
func (p *NaturalQueryParser) parseTimeRange(query string) (start, end time.Time) {
	now := time.Now()
	query = strings.ToLower(query)

	// 相对时间模式
	patterns := map[string]func() (time.Time, time.Time){
		// 最近 X 分钟/小时/天
		`最近(\d+)分钟`: func() (time.Time, time.Time) {
			if m := regexp.MustCompile(`最近(\d+)分钟`).FindStringSubmatch(query); len(m) > 1 {
				if n, _ := strconv.Atoi(m[1]); n > 0 {
					return now.Add(-time.Duration(n) * time.Minute), now
				}
			}
			return time.Time{}, time.Time{}
		},
		`最近(\d+)小时`: func() (time.Time, time.Time) {
			if m := regexp.MustCompile(`最近(\d+)小时`).FindStringSubmatch(query); len(m) > 1 {
				if n, _ := strconv.Atoi(m[1]); n > 0 {
					return now.Add(-time.Duration(n) * time.Hour), now
				}
			}
			return time.Time{}, time.Time{}
		},
		`最近(\d+)天`: func() (time.Time, time.Time) {
			if m := regexp.MustCompile(`最近(\d+)天`).FindStringSubmatch(query); len(m) > 1 {
				if n, _ := strconv.Atoi(m[1]); n > 0 {
					return now.AddDate(0, 0, -n), now
				}
			}
			return time.Time{}, time.Time{}
		},
	}

	// 检查相对时间
	for pattern, fn := range patterns {
		if regexp.MustCompile(pattern).MatchString(query) {
			return fn()
		}
	}

	// 固定时间词
	timeWords := map[string]func() (time.Time, time.Time){
		"今天": func() (time.Time, time.Time) {
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			return today, now
		},
		"昨天": func() (time.Time, time.Time) {
			yesterday := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, now.Location())
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			return yesterday, today
		},
		"本周": func() (time.Time, time.Time) {
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			weekStart := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
			return weekStart, now
		},
		"上周": func() (time.Time, time.Time) {
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			weekStart := time.Date(now.Year(), now.Month(), now.Day()-weekday+1-7, 0, 0, 0, 0, now.Location())
			weekEnd := weekStart.AddDate(0, 0, 7)
			return weekStart, weekEnd
		},
		"这个月": func() (time.Time, time.Time) {
			monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			return monthStart, now
		},
		"本月": func() (time.Time, time.Time) {
			monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			return monthStart, now
		},
		"今天上午": func() (time.Time, time.Time) {
			morning := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			noon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
			return morning, noon
		},
		"今天下午": func() (time.Time, time.Time) {
			noon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
			evening := time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location())
			return noon, evening
		},
		"下午": func() (time.Time, time.Time) {
			noon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
			evening := time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location())
			return noon, evening
		},
		"上午": func() (time.Time, time.Time) {
			morning := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			noon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
			return morning, noon
		},
		"今天晚上": func() (time.Time, time.Time) {
			evening := time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location())
			midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			return evening, midnight
		},
		"1小时内": func() (time.Time, time.Time) {
			return now.Add(-1 * time.Hour), now
		},
		"一小时内": func() (time.Time, time.Time) {
			return now.Add(-1 * time.Hour), now
		},
		"半小时内": func() (time.Time, time.Time) {
			return now.Add(-30 * time.Minute), now
		},
		"5分钟内": func() (time.Time, time.Time) {
			return now.Add(-5 * time.Minute), now
		},
		"10分钟内": func() (time.Time, time.Time) {
			return now.Add(-10 * time.Minute), now
		},
		"30分钟内": func() (time.Time, time.Time) {
			return now.Add(-30 * time.Minute), now
		},
	}

	for word, fn := range timeWords {
		if strings.Contains(query, word) {
			return fn()
		}
	}

	// 默认最近1小时
	return now.Add(-1 * time.Hour), now
}

// parseLevel 解析日志级别
func (p *NaturalQueryParser) parseLevel(query string) string {
	query = strings.ToLower(query)

	levelMap := map[string]string{
		"错误":    "error",
		"error": "error",
		"报错":    "error",
		"异常":    "error",
		"失败":    "error",
		"警告":    "warn",
		"warn":  "warn",
		"告警":    "warn",
		"信息":    "info",
		"info":  "info",
		"调试":    "debug",
		"debug": "debug",
	}

	for keyword, level := range levelMap {
		if strings.Contains(query, keyword) {
			return level
		}
	}

	return ""
}

// parseService 解析服务名
func (p *NaturalQueryParser) parseService(query string) string {
	// 匹配 "xxx服务" 或 "service=xxx" 或 "服务xxx"
	patterns := []string{
		`(\w+[-_]?\w*)服务`,
		`服务[=:：]?(\w+[-_]?\w*)`,
		`service[=:：](\w+[-_]?\w*)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if m := re.FindStringSubmatch(query); len(m) > 1 {
			return m[1]
		}
	}

	return ""
}

// extractKeyword 提取关键词
func (p *NaturalQueryParser) extractKeyword(query string) string {
	result := query

	// 移除已解析的时间词（注意顺序：长词先处理，避免误删）
	timeWords := []string{
		"今天上午", "今天下午", "今天晚上",
		"这个月", "一小时内", "半小时内",
		"5分钟内", "10分钟内", "30分钟内", "1小时内",
		"今天", "昨天", "本周", "上周", "本月",
		"上午", "下午", "晚上",
		"最近", "分钟", "小时", "天", "内",
	}
	for _, word := range timeWords {
		result = strings.ReplaceAll(result, word, "")
	}

	// 移除级别词
	levelWords := []string{
		"错误日志", "警告日志", "信息日志", "调试日志",
		"错误", "error", "报错", "异常", "失败",
		"警告", "warn", "告警",
		"信息", "info",
		"调试", "debug",
	}
	for _, word := range levelWords {
		result = strings.ReplaceAll(result, word, "")
	}

	// 移除 "的" "日志" 等填充词
	fillerWords := []string{"的", "日志", "log", "logs", "服务", "所有", "全部", "查询", "查看", "显示"}
	for _, word := range fillerWords {
		result = strings.ReplaceAll(result, word, "")
	}

	// 移除服务名模式
	servicePatterns := []string{
		`(\w+[-_]?\w*)服务`,
		`服务[=:：]?(\w+[-_]?\w*)`,
		`service[=:：]?(\w+[-_]?\w*)`,
	}
	for _, pattern := range servicePatterns {
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllString(result, "")
	}

	// 清理数字和空白
	result = regexp.MustCompile(`\d+`).ReplaceAllString(result, "")
	result = strings.TrimSpace(result)

	return result
}
