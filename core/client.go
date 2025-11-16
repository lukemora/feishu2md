package core

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chyroc/lark"
)

type Client struct {
	larkClient *lark.Lark
	limiter    *FeishuRateLimiter // 飞书API限流器
}

func NewClient(appID, appSecret string) *Client {
	return &Client{
		larkClient: lark.New(
			lark.WithAppCredential(appID, appSecret),
			lark.WithTimeout(60*time.Second),
			// 移除SDK自带限流，使用我们的精确控制
		),
		limiter: NewFeishuRateLimiter(), // 100次/分钟, 5次/秒
	}
}

func (c *Client) DownloadImage(ctx context.Context, imgToken, outDir string) (string, error) {
	// 如果本地已经存在以 imgToken 命名的图片文件（任意扩展名），则直接复用，跳过网络下载
	if existingPath, ok := findExistingLocalImage(outDir, imgToken); ok {
		relativePath := fmt.Sprintf("./%s/%s", filepath.Base(outDir), filepath.Base(existingPath))
		return relativePath, nil
	}

	// 限流: 等待飞书API调用许可
	if err := c.limiter.Wait(ctx); err != nil {
		return imgToken, fmt.Errorf("限流等待失败: %v", err)
	}

	resp, _, err := c.larkClient.Drive.DownloadDriveMedia(ctx, &lark.DownloadDriveMediaReq{
		FileToken: imgToken,
	})
	if err != nil {
		// 提供更详细的错误信息，帮助诊断权限问题
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "Forbidden") {
			return imgToken, fmt.Errorf("图片下载权限不足 (403 Forbidden): 请检查飞书应用是否有 drive:media:download 权限")
		}
		return imgToken, fmt.Errorf("图片下载失败: %v", err)
	}

	// 获取文件扩展名，如果没有则使用默认的
	fileext := filepath.Ext(resp.Filename)
	if fileext == "" {
		fileext = ".png" // 默认扩展名
	}

	// 确保输出目录存在
	err = os.MkdirAll(outDir, 0o755)
	if err != nil {
		return imgToken, fmt.Errorf("创建目录失败: %v", err)
	}

	// 构建完整的文件路径
	filename := filepath.Join(outDir, fmt.Sprintf("%s%s", imgToken, fileext))

	// 先将远端文件读入内存，便于按类型进行无损压缩处理（目前仅对 PNG 应用）
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.File); err != nil {
		return imgToken, fmt.Errorf("读取远端文件失败: %v", err)
	}

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
	if err != nil {
		return imgToken, fmt.Errorf("创建文件失败: %v", err)
	}
	defer file.Close()

	// 对 PNG 进行无损压缩（BestCompression）。若解码/编码失败则回退为原始字节写入。
	if strings.EqualFold(fileext, ".png") {
		if img, err := png.Decode(bytes.NewReader(buf.Bytes())); err == nil {
			enc := png.Encoder{CompressionLevel: png.BestCompression}
			if err := enc.Encode(file, img); err == nil {
				// 已完成优化写入
			} else {
				// 编码失败，回退原始字节
				if _, werr := file.Write(buf.Bytes()); werr != nil {
					return imgToken, fmt.Errorf("写入文件失败: %v", werr)
				}
			}
		} else {
			// 解码失败，回退原始字节
			if _, werr := file.Write(buf.Bytes()); werr != nil {
				return imgToken, fmt.Errorf("写入文件失败: %v", werr)
			}
		}
	} else {
		// 其他类型暂不处理，直接原样写入
		if _, werr := file.Write(buf.Bytes()); werr != nil {
			return imgToken, fmt.Errorf("写入文件失败: %v", werr)
		}
	}

	// 返回相对路径，用于markdown引用
	relativePath := fmt.Sprintf("./%s/%s%s", filepath.Base(outDir), imgToken, fileext)
	return relativePath, nil
}

// findExistingLocalImage 在 outDir 内查找以 imgToken 命名、任意扩展名的已存在图片文件
// 命中则返回绝对路径与 true，否则返回空字符串与 false
func findExistingLocalImage(outDir, imgToken string) (string, bool) {
	// 模式如: /abs/outDir/<imgToken>.*
	pattern := filepath.Join(outDir, imgToken+".*")
	matches, _ := filepath.Glob(pattern)
	if len(matches) == 0 {
		return "", false
	}
	// 取第一个匹配项（通常只会存在一个）
	return matches[0], true
}

