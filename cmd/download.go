// Package main 包含将飞书文档转换为Markdown的下载功能
// 此文件处理核心下载操作，包括单个文档、批量文件夹和知识库
package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/88250/lute"
	"github.com/Perfecto23/feishu2md/core"
	"github.com/Perfecto23/feishu2md/imgbed"
	"github.com/Perfecto23/feishu2md/utils"
	"github.com/chyroc/lark"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

// DownloadOpts 包含下载操作的选项
type DownloadOpts struct {
	outputDir     string   // 文件保存的目录
	dumpJSON      bool     // 是否转储API的JSON响应
	skipDuplicate bool     // 是否跳过重复文件
	forceDownload bool     // 是否强制下载
	spaceID       string   // 知识库空间ID（用于检查子节点）
	nodeToken     string   // 当前节点令牌（用于检查子节点）
	relDir        string   // 相对根输出目录的路径（仅 wiki-tree 用于日志排序）
	tags          []string // 标签列表
	categories    []string // 分类列表（支持多层级）
	tagMode       string   // 标签模式: "last"(只取最后一层) / "all"(取所有层级)
	categoryMode  string   // 分类模式: "last"(只取最后一层) / "all"(取所有层级)
}

// calculateMD5 计算字符串的MD5哈希值
func calculateMD5(content string) string {
	h := md5.New()
	io.WriteString(h, content)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// fileExists 检查文件是否存在
func fileExists(filepath string) bool {
	_, err := os.Stat(filepath)
	return !os.IsNotExist(err)
}

// shouldSkipFile 检查是否应该跳过文件下载（基于内容对比）
func shouldSkipFile(outputPath, content string, skipDuplicate bool) bool {
	if !skipDuplicate {
		return false
	}

	if !fileExists(outputPath) {
		return false
	}

	// 读取现有文件内容
	existingContent, err := os.ReadFile(outputPath)
	if err != nil {
		// 读取失败，不跳过
		return false
	}

	// 对比MD5哈希值
	existingMD5 := calculateMD5(string(existingContent))
	newMD5 := calculateMD5(content)

	return existingMD5 == newMD5
}

// dlConfig 保存当前下载操作的配置
var dlConfig core.Config

// DownloadStats 用于跨文档统计下载/缓存命中等信息（主要用于 wiki-tree 汇总）
type DownloadStats struct {
	mu          sync.Mutex
	totalDocs   int
	docsNew     int
	totalImages int
	imagesNew   int
}

func (s *DownloadStats) SetTotalDocs(n int) {
	s.mu.Lock()
	s.totalDocs = n
	s.mu.Unlock()
}
func (s *DownloadStats) AddDocNew() {
	s.mu.Lock()
	s.docsNew++
	s.mu.Unlock()
}
func (s *DownloadStats) AddImages(encountered, newlyDownloaded int) {
	s.mu.Lock()
	s.totalImages += encountered
	s.imagesNew += newlyDownloaded
	s.mu.Unlock()
}
func (s *DownloadStats) Snapshot() (totalDocs, docsNew, totalImages, imagesNew int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.totalDocs, s.docsNew, s.totalImages, s.imagesNew
}

// dlStats 在 wiki-tree 模式下初始化用于统计；其他模式保持 nil
var dlStats *DownloadStats

// DocLog 记录单篇文档的处理情况
type DocLog struct {
	Path     string
	Skipped  bool
	Reason   string
	ImgCache int
	ImgNew   int
	DocNew   bool // 仅当首次创建文件时记为 true
}

type LogCollector struct {
	mu   sync.Mutex
	logs []DocLog
}

func (lc *LogCollector) Add(l DocLog) {
	lc.mu.Lock()
	lc.logs = append(lc.logs, l)
	lc.mu.Unlock()
}

func (lc *LogCollector) SortedByPath() []DocLog {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	out := make([]DocLog, len(lc.logs))
	copy(out, lc.logs)
	// 简单按 Path 字典序排序，接近文档层级顺序
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

var logCollector = &LogCollector{}

// deriveTagsFromPath 根据 tagMode 从相对路径推导标签
// tagMode="last": 只取最后一层目录作为 tag（默认行为）
// tagMode="all": 取路径的所有层级目录作为 tags
func deriveTagsFromPath(relPath string, tagMode string) []string {
	cleanPath := filepath.Clean(relPath)
	if cleanPath == "." || cleanPath == string(os.PathSeparator) || cleanPath == "" {
		return nil
	}

	if tagMode == "all" {
		// 取所有层级目录
		parts := strings.Split(cleanPath, string(os.PathSeparator))
		var tags []string
		for _, part := range parts {
			if part != "" && part != "." {
				tags = append(tags, part)
			}
		}
		return tags
	}

	// 默认: 只取直接父目录作为 tag
	parentDir := filepath.Base(cleanPath)
	if parentDir == "" || parentDir == "." {
		return nil
	}
	return []string{parentDir}
}

// deriveCategoriesFromPath 根据 categoryMode 从相对路径推导分类
// categoryMode="last": 返回单元素数组，只包含最后一层目录
// categoryMode="all": 返回所有层级目录
func deriveCategoriesFromPath(relPath string, categoryMode string) []string {
	cleanPath := filepath.Clean(relPath)
	if cleanPath == "." || cleanPath == string(os.PathSeparator) || cleanPath == "" {
		return nil
	}

	if categoryMode == "all" {
		// 取所有层级目录
		parts := strings.Split(cleanPath, string(os.PathSeparator))
		var categories []string
		for _, part := range parts {
			if part != "" && part != "." {
				categories = append(categories, part)
			}
		}
		return categories
	}

	// 默认: 只取最后一层目录
	parentDir := filepath.Base(cleanPath)
	if parentDir == "" || parentDir == "." {
		return nil
	}
	return []string{parentDir}
}

// downloadDocument 下载单个飞书文档并转换为Markdown
// 它处理文档验证、内容检索、图片处理和文件输出
func downloadDocument(ctx context.Context, client *core.Client, url string, opts *DownloadOpts) error {
	// 验证URL并提取文档类型和令牌
	docType, docToken, err := utils.ValidateDocumentURL(url)
	if err != nil {
		return err
	}
	// 移除冗余的令牌输出

	// 对于知识库页面，我们需要先更新docType和docToken
	if docType == "wiki" {
		node, err := client.GetWikiNodeInfo(ctx, docToken)
		if err != nil {
			err = fmt.Errorf("GetWikiNodeInfo err: %v for %v", err, url)
		}
		utils.CheckErr(err)
		docType = node.ObjType
		docToken = node.ObjToken

		// 如果提供了spaceID，检查该节点是否有子节点
		if opts.spaceID != "" {
			childNodes, err := client.GetChildNodes(ctx, opts.spaceID, node.NodeToken)
			if err == nil && len(childNodes) > 0 {
				fmt.Printf("⏭️  跳过有子节点的文档: %s\n", node.Title)
				return nil
			}
		}
	}
	if docType == "docs" {
		return errors.Errorf(
			`不再支持飞书文档。` +
				`请参考Readme/Release获取v1_support信息。`)
	}

	// 处理下载：先快速获取文档元信息（包含 RevisionID），用于命中跳过
	meta, err := client.GetDocxDocumentMeta(ctx, docToken)
	utils.CheckErr(err)

	// 如果开启跳过重复，并且本地存在同名 md 文件，同时可读取历史 RevisionID，且一致，则直接跳过
	// 仅在使用标题作为文件名时，文件名依赖 meta.Title；否则用 token
	mdName := fmt.Sprintf("%s.md", docToken)
	if dlConfig.Output.TitleAsFilename {
		mdName = fmt.Sprintf("%s.md", utils.SanitizeFileName(meta.Title))
	}
	outputPath := filepath.Join(opts.outputDir, mdName)

	// 未命中快速跳过，拉取块内容
	docx, blocks, err := client.GetDocxContent(ctx, docToken)
	utils.CheckErr(err)

	parser := core.NewParser(dlConfig.Output)

	markdown := parser.ParseDocxContent(docx, blocks)

	if !dlConfig.Output.SkipImgDownload && len(parser.ImgTokens) > 0 {
		// 对图片 token 去重，避免重复下载
		uniqueTokens := make([]string, 0, len(parser.ImgTokens))
		seen := make(map[string]struct{}, len(parser.ImgTokens))
		for _, t := range parser.ImgTokens {
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			uniqueTokens = append(uniqueTokens, t)
		}

		// 初始化图床上传器（如果启用了图床）
		var uploader *imgbed.Uploader
		if imgbed.IsEnabled(&dlConfig.ImageBed) {
			var err error
			uploader, err = imgbed.NewUploader(&dlConfig.ImageBed)
			if err != nil {
				fmt.Printf("⚠️  创建图床上传器失败: %v\n", err)
				uploader = nil
			}
		}

		// 控制单文档内图片下载并发度
		// 提高到16个并发（限流器会自动控制）
		maxImgConcurrency := 16
		type result struct {
			token, link string
			fromImgbed  bool // 是否从图床直接获取
			needUpload  bool // 是否需要上传到图床
			err         error
		}
		jobs := make(chan string)
		results := make(chan result, len(uniqueTokens))
		outImgDir := filepath.Join(opts.outputDir, dlConfig.Output.ImageDir)

		worker := func() {
			for token := range jobs {
				// 优化：如果启用图床，用token前缀查找（支持任意扩展名）
				if uploader != nil {
					platform := uploader.GetPlatform()

					// 1. 通过前缀查找图床（无需猜测扩展名，无需调用飞书API！）
					found, imgbedURL, _ := platform.FindByPrefix(ctx, token)
					if found {
						// 图床已存在，直接使用图床URL，完全跳过下载！⚡
						results <- result{token: token, link: imgbedURL, fromImgbed: true, needUpload: false, err: nil}
						continue
					}
				}

				// 2. 图床不存在或未启用图床，从飞书下载
				localLink, err := client.DownloadImage(ctx, token, outImgDir)
				if err != nil {
					results <- result{token: token, link: "", fromImgbed: false, needUpload: false, err: err}
					continue
				}

				// 3. 下载成功，如果启用了图床，标记需要上传
				if uploader != nil {
					results <- result{token: token, link: localLink, fromImgbed: false, needUpload: true, err: nil}
				} else {
					// 未启用图床，使用本地路径
					results <- result{token: token, link: localLink, fromImgbed: false, needUpload: false, err: nil}
				}
			}
		}
		for i := 0; i < maxImgConcurrency; i++ {
			go worker()
		}
		for _, token := range uniqueTokens {
			jobs <- token
		}
		close(jobs)

		// 收集结果并替换链接
		successCount := 0
		imgbedHitCount := 0
		failedTokens := 0
		tokenToLink := make(map[string]string, len(uniqueTokens))
		needUploadImages := make(map[string]string) // 记录需要上传到图床的图片
		for i := 0; i < len(uniqueTokens); i++ {
			r := <-results
			if r.err != nil {
				fmt.Printf("⚠️  图片下载失败: %v\n", r.err)
				failedTokens++
				continue
			}
			tokenToLink[r.token] = r.link
			successCount++

			if r.fromImgbed {
				// 从图床直接获取
				imgbedHitCount++
			} else if r.needUpload {
				// 需要上传到图床
				needUploadImages[r.token] = r.link
			}
		}

		// 一次性替换，避免多次 strings.Replace 带来的重复扫描
		if successCount > 0 {
			// 如果有图片需要上传到图床
			uploadedCount := 0
			if uploader != nil && len(needUploadImages) > 0 {
				// 收集需要上传的图片路径
				localPaths := make([]string, 0, len(needUploadImages))
				for _, link := range needUploadImages {
					fullPath := filepath.Join(opts.outputDir, link)
					localPaths = append(localPaths, fullPath)
				}

				// 批量上传到图床
				imgbedURLs := uploader.BatchUploadFromLocal(ctx, localPaths)

				// 替换tokenToLink中的链接为图床URL，并删除已上传的本地文件
				for token, link := range needUploadImages {
					fullPath := filepath.Join(opts.outputDir, link)
					if imgbedURL, ok := imgbedURLs[fullPath]; ok {
						tokenToLink[token] = imgbedURL
						uploadedCount++

						// 上传成功后删除本地图片
						os.Remove(fullPath)
					}
				}

				// 尝试删除空的图片目录
				imgDir := filepath.Join(opts.outputDir, dlConfig.Output.ImageDir)
				if entries, err := os.ReadDir(imgDir); err == nil && len(entries) == 0 {
					os.Remove(imgDir)
				}
			}

			// 替换markdown中的token为最终链接（本地链接或图床链接）
			for token, link := range tokenToLink {
				markdown = strings.ReplaceAll(markdown, token, link)
			}

			if dlStats != nil {
				// 注意：successCount 包含从飞书下载的图片（需要上传的）
				// imgbedHitCount 是从图床直接获取的（不算新增）
				downloaded := len(needUploadImages) // 只有需要上传的才是真正新下载的
				dlStats.AddImages(len(uniqueTokens), downloaded)
				// 把图片统计合并到当前文档日志（最后汇总输出）
				pathForLog := mdName
				if opts.relDir != "" {
					pathForLog = filepath.Join(opts.relDir, mdName)
				}
				logCollector.Add(DocLog{Path: pathForLog, ImgCache: imgbedHitCount, ImgNew: downloaded})
			}
		}
	}

	// Format the markdown document
	engine := lute.New(func(l *lute.Lute) {
		l.RenderOptions.AutoSpace = true
	})
	result := engine.FormatStr("md", markdown)

	// 构建 frontmatter（MDX/YAML）
	// 标题
	fmTitle := meta.Title
	// 获取时间元数据
	var fmDate, fmUpdated string
	if createdAt, updatedAt, terr := client.GetDocxTimes(ctx, docToken); terr == nil {
		// 固定东八区 +08:00
		loc, _ := time.LoadLocation("Asia/Shanghai")
		if createdAt != nil {
			fmDate = createdAt.In(loc).Format("2006-01-02T15:04:05-07:00")
		}
		if updatedAt != nil {
			fmUpdated = updatedAt.In(loc).Format("2006-01-02T15:04:05-07:00")
		}
	}
	// 兜底：若时间缺失，使用当前时间
	if fmDate == "" || fmUpdated == "" {
		now := time.Now().In(time.FixedZone("CST-8", 8*3600))
		if fmDate == "" {
			fmDate = now.Format("2006-01-02T15:04:05-07:00")
		}
		if fmUpdated == "" {
			fmUpdated = now.Format("2006-01-02T15:04:05-07:00")
		}
	}
	// YAML 转义标题中的冒号等
	escapeYAML := func(s string) string {
		// 简单处理：若包含特殊字符，则使用双引号并转义
		special := ":-#{}[],&*?|\"<>=!%@`) \\" // 包含引号、反斜线与常见特殊字符
		if strings.ContainsAny(s, special) {
			// 转义双引号与反斜线
			s = strings.ReplaceAll(s, "\\", "\\\\")
			s = strings.ReplaceAll(s, "\"", "\\\"")
			return "\"" + s + "\""
		}
		return s
	}
	var fmBuilder strings.Builder
	fmBuilder.WriteString("---\n")
	fmBuilder.WriteString("title: " + escapeYAML(fmTitle) + "\n")
	fmBuilder.WriteString("date: " + fmDate + "\n")
	fmBuilder.WriteString("updated: " + fmUpdated + "\n")

	// categories: 使用提供的 categories，或从 tags 推导，或使用默认分类
	fmCategories := opts.categories
	if len(fmCategories) == 0 && len(opts.tags) > 0 {
		fmCategories = opts.tags // 使用 tags 作为 categories
	}
	if len(fmCategories) == 0 {
		fmCategories = []string{"未分类"} // 默认分类
	}
	fmBuilder.WriteString("categories:\n")
	for _, cat := range fmCategories {
		if strings.TrimSpace(cat) == "" {
			continue
		}
		fmBuilder.WriteString("  - " + escapeYAML(cat) + "\n")
	}

	// tags: 输出标签列表
	if len(opts.tags) > 0 {
		fmBuilder.WriteString("tags:\n")
		for _, tag := range opts.tags {
			if strings.TrimSpace(tag) == "" {
				continue
			}
			fmBuilder.WriteString("  - " + escapeYAML(tag) + "\n")
		}
	}
	// id: 使用 docToken 作为唯一标识
	fmBuilder.WriteString("id: " + escapeYAML(docToken) + "\n")
	fmBuilder.WriteString("---\n\n")

	// 合并 frontmatter 与正文
	result = fmBuilder.String() + result

	// 处理输出目录和名称
	if _, err := os.Stat(opts.outputDir); os.IsNotExist(err) {
		if err := os.MkdirAll(opts.outputDir, 0o755); err != nil {
			return err
		}
	}

	if opts.dumpJSON {
		jsonName := fmt.Sprintf("%s.json", docToken)
		jsonOutputPath := filepath.Join(opts.outputDir, jsonName)
		data := struct {
			Document *lark.DocxDocument `json:"document"`
			Blocks   []*lark.DocxBlock  `json:"blocks"`
		}{
			Document: docx,
			Blocks:   blocks,
		}
		pdata := utils.PrettyPrint(data)

		// 检查JSON文件是否需要跳过
		if !opts.forceDownload && shouldSkipFile(jsonOutputPath, pdata, opts.skipDuplicate) {
			fmt.Printf("⏭️  跳过重复JSON: %s\n", jsonName)
		} else {
			if err = os.WriteFile(jsonOutputPath, []byte(pdata), 0o644); err != nil {
				return err
			}
			fmt.Printf("📄 JSON响应已转储到 %s\n", jsonOutputPath)
		}
	}

	// 写入markdown文件

	// 检查是否需要跳过重复文件
	if !opts.forceDownload && shouldSkipFile(outputPath, result, opts.skipDuplicate) {
		// 静默跳过，不输出日志
		return nil
	}

	if err = os.WriteFile(outputPath, []byte(result), 0o644); err != nil {
		return err
	}
	// 静默完成，不输出日志（在最后统计输出）
	if dlStats != nil {
		dlStats.AddDocNew()
		// 记录文档新增日志（图片统计在前面 AddImages 已做累加）
		pathForLog := mdName
		if opts.relDir != "" {
			pathForLog = filepath.Join(opts.relDir, mdName)
		}
		logCollector.Add(DocLog{Path: pathForLog, DocNew: true})
	}

	return nil
}

// downloadDocuments 下载文件夹中的所有文档
func downloadDocuments(ctx context.Context, client *core.Client, url string, opts *DownloadOpts) error {
	// 验证要下载的URL
	folderToken, err := utils.ValidateFolderURL(url)
	if err != nil {
		return err
	}
	// 移除冗余的令牌输出

	// 错误通道和等待组
	errChan := make(chan error)
	wg := sync.WaitGroup{}

	// 递归遍历文件夹并下载文档
	var processFolder func(ctx context.Context, folderPath, folderToken string) error
	processFolder = func(ctx context.Context, folderPath, folderToken string) error {
		files, err := client.GetDriveFolderFileList(ctx, nil, &folderToken)
		if err != nil {
			return err
		}
		localOpts := DownloadOpts{
			outputDir:     folderPath,
			dumpJSON:      opts.dumpJSON,
			skipDuplicate: opts.skipDuplicate,
			forceDownload: opts.forceDownload,
			spaceID:       opts.spaceID,
			nodeToken:     opts.nodeToken,
		}
		for _, file := range files {
			switch file.Type {
			case "folder":
				_folderPath := filepath.Join(folderPath, file.Name)
				if err := processFolder(ctx, _folderPath, file.Token); err != nil {
					return err
				}
			case "docx":
				// 并发下载文档
				wg.Add(1)
				go func(_url string) {
					if err := downloadDocument(ctx, client, _url, &localOpts); err != nil {
						errChan <- err
					}
					wg.Done()
				}(file.URL)
			}
		}
		return nil
	}
	if err := processFolder(ctx, opts.outputDir, folderToken); err != nil {
		return err
	}

	// Wait for all the downloads to finish
	go func() {
		wg.Wait()
		close(errChan)
	}()
	for err := range errChan {
		return err
	}
	return nil
}

// downloadWiki 下载知识库中的所有文档
func downloadWiki(ctx context.Context, client *core.Client, url string, opts *DownloadOpts) error {
	prefixURL, spaceID, err := utils.ValidateWikiURL(url)
	if err != nil {
		return err
	}

	folderPath, err := client.GetWikiName(ctx, spaceID)
	if err != nil {
		return err
	}
	if folderPath == "" {
		return fmt.Errorf("failed to GetWikiName")
	}

	errChan := make(chan error)

	var maxConcurrency = 10 // 设置最大并发级别
	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, maxConcurrency) // 创建具有最大并发级别的信号量

	var downloadWikiNode func(ctx context.Context,
		client *core.Client,
		spaceID string,
		parentPath string,
		parentNodeToken *string) error

	downloadWikiNode = func(ctx context.Context,
		client *core.Client,
		spaceID string,
		folderPath string,
		parentNodeToken *string) error {
		nodes, err := client.GetWikiNodeList(ctx, spaceID, parentNodeToken)
		if err != nil {
			return err
		}
		for _, n := range nodes {
			if n.HasChild {
				_folderPath := filepath.Join(folderPath, n.Title)
				if err := downloadWikiNode(ctx, client,
					spaceID, _folderPath, &n.NodeToken); err != nil {
					return err
				}
			}
			if n.ObjType == "docx" {
				wikiOpts := DownloadOpts{
					outputDir:     folderPath,
					dumpJSON:      opts.dumpJSON,
					skipDuplicate: opts.skipDuplicate,
					forceDownload: opts.forceDownload,
					spaceID:       spaceID,
					nodeToken:     n.NodeToken,
				}
				wg.Add(1)
				semaphore <- struct{}{}
				go func(_url string) {
					if err := downloadDocument(ctx, client, _url, &wikiOpts); err != nil {
						errChan <- err
					}
					wg.Done()
					<-semaphore
				}(prefixURL + "/wiki/" + n.NodeToken)
			}
		}
		return nil
	}

	if err = downloadWikiNode(ctx, client, spaceID, folderPath, nil); err != nil {
		return err
	}

	// Wait for all the downloads to finish
	go func() {
		wg.Wait()
		close(errChan)
	}()
	for err := range errChan {
		return err
	}
	return nil
}

// downloadWikiChildren 下载指定知识库文档下的所有子文档
func downloadWikiChildren(ctx context.Context, client *core.Client, url string, opts *DownloadOpts) error {
	startTime := time.Now()

	// 优先使用配置中的spaceID，然后使用环境变量
	spaceID := opts.spaceID
	if spaceID == "" {
		spaceID = os.Getenv("FEISHU_SPACE_ID")
	}
	var prefixURL string

	if spaceID == "" {
		// 尝试从URL解析spaceID（如果是知识库设置页面URL）
		var parsedSpaceID string
		var err error
		prefixURL, parsedSpaceID, err = utils.ValidateWikiURL(url)
		if err == nil {
			spaceID = parsedSpaceID
		}
	}

	if spaceID == "" {
		return fmt.Errorf("无法获取知识库spaceID。请通过以下方式提供:\n" +
			"  1. 环境变量: FEISHU_SPACE_ID (在 .env 文件中配置)\n" +
			"  2. 使用知识库设置页面URL\n\n" +
			"提示: 运行 'feishu2md init' 创建配置文件模板")
	}

	// 如果还没有获取URL前缀，则从URL中提取
	if prefixURL == "" {
		if urlParts := strings.Split(url, "/wiki/"); len(urlParts) >= 2 {
			prefixURL = urlParts[0]
		}
	}

	// 从URL中提取nodeToken
	docType, nodeToken, err := utils.ValidateDocumentURL(url)
	if err != nil {
		return err
	}

	// 如果是wiki类型，需要获取实际的文档信息
	if docType == "wiki" {
		node, err := client.GetWikiNodeInfo(ctx, nodeToken)
		if err != nil {
			return fmt.Errorf("GetWikiNodeInfo err: %v for %v", err, url)
		}
		nodeToken = node.NodeToken
	}

	fmt.Printf("🔍 正在获取子文档...\n")

	// 获取所有子节点
	allNodes, err := client.GetAllChildNodes(ctx, spaceID, nodeToken)
	if err != nil {
		return fmt.Errorf("获取子节点失败: %v", err)
	}

	if len(allNodes) == 0 {
		fmt.Println("📭 未找到任何子文档")
		return nil
	}

	fmt.Printf("📚 找到 %d 个子文档\n", len(allNodes))
	// 初始化统计器
	dlStats = &DownloadStats{}
	dlStats.SetTotalDocs(len(allNodes))

	// 创建目录结构映射：nodeToken -> 相对路径
	pathMap := make(map[string]string)

	// 首先为根节点建立路径
	pathMap[nodeToken] = "."

	// 递归构建路径映射
	var buildPaths func(parentToken, parentPath string)
	buildPaths = func(parentToken, parentPath string) {
		for _, node := range allNodes {
			if node.ParentToken == parentToken {
				// 构建当前节点的路径
				nodePath := filepath.Join(parentPath, utils.SanitizeFileName(node.Name))
				pathMap[node.NodeToken] = nodePath

				// 如果有子节点，递归处理
				if node.HasChild {
					buildPaths(node.NodeToken, nodePath)
				}
			}
		}
	}

	buildPaths(nodeToken, ".")

	// 并发下载控制
	// 提高并发度到20：限流器(100次/分钟+5次/秒)会自动控制API调用速率
	// 20个并发文档 × 平均3次API调用/文档 = 约60次并发API调用
	// 限流器会将其平滑到安全范围内
	var maxConcurrency = 20
	errChan := make(chan error, len(allNodes))
	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, maxConcurrency)

	// 下载所有文档类型的节点
	for _, node := range allNodes {
		if node.Type == "docx" {
			wg.Add(1)
			semaphore <- struct{}{}

			go func(n *core.Document) {
				defer func() {
					wg.Done()
					<-semaphore
				}()

				// 确定文档的输出目录
				nodePath := pathMap[n.ParentToken]
				if nodePath == "" {
					nodePath = "." // 默认到当前目录
				}

				fullOutputDir := filepath.Join(opts.outputDir, nodePath)

				// 创建输出目录
				if err := os.MkdirAll(fullOutputDir, 0o755); err != nil {
					errChan <- fmt.Errorf("创建目录失败 %s: %v", fullOutputDir, err)
					return
				}

				// 构建文档URL并下载
				docURL := prefixURL + "/wiki/" + n.NodeToken
				localOpts := DownloadOpts{
					outputDir:     fullOutputDir,
					dumpJSON:      opts.dumpJSON,
					skipDuplicate: opts.skipDuplicate,
					forceDownload: opts.forceDownload,
					spaceID:       spaceID,
					nodeToken:     n.NodeToken,
					relDir:        nodePath,
					tagMode:       opts.tagMode,
					categoryMode:  opts.categoryMode,
					tags:          deriveTagsFromPath(nodePath, opts.tagMode),
					categories:    deriveCategoriesFromPath(nodePath, opts.categoryMode),
				}

				// 移除冗余的下载路径输出
				if err := downloadDocument(ctx, client, docURL, &localOpts); err != nil {
					errChan <- fmt.Errorf("下载文档失败 %s: %v", n.Name, err)
				}
			}(node)
		}
	}

	// 等待所有下载完成
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// 检查是否有错误
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	// 计算总耗时
	elapsed := time.Since(startTime)

	// 统计汇总输出（整洁格式）
	fmt.Println()
	fmt.Println("📦 处理结果：")
	for _, l := range logCollector.SortedByPath() {
		status := "缓存"
		if l.DocNew {
			status = "新增"
		} else if l.Skipped {
			status = "跳过"
		}
		if l.Reason != "" {
			status += " (" + l.Reason + ")"
		}
		fmt.Printf("- %s  [%s]", l.Path, status)
		if l.ImgCache > 0 || l.ImgNew > 0 {
			fmt.Printf("  | 图片: +%d / 命中%d", l.ImgNew, l.ImgCache)
		}
		fmt.Println()
	}

	// 汇总
	totalDocs, docsNew, totalImages, imagesNew := dlStats.Snapshot()
	changes := docsNew + imagesNew
	if changes == 0 {
		fmt.Printf("🎉 完成！共 %d 个文档、%d 张图片，全部已缓存、无更新。耗时: %.2fs\n", totalDocs, totalImages, elapsed.Seconds())
	} else {
		fmt.Printf("🎉 完成！共 %d 个文档、%d 张图片，其中新增文档 %d、新增图片 %d，共 %d 处变更。耗时: %.2fs\n", totalDocs, totalImages, docsNew, imagesNew, changes, elapsed.Seconds())
	}
	return nil
}

