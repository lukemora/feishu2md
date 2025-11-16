// Package imgbed - 阿里云OSS图床实现
package imgbed

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/Perfecto23/feishu2md/core"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// OSSPlatform 阿里云OSS平台
type OSSPlatform struct {
	config *core.ImageBedConfig
	client *oss.Client
	bucket *oss.Bucket
}

// NewOSSPlatform 创建阿里云OSS平台实例
func NewOSSPlatform(cfg *core.ImageBedConfig) (*OSSPlatform, error) {
	// 构建endpoint
	endpoint := fmt.Sprintf("https://oss-%s.aliyuncs.com", cfg.Region)

	// 创建OSS客户端
	client, err := oss.New(endpoint, cfg.SecretID, cfg.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("创建OSS客户端失败: %w", err)
	}

	// 获取Bucket
	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("获取Bucket失败: %w", err)
	}

	return &OSSPlatform{
		config: cfg,
		client: client,
		bucket: bucket,
	}, nil
}

// GetName 获取平台名称
func (p *OSSPlatform) GetName() string {
	return "阿里云OSS"
}

// Upload 上传图片到OSS
func (p *OSSPlatform) Upload(ctx context.Context, buffer []byte, filename string) (string, error) {
	// 构建对象键（带路径前缀）
	objectKey := p.getObjectKey(filename)

	// 上传文件
	err := p.bucket.PutObject(objectKey, bytes.NewReader(buffer))
	if err != nil {
		return "", fmt.Errorf("上传失败: %w", err)
	}

	// 构建并返回URL
	url := p.getObjectURL(objectKey)
	return url, nil
}

// getObjectKey 获取对象键（带路径前缀）
func (p *OSSPlatform) getObjectKey(filename string) string {
	if p.config.PrefixKey != "" {
		return path.Join(p.config.PrefixKey, filename)
	}
	return filename
}

// getObjectURL 获取对象URL
func (p *OSSPlatform) getObjectURL(objectKey string) string {
	// 如果配置了自定义域名，使用自定义域名
	if p.config.Host != "" {
		host := strings.TrimPrefix(p.config.Host, "https://")
		host = strings.TrimPrefix(host, "http://")
		return fmt.Sprintf("https://%s/%s", host, objectKey)
	}

	// 使用默认的OSS域名
	return fmt.Sprintf("https://%s.oss-%s.aliyuncs.com/%s",
		p.config.Bucket, p.config.Region, objectKey)
}

// BuildURL 根据文件名构建图床URL（不检查是否存在）
func (p *OSSPlatform) BuildURL(filename string) string {
	objectKey := p.getObjectKey(filename)
	return p.getObjectURL(objectKey)
}

// CheckExists 检查文件是否已存在于图床
func (p *OSSPlatform) CheckExists(ctx context.Context, filename string) (bool, string) {
	objectKey := p.getObjectKey(filename)
	url := p.getObjectURL(objectKey)

	// 使用 IsObjectExist 检查对象是否存在
	exists, err := p.bucket.IsObjectExist(objectKey)
	if err != nil {
		return false, url
	}

	return exists, url
}

// FindByPrefix 通过前缀查找文件（支持任意扩展名）
func (p *OSSPlatform) FindByPrefix(ctx context.Context, prefix string) (bool, string, string) {
	// 构建对象前缀（带路径）
	objectPrefix := p.getObjectKey(prefix)

	// 使用 ListObjects 查找以该前缀开头的对象
	marker := ""
	for {
		lsRes, err := p.bucket.ListObjects(oss.Prefix(objectPrefix), oss.Marker(marker), oss.MaxKeys(10))
		if err != nil {
			return false, "", ""
		}

		// 查找第一个匹配的对象
		for _, object := range lsRes.Objects {
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

		// 如果没有更多结果，退出
		if !lsRes.IsTruncated {
			break
		}
		marker = lsRes.NextMarker
	}

	return false, "", ""
}

