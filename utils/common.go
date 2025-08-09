package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

var StopWhenErr = true

func CheckErr(e error) error {
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		fmt.Fprintf(
			os.Stderr,
			"\n%s\n\n%s\n\n",
			strings.Repeat("=", 20),
			"Report the following if it is a bug",
		)
		if StopWhenErr {
			panic(e)
		}
	}
	return e
}

func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "  ")
	return string(s)
}

func SanitizeFileName(title string) string {
	// 特殊字符的智能替换规则
	replacements := map[string]string{
		"/":  "-", // 斜杠用连字符替换（如 JavaScript/TypeScript -> JavaScript-TypeScript）
		"\\": "-", // 反斜杠用连字符替换
		":":  "-", // 冒号用连字符替换
		"*":  "★", // 星号用星形符号替换
		"?":  "？", // 问号用中文问号替换
		"\"": "'", // 双引号用单引号替换
		"<":  "《", // 小于号用中文书名号替换
		">":  "》", // 大于号用中文书名号替换
		"|":  "-", // 竖线用连字符替换
	}

	// 应用替换规则
	for invalid, replacement := range replacements {
		title = strings.ReplaceAll(title, invalid, replacement)
	}

	// 移除首尾空白字符
	title = strings.TrimSpace(title)

	// 如果文件名为空或只包含点，使用默认名称
	if title == "" || title == "." || title == ".." {
		title = "untitled"
	}

	return title
}
