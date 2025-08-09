// Package main 包含将飞书文档转换为Markdown的下载功能
// 此文件处理核心下载操作，包括单个文档、批量文件夹和知识库
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/88250/lute"
	"github.com/Wsine/feishu2md/core"
	"github.com/Wsine/feishu2md/utils"
	"github.com/chyroc/lark"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

// DownloadOpts 包含下载操作的选项
type DownloadOpts struct {
	outputDir    string // 文件保存的目录
	dump         bool   // 是否转储API的JSON响应
	batch        bool   // 是否下载文件夹中的所有文档
	wiki         bool   // 是否下载知识库中的所有文档
	wikiChildren bool   // 是否下载指定知识库文档下的所有子文档
	spaceID      string // 知识库空间ID（用于检查子节点）
	nodeToken    string // 当前节点令牌（用于检查子节点）
}

// dlConfig 保存当前下载操作的配置
var dlConfig core.Config

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

	// 处理下载
	docx, blocks, err := client.GetDocxContent(ctx, docToken)
	utils.CheckErr(err)

	parser := core.NewParser(dlConfig.Output)

	title := docx.Title
	markdown := parser.ParseDocxContent(docx, blocks)

	if !dlConfig.Output.SkipImgDownload && len(parser.ImgTokens) > 0 {
		successCount := 0
		for _, imgToken := range parser.ImgTokens {
			localLink, err := client.DownloadImage(
				ctx, imgToken, filepath.Join(opts.outputDir, dlConfig.Output.ImageDir),
			)
			if err != nil {
				// 图片下载失败时不应该导致整个文档下载失败
				// 记录警告并继续处理其他图片
				fmt.Printf("⚠️  图片下载失败: %v\n", err)
				continue
			}
			markdown = strings.Replace(markdown, imgToken, localLink, 1)
			successCount++
		}
		if successCount > 0 {
			fmt.Printf("📸 下载了 %d 张图片\n", successCount)
		}
	}

	// Format the markdown document
	engine := lute.New(func(l *lute.Lute) {
		l.RenderOptions.AutoSpace = true
	})
	result := engine.FormatStr("md", markdown)

	// 处理输出目录和名称
	if _, err := os.Stat(opts.outputDir); os.IsNotExist(err) {
		if err := os.MkdirAll(opts.outputDir, 0o755); err != nil {
			return err
		}
	}

	if opts.dump {
		jsonName := fmt.Sprintf("%s.json", docToken)
		outputPath := filepath.Join(opts.outputDir, jsonName)
		data := struct {
			Document *lark.DocxDocument `json:"document"`
			Blocks   []*lark.DocxBlock  `json:"blocks"`
		}{
			Document: docx,
			Blocks:   blocks,
		}
		pdata := utils.PrettyPrint(data)

		if err = os.WriteFile(outputPath, []byte(pdata), 0o644); err != nil {
			return err
		}
		fmt.Printf("📄 JSON响应已转储到 %s\n", outputPath)
	}

	// 写入markdown文件
	mdName := fmt.Sprintf("%s.md", docToken)
	if dlConfig.Output.TitleAsFilename {
		mdName = fmt.Sprintf("%s.md", utils.SanitizeFileName(title))
	}
	outputPath := filepath.Join(opts.outputDir, mdName)
	if err = os.WriteFile(outputPath, []byte(result), 0o644); err != nil {
		return err
	}
	fmt.Printf("✅ %s\n", title)

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
			outputDir: folderPath,
			dump:      opts.dump,
			batch:     false,
			spaceID:   opts.spaceID,
			nodeToken: opts.nodeToken,
		}
		for _, file := range files {
			if file.Type == "folder" {
				_folderPath := filepath.Join(folderPath, file.Name)
				if err := processFolder(ctx, _folderPath, file.Token); err != nil {
					return err
				}
			} else if file.Type == "docx" {
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
					outputDir: folderPath,
					dump:      opts.dump,
					batch:     false,
					spaceID:   spaceID,
					nodeToken: n.NodeToken,
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
			"  1. 命令行参数: --space-id <id>\n" +
			"  2. 环境变量: FEISHU_SPACE_ID\n" +
			"  3. 使用知识库设置页面URL")
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
	var maxConcurrency = 10
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
					outputDir:    fullOutputDir,
					dump:         opts.dump,
					batch:        false,
					wiki:         false,
					wikiChildren: false,
					spaceID:      spaceID,
					nodeToken:    n.NodeToken,
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

	fmt.Printf("🎉 完成！成功下载了 %d 个文档\n", len(allNodes))
	return nil
}

// handleDownloadCommand 是下载操作的主要处理程序
// 它处理CLI标志、加载配置、验证凭据并启动适当的下载类型
func handleDownloadCommand(cliCtx *cli.Context, url string) error {
	// 提取下载配置的所有CLI标志
	appId := cliCtx.String("app-id")
	appSecret := cliCtx.String("app-secret")
	spaceId := cliCtx.String("space-id")
	outputDir := cliCtx.String("output")
	dump := cliCtx.Bool("dump")
	batch := cliCtx.Bool("batch")
	wiki := cliCtx.Bool("wiki")
	wikiChildren := cliCtx.Bool("wiki-children")
	titleAsFilename := cliCtx.Bool("title-as-filename")
	imageDir := cliCtx.String("image-dir")
	useHTMLTags := cliCtx.Bool("use-html-tags")
	skipImgDownload := cliCtx.Bool("skip-img-download")

	// 加载配置，优先级：CLI参数 > 环境变量 > 默认值
	config, err := core.LoadConfig(appId, appSecret)
	if err != nil {
		return err
	}

	// 验证凭据
	if config.Feishu.AppId == "" || config.Feishu.AppSecret == "" {
		return cli.Exit("需要应用ID和应用密钥。请通过以下方式设置:\n"+
			"  1. 命令行: --app-id <id> --app-secret <secret>\n"+
			"  2. 环境变量: FEISHU_APP_ID 和 FEISHU_APP_SECRET", 1)
	}

	// 使用CLI标志覆盖配置
	// titleAsFilename 现在默认为 true，可以被用户明确设置为 false
	config.Output.TitleAsFilename = titleAsFilename
	if useHTMLTags {
		config.Output.UseHTMLTags = true
	}
	if skipImgDownload {
		config.Output.SkipImgDownload = true
	}
	if imageDir != "img" { // 仅在用户提供非默认值时覆盖
		config.Output.ImageDir = imageDir
	}

	dlConfig = *config

	// 设置下载选项
	opts := DownloadOpts{
		outputDir:    outputDir,
		dump:         dump,
		batch:        batch,
		wiki:         wiki,
		wikiChildren: wikiChildren,
		spaceID:      spaceId, // 使用CLI参数或环境变量提供的spaceID
		nodeToken:    "",      // 单个文档下载时通常不需要nodeToken
	}

	// 实例化客户端
	client := core.NewClient(
		config.Feishu.AppId, config.Feishu.AppSecret,
	)
	ctx := context.Background()

	if batch {
		return downloadDocuments(ctx, client, url, &opts)
	}

	if wiki {
		return downloadWiki(ctx, client, url, &opts)
	}

	if wikiChildren {
		return downloadWikiChildren(ctx, client, url, &opts)
	}

	return downloadDocument(ctx, client, url, &opts)
}
