package core

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chyroc/lark"
	"github.com/chyroc/lark_rate_limiter"
)

type Client struct {
	larkClient *lark.Lark
}

func NewClient(appID, appSecret string) *Client {
	return &Client{
		larkClient: lark.New(
			lark.WithAppCredential(appID, appSecret),
			lark.WithTimeout(60*time.Second),
			lark.WithApiMiddleware(lark_rate_limiter.Wait(4, 4)),
		),
	}
}

func (c *Client) DownloadImage(ctx context.Context, imgToken, outDir string) (string, error) {
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

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
	if err != nil {
		return imgToken, fmt.Errorf("创建文件失败: %v", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.File)
	if err != nil {
		return imgToken, fmt.Errorf("写入文件失败: %v", err)
	}

	// 返回相对路径，用于markdown引用
	relativePath := fmt.Sprintf("./%s/%s%s", filepath.Base(outDir), imgToken, fileext)
	return relativePath, nil
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

func (c *Client) GetDocxContent(ctx context.Context, docToken string) (*lark.DocxDocument, []*lark.DocxBlock, error) {
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

func (c *Client) GetWikiNodeInfo(ctx context.Context, token string) (*lark.GetWikiNodeRespNode, error) {
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
