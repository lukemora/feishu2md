// Package main 为 feishu2md 工具提供命令行接口
// feishu2md 是一个用于下载飞书/LarkSuite 文档并转换为 Markdown 格式的工具
package main

import (
	"log"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
)

// version 是应用程序版本，通常在构建时设置
var version = "v2-test"

// main 是应用程序的入口点
// 它设置带有全局标志和命令的 CLI 应用程序
func main() {
	app := &cli.App{
		Name:    "feishu2md",
		Version: strings.TrimSpace(string(version)),
		Usage:   "下载飞书/LarkSuite文档并转换为Markdown文件",
		Description: "一个用于批量下载飞书/LarkSuite文档并转换为Markdown格式的命令行工具。\n" +
			"支持单个文档、文件夹批量下载、完整知识库下载以及知识库子文档下载。\n\n" +
			"使用示例:\n" +
			"  feishu2md document https://example.feishu.cn/docx/xxx\n" +
			"  feishu2md folder https://example.feishu.cn/drive/folder/xxx\n" +
			"  feishu2md wiki https://example.feishu.cn/wiki/space/xxx\n" +
			"  feishu2md wiki-tree https://example.feishu.cn/wiki/xxx",
		// 可与任何命令一起使用或作为独立选项的全局标志
		// 全局标志，适用于所有子命令
		Flags: []cli.Flag{
			// === 配置文件 ===
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "指定配置文件路径",
				Value:   ".env",
			},

			// === 文件选项 ===
			&cli.BoolFlag{
				Name:    "title-name",
				Aliases: []string{"t"},
				Usage:   "使用标题作为文件名",
				Value:   true,
			},
			&cli.BoolFlag{
				Name:    "skip-same",
				Aliases: []string{"s"},
				Usage:   "跳过相同文件 (MD5检查)",
				Value:   true,
			},
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "强制下载",
			},

			// === 内容选项 ===
			&cli.BoolFlag{
				Name:  "no-img",
				Usage: "跳过图片下载",
			},
			&cli.BoolFlag{
				Name:  "html",
				Usage: "使用HTML而非Markdown",
			},

			// === 调试选项 ===
			&cli.BoolFlag{
				Name:  "json",
				Usage: "导出JSON响应",
			},

			// === 元数据选项 ===
			&cli.StringFlag{
				Name:  "tag-mode",
				Usage: "标签生成模式: last(只取最后一层目录,默认) / all(取所有层级目录)",
				Value: "last",
			},
			&cli.StringFlag{
				Name:  "category-mode",
				Usage: "分类生成模式: last(只取最后一层目录,默认) / all(取所有层级目录)",
				Value: "last",
			},
		},
		ArgsUsage: "<url>",
		// 未指定子命令时的默认操作 - 作为下载处理
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() == 0 {
				cli.ShowAppHelp(ctx)
				return cli.Exit("\n错误: 请指定要下载的URL\n\n"+
					"使用示例:\n"+
					"  feishu2md document <文档URL>\n"+
					"  feishu2md folder <文件夹URL>\n"+
					"  feishu2md wiki <知识库URL>\n\n"+
					"运行 'feishu2md help' 查看完整帮助信息", 1)
			}
			url := ctx.Args().First()
			return handleDownloadCommand(ctx, url)
		},
		Commands: []*cli.Command{
			// 初始化配置文件
			{
				Name:    "init",
				Aliases: []string{"i"},
				Usage:   "创建环境变量配置文件",
				Description: "在当前目录创建 .env 示例文件，包含所有配置项说明。\n\n" +
					"配置项包括:\n" +
					"  - 飞书API认证信息\n" +
					"  - 图床配置（阿里云OSS/腾讯云COS）\n\n" +
					"示例:\n" +
					"  feishu2md init\n" +
					"  feishu2md init --force  # 强制覆盖已存在的文件",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
						Usage:   "强制覆盖已存在的配置文件",
					},
				},
				Action: handleInitCommand,
			},

			// 单个文档下载
			{
				Name:      "document",
				Aliases:   []string{"doc", "d"},
				Usage:     "下载单个飞书文档",
				ArgsUsage: "<文档URL>",
				Description: "下载指定的飞书/LarkSuite文档并转换为Markdown文件。\n\n" +
					"支持的URL格式:\n" +
					"  - https://example.feishu.cn/docx/xxx\n" +
					"  - https://example.feishu.cn/wiki/xxx (单个知识库文档)\n\n" +
					"示例:\n" +
					"  feishu2md document https://example.feishu.cn/docx/abc123\n" +
					"  feishu2md doc https://example.feishu.cn/wiki/def456 --no-img",
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() == 0 {
						return cli.Exit("错误: 请指定文档URL\n\n示例: feishu2md document https://example.feishu.cn/docx/xxx", 1)
					}
					url := ctx.Args().First()
					return handleDocumentDownload(ctx, url)
				},
			},

			// 文件夹批量下载
			{
				Name:      "folder",
				Aliases:   []string{"f", "batch"},
				Usage:     "批量下载文件夹中的所有文档",
				ArgsUsage: "<文件夹URL>",
				Description: "递归下载指定文件夹中的所有文档，保持原有目录结构。\n\n" +
					"支持的URL格式:\n" +
					"  - https://example.feishu.cn/drive/folder/xxx\n\n" +
					"特性:\n" +
					"  - 递归遍历子文件夹\n" +
					"  - 并发下载提升效率\n" +
					"  - 自动跳过非文档文件\n\n" +
					"示例:\n" +
					"  feishu2md folder https://example.feishu.cn/drive/folder/abc123\n" +
					"  feishu2md f https://example.feishu.cn/drive/folder/abc123 --force",
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() == 0 {
						return cli.Exit("错误: 请指定文件夹URL\n\n示例: feishu2md folder https://example.feishu.cn/drive/folder/xxx", 1)
					}
					url := ctx.Args().First()
					return handleFolderDownload(ctx, url)
				},
			},

			// 知识库完整下载
			{
				Name:      "wiki",
				Aliases:   []string{"w"},
				Usage:     "下载整个知识库",
				ArgsUsage: "<知识库URL>",
				Description: "下载知识库中的所有文档，保持原有层级结构。\n\n" +
					"支持的URL格式:\n" +
					"  - https://example.feishu.cn/wiki/space/xxx\n\n" +
					"特性:\n" +
					"  - 完整下载知识库所有内容\n" +
					"  - 保持原有目录结构\n" +
					"  - 智能处理层级关系\n" +
					"  - 高效并发下载\n\n" +
					"示例:\n" +
					"  feishu2md wiki https://example.feishu.cn/wiki/space/abc123\n" +
					"  feishu2md w https://example.feishu.cn/wiki/space/abc123 --skip-same",
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() == 0 {
						return cli.Exit("错误: 请指定知识库URL\n\n示例: feishu2md wiki https://example.feishu.cn/wiki/space/xxx", 1)
					}
					url := ctx.Args().First()
					return handleWikiDownload(ctx, url)
				},
			},

			// 知识库子文档下载
			{
				Name:      "wiki-tree",
				Aliases:   []string{"wt", "children"},
				Usage:     "下载知识库文档的所有子文档",
				ArgsUsage: "[知识库文档URL]",
				Description: "下载指定知识库文档下的所有子文档，保持层级结构。\n\n" +
					"要求:\n" +
					"  需要在配置文件中设置 FEISHU_SPACE_ID\n" +
					"  文档 URL 可以通过命令行参数或 FEISHU_FOLDER_TOKEN 环境变量提供\n\n" +
					"支持的URL格式:\n" +
					"  - https://example.feishu.cn/wiki/xxx (知识库文档)\n\n" +
					"特性:\n" +
					"  - 递归下载所有子文档\n" +
					"  - 保持原有层级结构\n" +
					"  - 智能跳过有子节点的文档\n" +
					"  - 支持并发下载\n\n" +
					"示例:\n" +
					"  feishu2md wiki-tree https://example.feishu.cn/wiki/abc123\n" +
					"  feishu2md wiki-tree  # 使用 .env 中配置的 FEISHU_FOLDER_TOKEN",
				Action: handleWikiTreeCommand,
			},

			// 兼容性命令 - 保持向后兼容
			{
				Name:      "download",
				Aliases:   []string{"dl"},
				Usage:     "智能下载 (已废弃，建议使用具体的子命令)",
				ArgsUsage: "<URL>",
				Hidden:    true,
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() == 0 {
						return cli.Exit("请指定URL", 1)
					}
					url := ctx.Args().First()
					return handleLegacyDownload(ctx, url)
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