func (c *Client) DownloadImageRaw(ctx context.Context, imgToken, imgDir string) (string, []byte, error) {
	resp, _, err := c.larkClient.Drive.DownloadDriveMedia(ctx, &lark.DownloadDriveMediaReq{
		FileToken: imgToken,
	})
	if err != nil {
		return imgToken, nil, err
	}
	fileext := filepath.Ext(resp.Filename)
	filename := fmt.Sprintf("%s/%s%s", imgDir, imgToken, fileext)
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.File)
	return filename, buf.Bytes(), nil
}

// GetDocxDocumentMeta 仅获取文档的基本信息（不拉取块列表），用于快速判断修订版本
func (c *Client) GetDocxDocumentMeta(ctx context.Context, docToken string) (*lark.DocxDocument, error) {
	// 限流: 等待飞书API调用许可
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("限流等待失败: %v", err)
	}

	resp, _, err := c.larkClient.Drive.GetDocxDocument(ctx, &lark.GetDocxDocumentReq{
		DocumentID: docToken,
	})
	if err != nil {
		return nil, err
	}
	docx := &lark.DocxDocument{
		DocumentID: resp.Document.DocumentID,
		RevisionID: resp.Document.RevisionID,
		Title:      resp.Document.Title,
	}
	return docx, nil
}

func (c *Client) GetDocxContent(ctx context.Context, docToken string) (*lark.DocxDocument, []*lark.DocxBlock, error) {
	// 限流: 等待飞书API调用许可
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, nil, fmt.Errorf("限流等待失败: %v", err)
	}

	resp, _, err := c.larkClient.Drive.GetDocxDocument(ctx, &lark.GetDocxDocumentReq{
		DocumentID: docToken,
	})
	if err != nil {
		return nil, nil, err
	}
	docx := &lark.DocxDocument{
		DocumentID: resp.Document.DocumentID,
		RevisionID: resp.Document.RevisionID,
		Title:      resp.Document.Title,
	}
	var blocks []*lark.DocxBlock
	var pageToken *string
	for {
		// 每次分页调用都需要限流
		if err := c.limiter.Wait(ctx); err != nil {
			return docx, nil, fmt.Errorf("限流等待失败: %v", err)
		}

		resp2, _, err := c.larkClient.Drive.GetDocxBlockListOfDocument(ctx, &lark.GetDocxBlockListOfDocumentReq{
			DocumentID: docx.DocumentID,
			PageToken:  pageToken,
		})
		if err != nil {
			return docx, nil, err
		}
		blocks = append(blocks, resp2.Items...)
		pageToken = &resp2.PageToken
		if !resp2.HasMore {
			break
		}
	}
	return docx, blocks, nil
}

// GetDocxTimes 获取 docx 文档的创建时间与最近修改时间
// 返回值为指针，若对应字段不可用则为 nil
func (c *Client) GetDocxTimes(ctx context.Context, docToken string) (createdAt *time.Time, updatedAt *time.Time, err error) {
	resp, _, err := c.larkClient.Drive.GetDriveFileMeta(ctx, &lark.GetDriveFileMetaReq{
		RequestDocs: []*lark.GetDriveFileMetaReqRequestDocs{
			{DocToken: docToken, DocType: "docx"},
		},
	})
	if err != nil {
		return nil, nil, err
	}
	if resp == nil || len(resp.Metas) == 0 || resp.Metas[0] == nil {
		return nil, nil, fmt.Errorf("未获取到文档元数据")
	}
	meta := resp.Metas[0]

	parseUnixString := func(s string) (*time.Time, error) {
		if strings.TrimSpace(s) == "" {
			return nil, nil
		}
		// 兼容秒/毫秒时间戳
		v, perr := strconv.ParseInt(s, 10, 64)
		if perr != nil {
			return nil, perr
		}
		var t time.Time
		if v > 1_000_000_000_000 { // 毫秒
			t = time.Unix(0, v*int64(time.Millisecond))
		} else { // 秒
			t = time.Unix(v, 0)
		}
		return &t, nil
	}

	ctime, _ := parseUnixString(meta.CreateTime)
	mtime, _ := parseUnixString(meta.LatestModifyTime)
	return ctime, mtime, nil
}

func (c *Client) GetWikiNodeInfo(ctx context.Context, token string) (*lark.GetWikiNodeRespNode, error) {
	// 限流: 等待飞书API调用许可
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("限流等待失败: %v", err)
	}

	resp, _, err := c.larkClient.Drive.GetWikiNode(ctx, &lark.GetWikiNodeReq{
		Token: token,
	})
	if err != nil {
		return nil, err
	}
	return resp.Node, nil
}

