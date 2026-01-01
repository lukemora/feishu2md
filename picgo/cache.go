// Package picgo - 上传缓存管理
// 维护 token -> URL 的映射，避免重复上传
// 缓存存储在当前工作目录的 .feishu2md/ 下，便于跟随仓库提交
package picgo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// 缓存文件路径（相对于当前工作目录）
var (
	cacheDir  string
	cacheFile string
	cacheOnce sync.Once
)

// cache 内存缓存
var (
	cache   = make(map[string]string)
	cacheMu sync.RWMutex
	loaded  bool
)

// initCachePath 初始化缓存路径
// 缓存存储在当前工作目录的 .feishu2md/ 下
func initCachePath() {
	cacheOnce.Do(func() {
		// 使用当前工作目录，便于缓存跟随仓库
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		cacheDir = filepath.Join(cwd, ".feishu2md")
		cacheFile = filepath.Join(cacheDir, "upload-cache.json")
	})
}

// loadCache 从文件加载缓存
func loadCache() {
	if loaded {
		return
	}

	initCachePath()

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		// 文件不存在是正常的
		loaded = true
		return
	}

	cacheMu.Lock()
	defer cacheMu.Unlock()

	if err := json.Unmarshal(data, &cache); err != nil {
		// JSON 解析失败，忽略
		cache = make(map[string]string)
	}
	loaded = true
}

// saveCache 保存缓存到文件
func persistCache() error {
	initCachePath()

	// 确保目录存在
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	cacheMu.RLock()
	data, err := json.MarshalIndent(cache, "", "  ")
	cacheMu.RUnlock()

	if err != nil {
		return err
	}

	return os.WriteFile(cacheFile, data, 0644)
}

// GetCached 获取缓存的 URL
func GetCached(token string) (string, bool) {
	loadCache()

	cacheMu.RLock()
	defer cacheMu.RUnlock()

	url, ok := cache[token]
	return url, ok
}

// SaveCache 保存到缓存
func SaveCache(token, url string) {
	loadCache()

	cacheMu.Lock()
	cache[token] = url
	cacheMu.Unlock()

	// 异步持久化，不阻塞主流程
	go func() {
		if err := persistCache(); err != nil {
			// 持久化失败不影响主流程，仅打印警告
			// fmt.Printf("⚠️  缓存持久化失败: %v\n", err)
		}
	}()
}

// ClearCache 清空缓存（用于测试或重置）
func ClearCache() {
	cacheMu.Lock()
	cache = make(map[string]string)
	cacheMu.Unlock()

	initCachePath()
	os.Remove(cacheFile)
}

// CacheSize 返回缓存条目数
func CacheSize() int {
	loadCache()

	cacheMu.RLock()
	defer cacheMu.RUnlock()

	return len(cache)
}
