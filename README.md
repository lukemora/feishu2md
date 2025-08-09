# feishu2md

🚀 **强大的飞书文档转 Markdown 工具** - 支持单文档、批量下载和知识库子文档下载，智能处理图片和文档结构。

---

## ✨ 核心特性

| 特性 | 说明 |
|------|------|
| 📄 **多种下载模式** | 单文档、文件夹批量、整个知识库、子文档递归下载 |
| 🖼️ **智能图片处理** | 自动下载图片到本地，生成相对路径引用 `./img/` |
| 🌳 **保持文档结构** | 递归下载时保持原有层级结构，智能跳过父级文档 |
| ⚡ **高效并发** | 支持多线程并发下载，最大10个并发 |
| 📝 **友好文件名** | 默认使用文档标题，智能处理特殊字符（如 `JavaScript/TypeScript` → `JavaScript-TypeScript`） |
| 🎯 **格式保持** | 完整支持表格、列表、代码块等 Markdown 格式 |

---

## 🚀 快速开始

### 1. 准备工作

<details>
<summary><b>📋 获取飞书 API 凭证</b></summary>

1. 访问 [飞书开发者后台](https://open.feishu.cn/app)
2. 创建企业自建应用
3. 开通以下权限：
   - `drive:drive:readonly` - 查看云空间文件
   - `drive:file:read` - 读取文件内容  
   - `drive:media:download` - **下载媒体文件（必需）**
   - `wiki:wiki:readonly` - 查看知识库
4. 获取 **App ID** 和 **App Secret**

</details>

### 2. 构建安装

```bash
# 克隆并构建
git clone https://github.com/your-repo/feishu2md.git
cd feishu2md
make build

# 或直接编译
go build -o feishu2md cmd/main.go cmd/download.go
```

### 3. 基本使用

```bash
# 设置凭证（可选，也可用命令行参数）
export FEISHU_APP_ID="your_app_id"
export FEISHU_APP_SECRET="your_app_secret"

# 下载单个文档
./feishu2md "https://domain.feishu.cn/docx/xxxxx"

# 批量下载文件夹
./feishu2md --batch "https://domain.feishu.cn/drive/folder/xxxxx"

# 下载知识库子文档（保持层级结构）
./feishu2md --space-id "space_id" --wiki-children "https://domain.feishu.cn/wiki/xxxxx"
```

---

## 📖 详细用法

### 命令行选项

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--app-id` | 飞书应用ID | `$FEISHU_APP_ID` |
| `--app-secret` | 飞书应用密钥 | `$FEISHU_APP_SECRET` |
| `--space-id` | 知识库空间ID | `$FEISHU_SPACE_ID` |
| `--output` | 输出目录 | 当前目录 |
| `--title-as-filename` | 使用标题作为文件名 | `true` |
| `--image-dir` | 图片存储目录 | `img` |
| `--skip-img-download` | 跳过图片下载 | `false` |

### 下载模式详解

<details>
<summary><b>📄 单文档下载</b></summary>

```bash
./feishu2md [选项] "文档URL"
```

**输出结构：**
```
output/
├── 文档标题.md          # 文档内容
└── img/                # 图片文件
    ├── image1.png
    └── image2.jpg
```

</details>

<details>
<summary><b>📁 批量文件夹下载</b></summary>

```bash
./feishu2md --batch "文件夹URL"
```

下载整个文件夹中的所有文档。

</details>

<details>
<summary><b>📚 知识库下载</b></summary>

```bash
./feishu2md --wiki "知识库设置页面URL"
```

下载整个知识库的所有文档。

</details>

<details>
<summary><b>🌳 子文档递归下载（推荐）</b></summary>

```bash
# 下载指定文档下的所有子文档，保持层级结构
./feishu2md --space-id "空间ID" --wiki-children "文档URL"
```

**特性：**
- ✅ 递归获取所有层级的子文档
- ✅ 自动创建文件夹层级结构
- ✅ 智能跳过有子文档的父级文档
- ✅ 并发下载提高效率

**输出结构：**
```
output/
├── 一级文档/
│   ├── 文档1.md
│   ├── 子文件夹1/
│   │   ├── 文档2.md
│   │   └── img/
│   └── img/
└── 其他文档.md
```

</details>

### 实用示例

```bash
# 下载到指定目录，使用自定义图片文件夹
./feishu2md --output ./docs --image-dir "images" "文档URL"

# 使用令牌作为文件名
./feishu2md --title-as-filename=false "文档URL"

# 只下载文档，跳过图片
./feishu2md --skip-img-download "文档URL"

# 完整参数示例
./feishu2md \
  --app-id "your_app_id" \
  --app-secret "your_secret" \
  --space-id "space_id" \
  --output ./wiki-docs \
  --wiki-children \
  "https://domain.feishu.cn/wiki/xxxxx"
```

---

## 🔧 权限问题解决

### 常见问题：403 权限错误

**现象**: 即使开通了所有 API 权限，下载知识库图片时仍然出现 `403 Forbidden`

**原因**: 非公开知识库需要应用具有**协作者权限**，仅 API 权限不够

### 解决方案

<details>
<summary><b>🤖 方法1: 知识库全权限（推荐批量下载）</b></summary>

**适用场景**: 需要下载整个知识库内容

**步骤**:
1. 在[开发者后台](https://open.feishu.cn/app)为应用添加**机器人能力**并发布
2. 创建飞书群，将机器人添加到群中
3. 在知识库设置中，将该群添加为**管理者**
4. 重新测试下载

</details>

<details>
<summary><b>📄 方法2: 单文档权限（适用个别文档）</b></summary>

**适用场景**: 只需要下载特定文档

**步骤**:
1. 在[开发者后台](https://open.feishu.cn/app)为应用添加**云文档能力**并发布
2. 在目标文档的权限设置中，直接将应用添加为**协作者**
3. 重新测试下载

</details>

### 权限对照表

| 场景 | 解决方案 | 应用能力 |
|------|---------|---------|
| 公开知识库 | 直接使用 | 基础API权限 |
| 非公开知识库 | 机器人+群+管理权限 | 机器人能力 |
| 单个云文档 | 文档协作者权限 | 云文档能力 |

> 💡 **快速排查**: 遇到权限错误时，优先检查应用是否为相关资源的协作者

---

## ❓ 常见问题

<details>
<summary><b>Q: 文件名中的特殊字符如何处理？</b></summary>

A: 工具会智能替换特殊字符：
- `JavaScript/TypeScript` → `JavaScript-TypeScript`
- `项目：重要` → `项目-重要`
- `文档<新版>` → `文档《新版》`

</details>

<details>
<summary><b>Q: 图片下载失败怎么办？</b></summary>

A: 按顺序检查：
1. 确认开通了 `drive:media:download` 权限
2. 参考上方权限解决方案添加协作者权限
3. 使用 `--skip-img-download` 跳过图片下载

</details>

<details>
<summary><b>Q: 如何获取文档链接？</b></summary>

A: 
1. 打开飞书文档
2. 点击右上角"分享"→"链接分享"
3. 设置权限为"链接获得者可阅读"
4. 复制链接

</details>

<details>
<summary><b>Q: 支持哪些文档类型？</b></summary>

A: 仅支持飞书**新版文档(docx)**，不支持旧版文档(docs)

</details>

---

## 📄 开源协议

本项目基于 MIT 协议开源，详见 [LICENSE](LICENSE) 文件。

## 🙏 致谢

- [chyroc/lark](https://github.com/chyroc/lark) - 飞书 Go SDK
- [88250/lute](https://github.com/88250/lute) - Markdown 处理引擎

---

<div align="center">

**⭐ 觉得有用请给个 Star ⭐**

</div>