// createCommonOpts 从CLI上下文创建通用的下载选项
func createCommonOpts(cliCtx *cli.Context) (*DownloadOpts, *core.Config, error) {
	// 加载配置文件（如果指定）
	configPath := cliCtx.String("config")
	if configPath != "" {
		if err := core.LoadEnvFileIfExists(configPath); err != nil {
			return nil, nil, fmt.Errorf("加载配置文件失败: %w", err)
		}
	}

	// 提取CLI标志
	spaceId := os.Getenv("FEISHU_SPACE_ID")
	titleAsFilename := cliCtx.Bool("title-name")
	useHTML := cliCtx.Bool("html")
	skipImages := cliCtx.Bool("no-img")
	skipDuplicate := cliCtx.Bool("skip-same")
	forceDownload := cliCtx.Bool("force")
	dumpJSON := cliCtx.Bool("json")
	tagMode := cliCtx.String("tag-mode")
	categoryMode := cliCtx.String("category-mode")

	// 加载配置
	config, err := core.LoadConfig("", "")
	if err != nil {
		return nil, nil, err
	}

	// 验证凭据
	if config.Feishu.AppId == "" || config.Feishu.AppSecret == "" {
		return nil, nil, cli.Exit("需要应用ID和应用密钥。请通过以下方式设置:\n"+
			"  1. 环境变量: FEISHU_APP_ID 和 FEISHU_APP_SECRET\n"+
			"  2. 配置文件: 使用 --config 指定配置文件路径\n"+
			"  3. 运行 'feishu2md init' 创建配置文件模板", 1)
	}

	// 使用CLI标志覆盖配置
	config.Output.TitleAsFilename = titleAsFilename
	config.Output.UseHTMLTags = useHTML
	config.Output.SkipImgDownload = skipImages

	// 创建下载选项
	opts := &DownloadOpts{
		outputDir:     config.Output.OutputDir,
		dumpJSON:      dumpJSON,
		skipDuplicate: skipDuplicate,
		forceDownload: forceDownload,
		spaceID:       spaceId,
		nodeToken:     "",
		tagMode:       tagMode,
		categoryMode:  categoryMode,
	}

	return opts, config, nil
}

