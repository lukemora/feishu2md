# feishu2md

🚀 **强大的飞书文档转 Markdown 工具** - 支持单文档、批量下载和知识库导出，智能处理图片并自动上传到图床。

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

---

## ✨ 核心特性

| 特性 | 说明 |
|------|------|
| 📄 **多种下载模式** | 单文档、文件夹批量、整个知识库、子文档递归下载 |
| 🖼️ **智能图片处理** | 自动下载图片，支持本地保存或上传图床 |
| ☁️ **PicGo 图床集成** | 通过 PicGo 支持 GitHub、阿里云、腾讯云等多种图床 |
| 🗜️ **图片压缩** | 支持 TinyPNG、ImageMin 等压缩方案（通过 PicGo 插件） |
| 🌳 **保持文档结构** | 递归下载时保持原有层级结构 |
| 🏷️ **层级元数据** | 自动从目录结构生成 tags 和 categories，支持灵活的层级选择 |
| ⚡ **高效并发** | 支持多线程并发下载，智能限流 |
| 📝 **友好文件名** | 默认使用文档标题，智能处理特殊字符 |
| 🎯 **格式完整** | 完整支持表格、列表、代码块等 Markdown 格式 |
| 💾 **智能缓存** | 图片和文档去重，避免重复下载和上传 |
| 🔧 **配置管理** | 环境变量配置，一键初始化配置文件 |

---

## 🚀 快速开始

### 1. 安装

```bash
# 克隆仓库
git clone https://github.com/Perfecto23/feishu2md.git
cd feishu2md

# 编译
make build

# 或使用 go build
go build -o feishu2md ./cmd/...
```

### 2. 初始化配置

```bash
# 创建配置文件
./feishu2md init

# 编辑配置文件
vim .env
```

配置文件示例：

```bash
# 飞书 API 认证（必需）
FEISHU_APP_ID=your_app_id
FEISHU_APP_SECRET=your_app_secret

# 知识库配置（wiki-tree 命令需要）
FEISHU_SPACE_ID=your_space_id
FEISHU_FOLDER_TOKEN=https://xxx.feishu.cn/wiki/your_node_token

# PicGo 图床配置（可选）
PICGO_ENABLED=true
```

### 3. 开始使用

```bash
# 下载单个文档
./feishu2md document https://xxx.feishu.cn/docx/abc123

# 批量下载文件夹
./feishu2md folder https://xxx.feishu.cn/drive/folder/abc123

# 下载整个知识库
./feishu2md wiki https://xxx.feishu.cn/wiki/space/abc123

# 下载知识库子文档（使用配置文件中的设置）
./feishu2md wiki-tree
```

---

## 📖 详细用法

### 命令概览

| 命令 | 别名 | 说明 |
|------|------|------|
| `init` | `i` | 创建配置文件模板 |
| `document` | `doc`, `d` | 下载单个文档 |
| `folder` | `f`, `batch` | 批量下载文件夹 |
| `wiki` | `w` | 下载整个知识库 |
| `wiki-tree` | `wt`, `children` | 下载子文档树 |

### 全局选项

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--config`, `-c` | 配置文件路径 | `.env` |
| `--title-name`, `-t` | 使用标题作为文件名 | `true` |
| `--skip-same`, `-s` | 跳过重复文件（MD5检查） | `true` |
| `--force`, `-f` | 强制下载 | `false` |
| `--no-img` | 跳过图片下载 | `false` |
| `--html` | 使用 HTML 而非 Markdown | `false` |
| `--json` | 导出 JSON 响应 | `false` |

### wiki-tree 专用选项

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--category-level` | 分类层级：正数从外向内(1=第一层)，负数从内向外(-1=最后一层) | `1` |
| `--no-body-title` | 禁用正文开头的 H1 标题（因为 frontmatter 已含 title） | `false` |

### 层级分类示例

`--category-level` 参数控制如何从文档路径生成 frontmatter 中的 categories。

**示例**：假设文档路径为 `技术/后端/Go语言/并发编程.md`

