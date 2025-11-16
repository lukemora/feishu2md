// Package imgbed 提供图床上传功能
// 支持多种图床平台（阿里云OSS、腾讯云COS等）
package imgbed

import "context"

// Platform 图床平台接口
type Platform interface {
	// Upload 上传图片到图床
	// buffer: 图片二进制数据
	// filename: 文件名
	// 返回图床URL和错误
	Upload(ctx context.Context, buffer []byte, filename string) (string, error)

	// GetName 获取平台名称
	GetName() string

	// BuildURL 根据文件名构建图床URL（不检查是否存在）
	BuildURL(filename string) string

	// CheckExists 检查文件是否已存在于图床
	// 返回 true 表示存在，并返回完整URL
	CheckExists(ctx context.Context, filename string) (bool, string)

	// FindByPrefix 通过前缀查找文件（不带扩展名）
	// prefix: 文件token（不含扩展名）
	// 返回 true 表示找到，并返回完整URL和文件名
	FindByPrefix(ctx context.Context, prefix string) (bool, string, string)
}

// UploadResult 上传结果
type UploadResult struct {
	LocalPath string // 本地路径
	URL       string // 图床URL
	Error     error  // 错误信息
}