// handleDocumentDownload 处理单个文档下载
func handleDocumentDownload(cliCtx *cli.Context, url string) error {
	opts, config, err := createCommonOpts(cliCtx)
	if err != nil {
		return err
	}

	dlConfig = *config
	client := core.NewClient(config.Feishu.AppId, config.Feishu.AppSecret)
	ctx := context.Background()

	return downloadDocument(ctx, client, url, opts)
}

// handleFolderDownload 处理文件夹批量下载
func handleFolderDownload(cliCtx *cli.Context, url string) error {
	opts, config, err := createCommonOpts(cliCtx)
	if err != nil {
		return err
	}

	dlConfig = *config
	client := core.NewClient(config.Feishu.AppId, config.Feishu.AppSecret)
	ctx := context.Background()

	return downloadDocuments(ctx, client, url, opts)
}

// handleWikiDownload 处理知识库完整下载
func handleWikiDownload(cliCtx *cli.Context, url string) error {
	opts, config, err := createCommonOpts(cliCtx)
	if err != nil {
		return err
	}

	dlConfig = *config
	client := core.NewClient(config.Feishu.AppId, config.Feishu.AppSecret)
	ctx := context.Background()

	return downloadWiki(ctx, client, url, opts)
}

// handleWikiTreeCommand 处理知识库子文档下载命令
func handleWikiTreeCommand(cliCtx *cli.Context) error {
	// 先加载配置文件
	configPath := cliCtx.String("config")
	if configPath != "" {
		if err := core.LoadEnvFileIfExists(configPath); err != nil {
			return fmt.Errorf("加载配置文件失败: %w", err)
		}
	}

	// 获取 URL：优先使用命令行参数，其次使用环境变量
	var url string
	if cliCtx.NArg() > 0 {
		url = cliCtx.Args().First()
	} else {
		url = os.Getenv("FEISHU_FOLDER_TOKEN")
	}

	if url == "" {
		return cli.Exit("错误: 请指定知识库文档URL\n\n"+
			"方式一: feishu2md wiki-tree <URL>\n"+
			"方式二: 在配置文件中设置 FEISHU_FOLDER_TOKEN\n\n"+
			"提示: 还需要在配置文件中设置 FEISHU_SPACE_ID", 1)
	}

	return handleWikiTreeDownload(cliCtx, url)
}