| 参数值 | categories |
|--------|------------|
| `--category-level=1` | `技术`（第1层） |
| `--category-level=2` | `后端`（第2层） |
| `--category-level=-1` | `Go语言`（最后一层） |
| `--category-level=-2` | `后端`（倒数第2层） |

**使用示例**：

```bash
# 默认：取第1层目录作为分类
./feishu2md wiki-tree

# 取最后一层目录作为分类
./feishu2md wiki-tree --category-level=-1

# 取倒数第2层目录作为分类，同时禁用正文 H1 标题
./feishu2md wiki-tree --category-level=-2 --no-body-title
```

**生成的 frontmatter 示例**：

```yaml
---
title: "并发编程"
date: 2025-01-01T12:00:00+08:00
updated: 2025-01-01T12:00:00+08:00
categories: Go语言
tags:
  - 技术
  - 后端
  - Go语言
id: xxxxx
---
```

---

## 🖼️ PicGo 图床功能

### 支持的图床平台

通过 PicGo CLI 支持多种图床：

- ✅ **GitHub** - 免费、稳定，推荐
- ✅ **SM.MS** - 免费图床
- ✅ **阿里云 OSS** - 国内访问快
- ✅ **腾讯云 COS** - 国内访问快
- ✅ **七牛云** - 国内 CDN 加速
- ✅ **又拍云** - 国内 CDN 加速
- ✅ **Imgur** - 国外免费图床
- ✅ 更多图床可通过 PicGo 插件扩展

### 配置图床

#### 1. 安装 PicGo CLI

```bash
# 需要 Node.js 环境
npm install picgo -g

# 验证安装
picgo -v
```

#### 2. 配置图床（以 GitHub 为例）

```bash
# 交互式配置
picgo set uploader github

# 根据提示填写：
# - repo: username/repo-name
# - branch: main
# - token: 你的 GitHub Personal Access Token
# - path: images/  (可选，图片存储路径)
# - customUrl: (可选，自定义域名)
```

#### 3. 安装压缩插件（可选）

```bash
# 安装压缩插件
picgo add compress

# 配置压缩选项
picgo config plugin compress
# 选择压缩方式：tinypng / imagemin / image2webp
```

#### 4. 启用 PicGo

在 `.env` 文件中设置：

```bash
PICGO_ENABLED=true
```

### 图床功能特性

- ✅ **智能缓存** - 基于 token 的本地缓存，避免重复上传
- ✅ **批量上传** - 10 并发上传提高效率
- ✅ **图片压缩** - 支持 TinyPNG、ImageMin 等压缩方案
- ✅ **链接替换** - 自动将 Markdown 中的图片链接替换为图床 URL
- ✅ **多图床支持** - 通过 PicGo 生态支持几乎所有主流图床

### 使用示例

```bash
# 启用图床下载文档
./feishu2md document https://xxx.feishu.cn/docx/abc123

# 输出示例（首次上传）
   ├─ 图片: 命中缓存 0, 新下载 6
✅ 文档标题

# 第二次运行（图片已缓存）
   ├─ 图片: 命中缓存 6, 新下载 0
⏭️  跳过重复文件: 文档标题
```

### 缓存说明

PicGo 上传成功后，会在当前工作目录的 `.feishu2md/upload-cache.json` 保存映射（便于跟随仓库提交）：

```json
{
  "boxcnXXXXXXX": "https://cdn.example.com/images/boxcnXXXXXXX.png"
}
```

清除缓存：删除该文件即可强制重新上传

---

## 📚 使用场景

### 场景 1: 下载单个文档

```bash
# 基础用法
./feishu2md document https://xxx.feishu.cn/docx/abc123

# 跳过图片下载
./feishu2md document https://xxx.feishu.cn/docx/abc123 --no-img

# 启用图床上传（需在 .env 中配置 PICGO_ENABLED=true）
./feishu2md document https://xxx.feishu.cn/docx/abc123
```

**输出结构**：
```
dist/
├── 文档标题.md
└── img/
    ├── image1.png
    └── image2.jpg
```

### 场景 2: 批量下载文件夹

```bash
./feishu2md folder https://xxx.feishu.cn/drive/folder/abc123
```

