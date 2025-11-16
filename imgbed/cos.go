// Package imgbed - 腾讯云COS图床实现
package imgbed

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/Perfecto23/feishu2md/core"
	"github.com/tencentyun/cos-go-sdk-v5"
)

// COSPlatform 腾讯云COS平台
type COSPlatform struct {
	config *core.ImageBedConfig
	client *cos.Client
}

// NewCOSPlatform 创建腾讯云COS平台实例
func NewCOSPlatform(cfg *core.ImageBedConfig) (*COSPlatform, error) {
	// 构建Bucket URL
	bucketURL := fmt.Sprintf("https://%s.cos.%s.myqcloud.com", cfg.Bucket, cfg.Region)
	u, err := url.Parse(bucketURL)
	if err != nil {
		return nil, fmt.Errorf("解析Bucket URL失败: %w", err)
	}

	// 创建COS客户端
	b := &cos.BaseURL{BucketURL: u}
	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
		},
	})

	return &COSPlatform{
		config: cfg,
		client: client,
	}, nil
}

// GetName 获取平台名称
func (p *COSPlatform) GetName() string {
	return "腾讯云COS"
}

// Upload 上传图片到COS
func (p *COSPlatform) Upload(ctx context.Context, buffer []byte, filename string) (string, error) {
	// 构建对象键（带路径前缀）
	objectKey := p.getObjectKey(filename)

	// 上传文件
	_, err := p.client.Object.Put(ctx, objectKey, bytes.NewReader(buffer), nil)
	if err != nil {
		return "", fmt.Errorf("上传失败: %w", err)
	}

	// 构建并返回URL
	url := p.getObjectURL(objectKey)
	return url, nil
}

// getObjectKey 获取对象键（带路径前缀）
func (p *COSPlatform) getObjectKey(filename string) string {
	if p.config.PrefixKey != "" {
		return path.Join(p.config.PrefixKey, filename)
	}
	return filename
}

// getObjectURL 获取对象URL
func (p *COSPlatform) getObjectURL(objectKey string) string {
	// 如果配置了自定义域名，使用自定义域名
	if p.config.Host != "" {
		host := strings.TrimPrefix(p.config.Host, "https://")
		host = strings.TrimPrefix(host, "http://")
		return fmt.Sprintf("https://%s/%s", host, objectKey)
	}

	// 使用默认的COS域名
	return fmt.Sprintf("https://%s.cos.%s.myqcloud.com/%s",
		p.config.Bucket, p.config.Region, objectKey)
}

// BuildURL 根据文件名构建图床URL（不检查是否存在）
func (p *COSPlatform) BuildURL(filename string) string {
	objectKey := p.getObjectKey(filename)
	return p.getObjectURL(objectKey)
}

// CheckExists 检查文件是否已存在于图床
func (p *COSPlatform) CheckExists(ctx context.Context, filename string) (bool, string) {
	objectKey := p.getObjectKey(filename)
	url := p.getObjectURL(objectKey)

	// 使用 Head 检查对象是否存在
	_, err := p.client.Object.Head(ctx, objectKey, nil)
	if err != nil {
		// 如果出错（比如404），说明不存在
		return false, url
	}

	return true, url
}

// FindByPrefix 通过前缀查找文件（支持任意扩展名）
func (p *COSPlatform) FindByPrefix(ctx context.Context, prefix string) (bool, string, string) {
	// 构建对象前缀（带路径）
	objectPrefix := p.getObjectKey(prefix)

	// 使用 Get 方法列出对象
	opt := &cos.BucketGetOptions{
		Prefix:  objectPrefix,
		MaxKeys: 10,
	}

	result, _, err := p.client.Bucket.Get(ctx, opt)
	if err != nil {
		return false, "", ""
	}

	// 查找第一个匹配的对象
	for _, object := range result.Contents {
		// 提取文件名（去除路径前缀）
		filename := strings.TrimPrefix(object.Key, p.config.PrefixKey)
		if strings.TrimPrefix(filename, "/") != "" {
			filename = strings.TrimPrefix(filename, "/")
		}
		
		// 检查是否以指定前缀开头
		if strings.HasPrefix(filename, prefix) {
			url := p.getObjectURL(object.Key)
			return true, url, filename
		}
	}

	return false, "", ""
}
