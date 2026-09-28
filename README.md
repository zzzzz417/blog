# Listening

一个以 Markdown 为主体的个人博客。Go + SQLite 后端，Vue 前端；提供单列文章流、全文搜索、日期排序、标签筛选，以及带实时预览的文章编辑器。正文可以穿插多张图片和多个视频，列表封面单独选择，也可以不设置。

## 本地运行

需要 Go 1.26、支持 CGO 的 C 编译器，以及 Node.js 22.18+ 或 24.12+。

项目根目录提供 [`.env.example`](./.env.example)；首次克隆后复制为 `.env`。当前工作区已经有一份可直接运行的 `.env`，它被 Git 忽略。后端从 `backend/` 启动时会读取根目录 `.env`，进程环境变量优先；Vite 从同一文件读取 `BLOG_DEV_API_TARGET` 作为开发代理地址。若更改 `BLOG_ADDR` 的端口，也要同步更改代理地址。

```bash
# 终端一：API，默认监听 http://localhost:8080
cd backend
go run ./cmd/server

# 终端二：页面，默认监听 http://localhost:5173
cd frontend
npm ci
npm run dev
```

Vite 会把 `/api` 和 `/media` 请求代理到 Go 服务。默认构建使用全文子串搜索，无需额外编译标签。可选用 `go run -tags sqlite_fts5 ./cmd/server` 启用 SQLite 的 FTS5 英文词组索引；两种构建方式可以使用同一个数据库文件。中文查询使用子串匹配，以支持任意长度的中文关键词。搜索、标签筛选、排序与分页可以组合使用。

生产构建：

```bash
cd frontend && npm run build
cd ../backend && go run ./cmd/server
```

后端检测到 `../frontend/dist/index.html` 后，会在同一端口提供构建后的页面和静态资源。默认 SQLite 文件是 `backend/data/blog.db`，首次启动时自动建表。

## 在页面中管理文章

1. 在根目录 `.env` 设置 `BLOG_EDITOR_KEY`（至少 16 字符）和独立的 `BLOG_JWT_SECRET`（至少 32 字符），修改配置后重启 Go 服务。当前工作区已生成随机值，可以直接使用；请勿提交 `.env`。
2. 打开页面的 **关于 → 编辑**，输入 `BLOG_EDITOR_KEY` 的值，登录为编辑者。
3. 点击 **新增文章**，填写标题、Markdown 正文、发布日期。摘要、分类、封面、标签均可选；摘要留空时自动提取。
4. 通过 **插入图片 / 视频** 多选文件，也可拖入或粘贴到正文编辑区。上传完成后插入光标位置，右侧同步预览。可反复插入媒体，在它们之间继续写文字。
5. 封面可以单独上传、从正文图片中选择，或移除。它只用于首页列表，正文由 Markdown 决定。
6. 标签可输入新名称，或点击已有标签；每篇默认最多 **10 个**，自动去重。点击文章标签或使用首页下拉框筛选。
7. 阅读文章时点击 **编辑文章** 可修改或删除；在关于页面 **退出编辑**。未保存修改离开时会提示，登录过期后可重新登录继续保存当前草稿。

支持上传 JPEG、PNG、GIF、WebP 图片和 MP4、WebM 视频，默认单文件上限 **100 MB**。上传需要登录，后端校验实际文件类型和大小并生成随机文件名。上传文件存放在 `backend/data/uploads/`，通过 `/media/…` 访问，支持视频分段读取；无需重新构建前端。

## Markdown 源文件

页面保存会同时更新 `backend/content/posts/` 中的 Markdown 文件与 SQLite 索引；重启后内容仍然保留。也可以手动新增、修改或删除 `.md` 文件，再重启 Go 服务同步。文件示例：

```md
---
slug: first-post
title: 第一篇文章
summary: 用于首页列表的一段摘要
category: 生活
cover_image: /images/first-post.jpg
published_at: 2026-09-27
tags: ["生活", "照片"]
---

## 一段记录

文字可以写在媒体之间。

![第一张照片](/images/first-post.jpg)

这里继续写文字，也可以使用表格、列表或代码块。

<video controls playsinline preload="metadata" src="/videos/first-post.mp4"></video>

继续添加图片或视频即可，没有一篇只能放一个媒体的限制。
```

`slug`、`title`、`published_at` 为必填元数据，正文不能为空。`slug` 使用小写字母、数字和连字符，发布后固定；`summary`、`category`、`cover_image`、`tags` 可省略。封面接受本地 `/images/…` 或 `/media/…` 图片路径。标签支持 JSON 字符串数组，也兼容逗号分隔写法。

元数据采用简单的 `key: value` 格式，并非完整 YAML。阅读器与编辑预览共用 `marked` 和 `DOMPurify`，支持 GFM 表格、嵌套列表、引用、代码块、链接、图片及安全的 `<video>` / `<source>`。脚本、事件属性、危险 URL 等会被移除。视频建议使用浏览器广泛支持的 MP4 编码；WebM 的播放取决于浏览器对其编码的支持。

手动维护的图片仍可放在 `frontend/public/images/`，视频放在 `frontend/public/videos/`；部署时随前端构建发布。旧文章的 `kind`、`video_url` 元数据仍兼容读取，旧视频会进入正文。示例短片 `seaside-dusk.mp4` 是三秒播放样例。

### 持久化与备份