**输出结构**：
```
dist/
├── 子文件夹1/
│   ├── 文档1.md
│   └── img/
├── 子文件夹2/
│   ├── 文档2.md
│   └── img/
└── 文档3.md
```

### 场景 3: 下载知识库

```bash
# 下载整个知识库
./feishu2md wiki https://xxx.feishu.cn/wiki/space/abc123
```

### 场景 4: 下载知识库子文档树

这是最强大的功能，可以下载知识库中某个节点下的所有子文档。

**配置 .env**：
```bash
FEISHU_SPACE_ID=7474915720537620484
FEISHU_FOLDER_TOKEN=https://xxx.feishu.cn/wiki/MekRwTsI9izbqbk
```

**运行**：
```bash
# 使用配置文件中的设置
./feishu2md wiki-tree

# 或指定 URL（会覆盖配置文件）
./feishu2md wiki-tree https://xxx.feishu.cn/wiki/another_node

# 取倒数第2层目录作为分类
./feishu2md wiki-tree --category-level=-2

# 禁用正文 H1 标题（因为 frontmatter 已含 title）
./feishu2md wiki-tree --no-body-title

# 组合使用
./feishu2md wiki-tree --category-level=-2 --no-body-title
```

**特性**：
- ✅ 递归获取所有层级的子文档
- ✅ 自动创建文件夹层级结构
- ✅ 智能跳过有子文档的父级文档
- ✅ 并发下载（最大20个并发）
- ✅ 智能去重，避免重复下载
- ✅ 层级元数据生成（tags 取所有层级，categories 按 `--category-level` 指定）

**输出结构**：
```
dist/
├── 一级目录/
│   ├── 二级文档1.md
│   ├── 子目录/
│   │   ├── 三级文档1.md
│   │   └── img/
│   └── img/
└── 其他文档.md
```

---

## 🔧 飞书 API 配置

### 1. 创建飞书应用

