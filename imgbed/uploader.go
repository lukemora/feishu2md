// Package imgbed - 图片上传核心逻辑
package imgbed

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Perfecto23/feishu2md/core"
)

// Uploader 图片上传器
type Uploader struct {
	config   *core.ImageBedConfig
	platform Platform
}

// NewUploader 创建图片上传器
func NewUploader(cfg *core.ImageBedConfig) (*Uploader, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("图床上传功能未启用")
	}

	// 验证必需配置
	if cfg.Platform == "" {
		return nil, fmt.Errorf("未指定图床平台")
	}
	if cfg.SecretID == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("图床密钥配置不完整")
	}
	if cfg.Bucket == "" || cfg.Region == "" {
		return nil, fmt.Errorf("图床存储桶或区域配置不完整")
	}

	// 创建对应的图床平台实例
	var platform Platform
	var err error

	switch cfg.Platform {
	case "oss":
		platform, err = NewOSSPlatform(cfg)
	case "cos":
		platform, err = NewCOSPlatform(cfg)
	default:
		return nil, fmt.Errorf("不支持的图床平台: %s (支持: oss, cos)", cfg.Platform)
	}

	if err != nil {
		return nil, fmt.Errorf("创建图床平台失败: %w", err)
	}

	return &Uploader{
		config:   cfg,
		platform: platform,
	}, nil
}

// GetPlatform 获取图床平台实例
func (u *Uploader) GetPlatform() Platform {
	return u.platform
}

// UploadFromLocal 从本地文件上传到图床
// localPath: 本地文件路径（相对于工作目录）
// 返回图床URL和错误
func (u *Uploader) UploadFromLocal(ctx context.Context, localPath string) (string, error) {
	// 读取本地文件
	buffer, err := os.ReadFile(localPath)
	if err != nil {
		return "", fmt.Errorf("读取本地文件失败: %w", err)
	}

	// 提取文件名
	filename := filepath.Base(localPath)

	// 上传到图床
	url, err := u.platform.Upload(ctx, buffer, filename)
	if err != nil {
		return "", fmt.Errorf("上传到%s失败: %w", u.platform.GetName(), err)
	}

	return url, nil
}

// BatchUploadFromLocal 批量上传本地文件到图床（并发）
// localPaths: 本地文件路径列表
// 返回路径到URL的映射
func (u *Uploader) BatchUploadFromLocal(ctx context.Context, localPaths []string) map[string]string {
	results := make(map[string]string, len(localPaths))
	if len(localPaths) == 0 {
		return results
	}

	// 图床上传并发：20个goroutine
	// 图床API不受飞书限流影响，可以高并发
	maxConcurrency := 20

	type uploadResult struct {
		path string
		url  string
		err  error
	}

	jobs := make(chan string, len(localPaths))
	resultChan := make(chan uploadResult, len(localPaths))

	// 启动worker池
	for i := 0; i < maxConcurrency; i++ {
		go func() {
			for localPath := range jobs {
				url, err := u.UploadFromLocal(ctx, localPath)
				resultChan <- uploadResult{
					path: localPath,
					url:  url,
					err:  err,
				}
			}
		}()
	}

	// 发送任务
	for _, path := range localPaths {
		jobs <- path
	}
	close(jobs)

	// 收集结果
	for i := 0; i < len(localPaths); i++ {
		result := <-resultChan
		if result.err != nil {
			log.Printf("⚠️  上传失败 %s: %v", result.path, result.err)
			continue
		}
		results[result.path] = result.url
	}

	return results
}

// IsEnabled 检查图床上传是否启用
func IsEnabled(cfg *core.ImageBedConfig) bool {
	return cfg != nil && cfg.Enabled
}
