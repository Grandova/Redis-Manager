package redis

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type ConfigItem struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Comment string `json:"comment"`
}

type ParsedConfig struct {
	Items map[string]string `json:"items"`
	Lines []string          `json:"lines"`
}

// ParseConfigFile parses a redis.conf file into a key-value dictionary and preserves comments
func ParseConfigFile(filePath string) (*ParsedConfig, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法读取配置文件 %s: %w", filePath, err)
	}
	defer file.Close()

	items := make(map[string]string)
	var lines []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		rawLine := scanner.Text()
		lines = append(lines, rawLine)
		trimmed := strings.TrimSpace(rawLine)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) >= 2 {
			key := strings.ToLower(fields[0])
			val := strings.Join(fields[1:], " ")
			// Remove surrounding quotes if any
			if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") && len(val) >= 2 {
				val = val[1 : len(val)-1]
			}
			items[key] = val
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("解析配置文件出错: %w", err)
	}

	return &ParsedConfig{
		Items: items,
		Lines: lines,
	}, nil
}

// UpdateConfigDirectives updates or appends directives in redis.conf content
func UpdateConfigDirectives(originalContent []byte, updates map[string]string) []byte {
	lines := strings.Split(string(originalContent), "\n")
	updatedKeys := make(map[string]bool)

	for i, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) >= 1 {
			key := strings.ToLower(fields[0])
			if newVal, exists := updates[key]; exists {
				if strings.TrimSpace(newVal) == "" {
					lines[i] = fmt.Sprintf("# %s", key)
				} else {
					lines[i] = fmt.Sprintf("%s %s", key, newVal)
				}
				updatedKeys[key] = true
			}
		}
	}

	// Append any new directives that were not found in existing lines
	for k, v := range updates {
		if !updatedKeys[k] && strings.TrimSpace(v) != "" {
			lines = append(lines, fmt.Sprintf("%s %s", k, v))
		}
	}

	return []byte(strings.Join(lines, "\n"))
}

// ValidateConfigSyntax validates common redis.conf syntax rules
func ValidateConfigSyntax(content []byte) error {
	lines := strings.Split(string(content), "\n")
	for lineNum, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			return fmt.Errorf("第 %d 行配置格式错误: '%s' 缺少参数值", lineNum+1, trimmed)
		}

		key := strings.ToLower(fields[0])
		val := fields[1]

		switch key {
		case "port", "tls-port":
			if val != "0" {
				// Must be valid number
				for _, c := range val {
					if c < '0' || c > '9' {
						return fmt.Errorf("第 %d 行端口配置无效: '%s'", lineNum+1, val)
					}
				}
			}
		case "appendonly", "protected-mode", "tls-auth-clients":
			lowerVal := strings.ToLower(val)
			if lowerVal != "yes" && lowerVal != "no" && lowerVal != "optional" {
				return fmt.Errorf("第 %d 行 '%s' 的值必须为 yes 或 no (当前为: %s)", lineNum+1, key, val)
			}
		}
	}
	return nil
}