1. 访问 [飞书开发者后台](https://open.feishu.cn/app)
2. 创建**企业自建应用**
3. 记录 **App ID** 和 **App Secret**

### 2. 开通 API 权限

在应用后台开通以下权限：

**必需权限**：
- ✅ `drive:drive:readonly` - 查看云空间文件
- ✅ `drive:file:read` - 读取文件内容  
- ✅ `drive:media:download` - **下载媒体文件（重要）**
- ✅ `wiki:wiki:readonly` - 查看知识库

### 3. 添加协作者权限

对于非公开文档，需要额外配置：

**方法一：知识库全局权限**
1. 为应用添加**机器人能力**并发布
2. 创建飞书群，将机器人添加到群中
3. 在知识库设置中，将该群添加为**管理员**

**方法二：单文档权限**
1. 为应用添加**云文档能力**并发布
2. 在文档的协作设置中，将应用添加为**协作者**

---

## ❓ 常见问题

<details>
<summary><b>Q: 如何获取知识库的 space_id？</b></summary>

A: 
1. 打开知识库
2. 点击右上角 **⚙️ 设置**
3. 查看浏览器地址栏：`https://xxx.feishu.cn/wiki/settings/7474915720537620484`
4. 最后的数字就是 space_id

</details>

<details>
<summary><b>Q: 图片下载失败显示 403 错误？</b></summary>

A: 按顺序检查：
1. 确认已开通 `drive:media:download` 权限
2. 检查应用是否为文档/知识库的协作者
3. 参考上方"添加协作者权限"部分

</details>

<details>
<summary><b>Q: 配置文件在哪里？</b></summary>

A: 默认使用当前目录的 `.env` 文件，也可以通过 `--config` 参数指定其他路径：

```bash
./feishu2md --config /path/to/custom.env document <url>
```

</details>

<details>
<summary><b>Q: 如何跳过图片下载？</b></summary>

A: 使用 `--no-img` 参数：

```bash
./feishu2md document <url> --no-img
```

</details>

<details>
<summary><b>Q: 支持哪些文档类型？</b></summary>

A: 仅支持飞书**新版文档 (docx)**，不支持旧版文档 (docs)

</details>

<details>
<summary><b>Q: 图床上传失败怎么办？</b></summary>

A: 检查以下步骤：
1. 确认 PicGo 已正确安装：`picgo -v`
2. 确认图床已配置：`picgo config uploader`
3. 手动测试上传：`picgo -d u /path/to/test.jpg`
4. 查看 PicGo 配置文件：`~/.picgo/config.json`
5. 确保 `.env` 中设置了 `PICGO_ENABLED=true`

</details>

<details>
<summary><b>Q: 如何清除 PicGo 上传缓存？</b></summary>

A: 删除缓存文件：

```bash
rm .feishu2md/upload-cache.json
```

</details>

---

## 🛠️ 开发

### 项目结构

```
feishu2md/
├── cmd/                # 命令行入口
│   ├── main.go        # 主程序
│   ├── download.go    # 下载逻辑
│   └── init.go        # 初始化命令
├── core/              # 核心功能
│   ├── client.go      # 飞书 API 客户端
│   ├── config.go      # 配置管理
│   ├── parser.go      # Markdown 解析器
│   ├── ratelimiter.go # API 限流器
│   └── envloader.go   # 环境变量加载
├── picgo/             # PicGo 图床模块
│   ├── picgo.go       # PicGo CLI 调用封装
│   └── cache.go       # 上传缓存管理
├── utils/             # 工具函数
│   ├── common.go
│   └── url.go
├── vendor/            # 依赖包（go mod vendor）
├── go.mod             # Go 模块定义
├── Makefile           # 构建脚本
└── CLAUDE.md          # AI 助手项目指南
```

### 环境准备

```bash
# 克隆仓库
git clone https://github.com/Perfecto23/feishu2md.git
cd feishu2md

# 确保 Go 1.21+ 已安装
go version

# 下载依赖
go mod download

# 同步 vendor 目录（可选）
go mod vendor
```

### 本地开发

```bash
# 直接运行（开发调试）
go run ./cmd document https://xxx.feishu.cn/docx/abc123

# 构建到 bin 目录
make build
# 或
go build -o bin/feishu2md ./cmd

# 运行构建产物
./bin/feishu2md document https://xxx.feishu.cn/docx/abc123
```

### 编译构建

```bash
# 开发构建（当前平台）
make build

# 跨平台构建（所有平台）
make build-all

# 单独构建指定平台
make build-darwin-arm64   # macOS ARM64 (M1/M2)
make build-darwin-amd64   # macOS Intel
make build-linux-amd64    # Linux x64
make build-windows-amd64  # Windows x64

# 手动跨平台编译
GOOS=linux GOARCH=amd64 go build -o feishu2md-linux ./cmd
GOOS=windows GOARCH=amd64 go build -o feishu2md.exe ./cmd
GOOS=darwin GOARCH=arm64 go build -o feishu2md-darwin-arm64 ./cmd
```

### 调试技巧

```bash
# 导出 JSON 响应用于调试 API 返回结构
./feishu2md document <url> --json

# 跳过图片下载（加速测试文档解析）
./feishu2md document <url> --no-img

# 强制重新下载（忽略缓存）
./feishu2md document <url> --force

# 检查 PicGo 是否可用
picgo -v

# 调试 PicGo 上传（显示详细日志）
picgo -d u /path/to/image.jpg
```

### 代码风格

```bash
# 格式化代码
make format
# 或
go fmt ./...

# 检查代码
go vet ./...
```

### 依赖管理

```bash
# 添加新依赖
go get github.com/xxx/yyy

# 清理未使用依赖
go mod tidy

# 同步 vendor 目录
go mod vendor
```

---

## 📄 开源协议

本项目基于 [MIT](LICENSE) 协议开源。

## 🙏 致谢

- [chyroc/lark](https://github.com/chyroc/lark) - 飞书 Go SDK
- [88250/lute](https://github.com/88250/lute) - Markdown 处理引擎
- [PicGo/PicGo-Core](https://github.com/PicGo/PicGo-Core) - 图床上传工具

---

## 🌟 贡献

欢迎提交 Issue 和 Pull Request！

---

<div align="center">

**如果觉得有用，请给个 ⭐ Star 支持一下！**

Made with ❤️ by [Perfecto23](https://github.com/Perfecto23)

</div>