func (c *Client) GetDriveFolderFileList(ctx context.Context, pageToken *string, folderToken *string) ([]*lark.GetDriveFileListRespFile, error) {
	resp, _, err := c.larkClient.Drive.GetDriveFileList(ctx, &lark.GetDriveFileListReq{
		PageSize:    nil,
		PageToken:   pageToken,
		FolderToken: folderToken,
	})
	if err != nil {
		return nil, err
	}
	files := resp.Files
	for resp.HasMore {
		resp, _, err = c.larkClient.Drive.GetDriveFileList(ctx, &lark.GetDriveFileListReq{
			PageSize:    nil,
			PageToken:   &resp.NextPageToken,
			FolderToken: folderToken,
		})
		if err != nil {
			return nil, err
		}
		files = append(files, resp.Files...)
	}
	return files, nil
}

func (c *Client) GetWikiName(ctx context.Context, spaceID string) (string, error) {
	resp, _, err := c.larkClient.Drive.GetWikiSpace(ctx, &lark.GetWikiSpaceReq{
		SpaceID: spaceID,
	})

	if err != nil {
		return "", err
	}

	return resp.Space.Name, nil
}

func (c *Client) GetWikiNodeList(ctx context.Context, spaceID string, parentNodeToken *string) ([]*lark.GetWikiNodeListRespItem, error) {
	// 限流: 等待飞书API调用许可
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("限流等待失败: %v", err)
	}

	resp, _, err := c.larkClient.Drive.GetWikiNodeList(ctx, &lark.GetWikiNodeListReq{
		SpaceID:         spaceID,
		PageSize:        nil,
		PageToken:       nil,
		ParentNodeToken: parentNodeToken,
	})

	if err != nil {
		return nil, err
	}

	nodes := resp.Items
	previousPageToken := ""

	for resp.HasMore && previousPageToken != resp.PageToken {
		previousPageToken = resp.PageToken
		resp, _, err := c.larkClient.Drive.GetWikiNodeList(ctx, &lark.GetWikiNodeListReq{
			SpaceID:         spaceID,
			PageSize:        nil,
			PageToken:       &resp.PageToken,
			ParentNodeToken: parentNodeToken,
		})

		if err != nil {
			return nil, err
		}

		nodes = append(nodes, resp.Items...)
	}

	return nodes, nil
}

// Document 表示知识库文档节点信息
type Document struct {
	Token       string // 文档令牌
	NodeToken   string // 节点令牌
	Name        string // 文档名称
	Type        string // 文档类型
	ParentToken string // 父节点令牌
	HasChild    bool   // 是否有子节点
}

// GetChildNodes 获取指定父节点下的所有直接子节点
func (c *Client) GetChildNodes(ctx context.Context, spaceID, parentNodeToken string) ([]*Document, error) {
	var allNodes []*Document
	pageToken := ""

	for {
		// 按飞书 API 限制，page_size 取值范围为 [1-50]
		pageSize := int64(50)
		req := &lark.GetWikiNodeListReq{
			SpaceID:         spaceID,
			PageSize:        &pageSize,
			ParentNodeToken: &parentNodeToken,
		}
		if pageToken != "" {
			req.PageToken = &pageToken
		}

		resp, _, err := c.larkClient.Drive.GetWikiNodeList(ctx, req)
		if err != nil {
			return nil, err
		}

		// 处理返回的节点数据
		for _, item := range resp.Items {
			doc := &Document{
				Token:       item.ObjToken,
				NodeToken:   item.NodeToken,
				Name:        item.Title,
				Type:        item.ObjType,
				ParentToken: item.ParentNodeToken,
				HasChild:    item.HasChild,
			}
			allNodes = append(allNodes, doc)
		}

		// 检查是否有下一页
		if !resp.HasMore || resp.PageToken == "" {
			break
		}
		pageToken = resp.PageToken
	}

	return allNodes, nil
}

// GetAllChildNodes 递归获取指定父节点下的所有子节点（包括子节点的子节点）
func (c *Client) GetAllChildNodes(ctx context.Context, spaceID, rootNodeToken string) ([]*Document, error) {
	var result []*Document

	var processNode func(nodeToken string) error
	processNode = func(nodeToken string) error {
		nodes, err := c.GetChildNodes(ctx, spaceID, nodeToken)
		if err != nil {
			return err
		}

		for _, node := range nodes {
			result = append(result, node)

			// 如果有子节点，递归处理
			if node.HasChild {
				if err := processNode(node.NodeToken); err != nil {
					return err
				}
			}
		}
		return nil
	}

	return result, processNode(rootNodeToken)
}
