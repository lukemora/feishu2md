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
		// 可与任何命令一起使用或作为独立选项的全局标志
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "app-id",
				Usage:   "飞书应用ID (也可使用 FEISHU_APP_ID 环境变量)",
				EnvVars: []string{"FEISHU_APP_ID"},
			},
			&cli.StringFlag{
				Name:    "app-secret",
				Usage:   "飞书应用密钥 (也可使用 FEISHU_APP_SECRET 环境变量)",
				EnvVars: []string{"FEISHU_APP_SECRET"},
			},
			&cli.StringFlag{
				Name:    "space-id",
				Usage:   "知识库空间ID (也可使用 FEISHU_SPACE_ID 环境变量)",
				EnvVars: []string{"FEISHU_SPACE_ID"},
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Value:   "./",
				Usage:   "Markdown文件的输出目录",
			},
			&cli.BoolFlag{
				Name:  "dump",
				Usage: "导出API的JSON响应",
			},
			&cli.BoolFlag{
				Name:  "batch",
				Usage: "下载文件夹下的所有文档",
			},
			&cli.BoolFlag{
				Name:  "wiki",
				Usage: "下载知识库中的所有文档",
			},
			&cli.BoolFlag{
				Name:  "title-as-filename",
				Usage: "使用文档标题作为输出的Markdown文件名 (默认: true)",
				Value: true,
			},
			&cli.StringFlag{
				Name:  "image-dir",
				Usage: "存储下载图片的目录 (相对于输出目录)",
				Value: "img",
			},
			&cli.BoolFlag{
				Name:  "use-html-tags",
				Usage: "使用HTML标签而不是Markdown来渲染某些样式",
			},
			&cli.BoolFlag{
				Name:  "skip-img-download",
				Usage: "跳过下载图片并保留原始链接",
			},
			&cli.BoolFlag{
				Name:  "wiki-children",
				Usage: "下载指定知识库文档下的所有子文档（需要设置环境变量 FEISHU_SPACE_ID）",
			},
		},
		ArgsUsage: "<url>",
		// 未指定子命令时的默认操作 - 作为下载处理
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() == 0 {
				cli.ShowAppHelp(ctx)
				return cli.Exit("请指定文档/文件夹/知识库的URL", 1)
			}
			url := ctx.Args().First()
			return handleDownloadCommand(ctx, url)
		},
		Commands: []*cli.Command{
			{
				Name:      "download",
				Aliases:   []string{"dl"},
				Usage:     "下载飞书/LarkSuite文档并转换为Markdown文件",
				ArgsUsage: "<url>",
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() == 0 {
						return cli.Exit("请指定文档/文件夹/知识库的URL", 1)
					}
					url := ctx.Args().First()
					return handleDownloadCommand(ctx, url)
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
