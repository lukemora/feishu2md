// Package main - 初始化配置文件功能
package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

// envTemplate 环境变量配置文件模板
const envTemplate = `# ====================================
# 飞书文档导出工具 - 环境变量配置
# ====================================

# ----------------------------------
# 飞书 API 认证配置（必需）
# ----------------------------------
# 获取方式：https://open.feishu.cn/app
FEISHU_APP_ID=your_app_id_here
FEISHU_APP_SECRET=your_app_secret_here

# ----------------------------------
# 知识库配置（可选）
# ----------------------------------
# 用于 wiki-tree 命令下载知识库子文档

# 知识库空间 ID（必需）
# 从知识库设置页面获取: https://xxx.feishu.cn/wiki/settings/{space_id}
# FEISHU_SPACE_ID=your_space_id_here

# 要下载的文档节点 URL（可选）
# 如果配置了此项，运行 wiki-tree 命令时可以不提供 URL 参数
# FEISHU_FOLDER_TOKEN=https://xxx.feishu.cn/wiki/your_node_token

# ----------------------------------
# 输出配置（可选）
# ----------------------------------
# 文档输出目录
# 默认: ./dist
# OUTPUT_DIR=./dist

# 图片目录（相对于输出目录）
# 默认: img
# IMAGE_DIR=img


# ====================================
# 图床配置（可选）
# ====================================
# 启用后，下载的图片会自动上传到图床
# 并将 Markdown 中的图片链接替换为图床 URL

# ----------------------------------
# 图床开关
# ----------------------------------
# 是否启用图床上传功能
# 值: true/false 或 1/0
IMGBED_ENABLED=false

# ----------------------------------
# 图床平台选择
# ----------------------------------
# 支持的平台: oss (阿里云) / cos (腾讯云)
IMGBED_PLATFORM=oss


# ==== 阿里云 OSS 配置 ====
# 使用阿里云 OSS 时填写以下配置

# 访问密钥 ID (AccessKey ID)
IMGBED_SECRET_ID=your_aliyun_access_key_id

# 访问密钥 (AccessKey Secret)
IMGBED_SECRET_KEY=your_aliyun_access_key_secret

# 存储桶名称
IMGBED_BUCKET=your-bucket-name

# 存储区域
# 可选值: oss-cn-hangzhou, oss-cn-beijing, oss-cn-shanghai, oss-cn-shenzhen 等
# 完整列表: https://help.aliyun.com/document_detail/31837.html
IMGBED_REGION=oss-cn-hangzhou

# 自定义域名（可选）
# 如果配置了 CDN 加速域名，填写此项
# 例如: cdn.example.com
# FEISHU_IMGBED_HOST=

# 上传路径前缀（可选）
# 图片上传到 OSS 的路径前缀，例如: images/
# IMGBED_PREFIX_KEY=images/


# ==== 腾讯云 COS 配置 ====
# 使用腾讯云 COS 时填写以下配置（与阿里云配置共用变量名）

# 访问密钥 ID (SecretId)
# IMGBED_SECRET_ID=your_tencent_secret_id

# 访问密钥 (SecretKey)
# IMGBED_SECRET_KEY=your_tencent_secret_key

# 存储桶名称
# 格式: bucket-appid，例如: my-bucket-1234567890
# IMGBED_BUCKET=your-bucket-appid

# 存储区域
# 可选值: ap-guangzhou, ap-beijing, ap-shanghai, ap-chengdu 等
# 完整列表: https://cloud.tencent.com/document/product/436/6224
# IMGBED_REGION=ap-guangzhou

# 自定义域名（可选）
# 如果配置了 CDN 加速域名，填写此项
# FEISHU_IMGBED_HOST=

# 上传路径前缀（可选）
# IMGBED_PREFIX_KEY=images/


# ----------------------------------
# 使用说明
# ----------------------------------
# 1. 填写上述配置项的值（至少需要填写 FEISHU_APP_ID 和 FEISHU_APP_SECRET）
# 2. 使用配置文件运行:
#    feishu2md document <url> --config .env
#    或者默认会自动加载当前目录的 .env 文件:
#    feishu2md document <url>
# 3. 也可以手动加载环境变量:
#    source .env  (Linux/macOS)
#
# 注意: .env 文件包含敏感信息，请勿提交到 Git 仓库
#       本项目的 .gitignore 已默认忽略 .env 文件
`

// handleInitCommand 处理 init 命令
func handleInitCommand(ctx *cli.Context) error {
	force := ctx.Bool("force")
	filename := ".env"

	// 检查文件是否已存在
	if !force {
		if _, err := os.Stat(filename); err == nil {
			return cli.Exit(fmt.Sprintf("❌ 文件 %s 已存在\n"+
				"使用 --force 参数强制覆盖，或手动删除后重试", filename), 1)
		}
	}

	// 写入配置文件
	if err := os.WriteFile(filename, []byte(envTemplate), 0644); err != nil {
		return cli.Exit(fmt.Sprintf("❌ 创建配置文件失败: %v", err), 1)
	}

	// 成功提示
	fmt.Println("✅ 配置文件已创建: " + filename)
	fmt.Println()
	fmt.Println("📝 后续步骤:")
	fmt.Println("  1. 编辑配置文件: vim .env  # 或使用你喜欢的编辑器")
	fmt.Println("  2. 填写必需的配置项（至少需要 FEISHU_APP_ID 和 FEISHU_APP_SECRET）")
	fmt.Println("  3. 开始使用: feishu2md document <url>")
	fmt.Println()
	fmt.Println("💡 提示:")
	fmt.Println("  - 工具会自动加载当前目录的 .env 文件")
	fmt.Println("  - 也可使用 --config 指定其他配置文件: feishu2md --config my.env document <url>")
	fmt.Println("  - 图床功能为可选，不需要可保持 IMGBED_ENABLED=false")
	fmt.Println("  - .env 文件已在 .gitignore 中，不会被提交到版本控制")

	return nil
}
