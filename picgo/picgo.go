// Package picgo 提供 PicGo CLI 的 Go 封装
// 通过调用 picgo 命令行工具实现图片上传
package picgo

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// 默认配置
const (
	DefaultTimeout   = 120 * time.Second // 单张图片上传超时
	MaxUploadRetries = 2                 // 最大重试次数
	BatchConcurrency = 10                // 批量上传并发数
)

// urlPattern 用于从 picgo 输出中提取 URL
var urlPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

// IsAvailable 检测 picgo CLI 是否可用
func IsAvailable() bool {
	_, err := exec.LookPath("picgo")
	return err == nil
}

// GetVersion 获取 picgo 版本信息
func GetVersion() (string, error) {
	cmd := exec.Command("picgo", "-v")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// Upload 上传单张图片到图床
// filePath: 本地图片路径
// 返回图床 URL
func Upload(filePath string) (string, error) {
	return UploadWithContext(context.Background(), filePath)
}

// UploadWithContext 带上下文的上传
func UploadWithContext(ctx context.Context, filePath string) (string, error) {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	// 执行 picgo 命令（不使用静默模式，以便获取完整输出）
	cmd := exec.CommandContext(ctx, "picgo", "u", filePath)
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	if err != nil {
		// 检查是否超时
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("上传超时（%v）: %s", DefaultTimeout, filePath)
		}
		return "", fmt.Errorf("picgo 上传失败: %v\n输出: %s", err, outputStr)
	}

	// 解析 URL
	url := extractURL(outputStr)
	if url == "" {
		// 输出更详细的调试信息
		if outputStr == "" {
			return "", fmt.Errorf("picgo 无输出，请检查配置: picgo config")
		}
		return "", fmt.Errorf("未能从输出中解析 URL，picgo 输出:\n%s", outputStr)
	}

	return url, nil
}

// extractURL 从 picgo 输出中提取 URL
// 优先取最后一个 URL（通常是最终结果）
func extractURL(output string) string {
	matches := urlPattern.FindAllString(output, -1)
	if len(matches) == 0 {
		return ""
	}
	// 返回最后一个匹配的 URL
	return matches[len(matches)-1]
}

// BatchUploadResult 批量上传结果
type BatchUploadResult struct {
	LocalPath string
	URL       string
	Error     error
}

// BatchUpload 批量上传图片
// 返回 localPath -> URL 的映射（仅包含成功的）
func BatchUpload(ctx context.Context, filePaths []string) map[string]string {
	if len(filePaths) == 0 {
		return make(map[string]string)
	}

	results := make(map[string]string, len(filePaths))
	var mu sync.Mutex

	// 并发控制
	semaphore := make(chan struct{}, BatchConcurrency)
	var wg sync.WaitGroup

	for _, path := range filePaths {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(filePath string) {
			defer func() {
				<-semaphore
				wg.Done()
			}()

			// 先检查缓存
			token := extractTokenFromPath(filePath)
			if token != "" {
				if cachedURL, ok := GetCached(token); ok {
					mu.Lock()
					results[filePath] = cachedURL
					mu.Unlock()
					return
				}
			}

			// 上传
			url, err := UploadWithContext(ctx, filePath)
			if err != nil {
				fmt.Printf("⚠️  上传失败 %s: %v\n", filePath, err)
				return
			}

			mu.Lock()
			results[filePath] = url
			mu.Unlock()

			// 保存缓存
			if token != "" {
				SaveCache(token, url)
			}
		}(path)
	}

	wg.Wait()
	return results
}

// extractTokenFromPath 从文件路径中提取 token（文件名不含扩展名）
func extractTokenFromPath(filePath string) string {
	// 提取文件名
	parts := strings.Split(filePath, "/")
	if len(parts) == 0 {
		return ""
	}
	filename := parts[len(parts)-1]

	// 移除扩展名
	if idx := strings.LastIndex(filename, "."); idx > 0 {
		return filename[:idx]
	}
	return filename
}