备份 `backend/content/posts/`、`backend/data/uploads/`、SQLite 数据文件及私有 `.env`。Markdown 是文章源文件，SQLite 是可重建的查询索引。网页写入采用文件原子替换与数据库事务；数据库失败时会恢复源文件。

删除文章会删除对应 Markdown 和数据库记录，**保留已上传媒体**，避免其他文章引用失效。取消编辑时已上传但未使用的文件也会保留；可在确认没有引用后手动清理。请使用单个后端实例管理同一套源文件与 SQLite。

## 结构

```text
backend/
  cmd/server/                    组合根、服务生命周期
  internal/config/               环境配置
  internal/server/               HTTP 路由组合
  internal/platform/             SQLite、日志、公共 HTTP 处理
  internal/editor/               密钥登录、JWT、编辑权限
  internal/media/                媒体上传、文件存储与读取
  internal/post/domain/          文章实体与仓储接口
  internal/post/app/             查询、编辑、标签与同步用例
  internal/post/infrastructure/  SQLite 仓储、Markdown 文件存储
  internal/post/transport/http/  HTTP 路由与响应
  content/posts/                 文章源文件
  data/uploads/                  运行时上传文件（不提交 Git）
frontend/
  src/views/                     文章、关于与编辑页面
  src/components/                标签选择、登录与文章卡片
  src/composables/                编辑者会话状态
  src/api.ts                     API 客户端、上传进度
  src/markdown.ts                共用 Markdown 渲染与净化
  public/images/                 图片
  public/videos/                 视频
```

业务代码围绕文章、编辑者和媒体纵向组织，用例与具体文件/数据库实现分离，由 `server` 和 `cmd/server` 组合。

## API 与鉴权

| 接口 | 说明 |
| --- | --- |
| `GET /api/health` | 健康检查 |
| `GET /api/v1/posts?q=&tag=&sort=newest&page=1&limit=10` | 列表、正文搜索、标签、日期排序和分页 |
| `GET /api/v1/posts/{slug}` | 文章详情与 Markdown 正文 |
| `GET /api/v1/tags` | 已有标签及文章数 |
| `GET /api/v1/auth/session` | 会话状态、标签和上传上限 |
| `POST /api/v1/auth/login` | JSON `{ "key": "编辑密钥" }` 登录 |
| `POST /api/v1/auth/logout` | 退出登录并清除 Cookie |
| `POST /api/v1/posts` | 新建文章，需要登录 |
| `PUT /api/v1/posts/{slug}` | 更新文章，需要登录 |
| `DELETE /api/v1/posts/{slug}` | 删除文章，需要登录 |
| `POST /api/v1/media` | multipart 字段 `file`，单次一个文件，需要登录 |
| `GET /media/{filename}` | 公开读取上传媒体，支持 Range |

编辑密钥只用于登录，JWT 使用独立密钥签名。JWT 存在 `HttpOnly`、`SameSite=Strict` 的 Cookie 中，不放在浏览器本地存储。服务端校验算法、签名、签发者、受众与过期时间，修改接口校验同源请求；登录接口按 IP 限速（15 分钟内最多 5 次失败尝试）。轮换 `BLOG_JWT_SECRET` 并重启服务可使已签发的会话失效。

## 配置

配置模板见 [`.env.example`](./.env.example)。下面的相对路径均以启动后端时的工作目录为准（按文档从 `backend/` 启动）。

| 配置项 | 默认值 / 说明 |
| --- | --- |
| `BLOG_ADDR` | `:8080` |
| `BLOG_DB_PATH` | `./data/blog.db` |
| `BLOG_CONTENT_DIR` | `./content/posts` |
| `BLOG_WEB_DIR` | `../frontend/dist` |
| `BLOG_LOG_LEVEL` / `BLOG_LOG_FORMAT` | `info` / `json`；格式也可为 `text` |
| `BLOG_EDITOR_KEY` | 留空则关闭编辑；启用时至少 16 字符 |
| `BLOG_JWT_SECRET` | JWT 签名密钥，启用编辑时至少 32 字符 |
| `BLOG_TOKEN_TTL` | `8h`，允许 `1m` 至 `168h` |
| `BLOG_COOKIE_SECURE` | `false`；线上 HTTPS 设置 `true` |
| `BLOG_MAX_TAGS` | `10`，可设置 `1` 至 `50` |
| `BLOG_UPLOAD_DIR` | `./data/uploads` |
| `BLOG_UPLOAD_MAX_MB` | `100`，单文件限制，可设置 `1` 至 `1024` |
| `BLOG_DEV_API_TARGET` | `http://127.0.0.1:8080`，仅供 Vite 开发代理使用 |
| `BLOG_ENV_FILE` | 进程环境变量，可指定其他配置文件路径 |

日志使用标准库 `slog`。线上反向代理应将页面、`/api`、`/media` 放在同一站点，并保留原始 `Host`；代理请求体上限和超时需容纳配置的上传大小。公开文章及媒体可直接访问，只有编辑接口需要登录。

## 检查

```bash
cd backend
go test ./...
go test -tags sqlite_fts5 ./...

cd ../frontend
npm test
npm run build
```

后端测试覆盖 JWT、未授权写入、文章与源文件持久化、失败回滚、标签组合查询、上传类型/大小限制和视频 Range。前端测试覆盖多媒体混排、Markdown 扩展语法与危险 HTML 清理。