// handleWikiTreeDownload 处理知识库子文档下载
func handleWikiTreeDownload(cliCtx *cli.Context, url string) error {
	opts, config, err := createCommonOpts(cliCtx)
	if err != nil {
		return err
	}

	dlConfig = *config
	client := core.NewClient(config.Feishu.AppId, config.Feishu.AppSecret)
	ctx := context.Background()

	return downloadWikiChildren(ctx, client, url, opts)
}

// handleLegacyDownload 处理遗留的智能下载命令（保持向后兼容）
func handleLegacyDownload(cliCtx *cli.Context, url string) error {
	fmt.Println("⚠️  使用了已废弃的命令，建议使用具体的子命令:")
	fmt.Println("  - feishu2md document <url>  # 下载单个文档")
	fmt.Println("  - feishu2md folder <url>    # 下载文件夹")
	fmt.Println("  - feishu2md wiki <url>      # 下载知识库")
	fmt.Println("  - feishu2md wiki-tree <url> # 下载子文档")
	fmt.Println()

	// 自动检测URL类型并使用相应的处理函数
	if strings.Contains(url, "/drive/folder/") {
		return handleFolderDownload(cliCtx, url)
	}
	if strings.Contains(url, "/wiki/space/") {
		return handleWikiDownload(cliCtx, url)
	}
	if strings.Contains(url, "/wiki/") {
		// 需要检查是否有space来决定是wiki-tree还是单文档
		if cliCtx.String("space") != "" {
			return handleWikiTreeDownload(cliCtx, url)
		}
	}

	// 默认作为单文档处理
	return handleDocumentDownload(cliCtx, url)
}

// handleDownloadCommand 是遗留的主要处理程序（保持向后兼容）
func handleDownloadCommand(cliCtx *cli.Context, url string) error {
	return handleLegacyDownload(cliCtx, url)
}
