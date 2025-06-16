package main

import (
	"fmt"
	"regexp"
)

func extractJSON(s string) (string, error) {
	re := regexp.MustCompile("(?s)```json\\s*(\\{.*?\\})\\s*```")
	match := re.FindStringSubmatch(s)
	if len(match) >= 2 {
		jsonStr := match[1]
		return jsonStr, nil
	}
	return "", fmt.Errorf("未找到 JSON 代码块")
}
