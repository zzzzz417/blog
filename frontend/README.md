# 前端

运行方式、编辑者登录、Markdown 格式、标签与上传配置见 [项目 README](../README.md)。

安装依赖用 `npm ci`，开发时运行 `npm run dev`。页面由 Vite 提供，`/api` 与 `/media` 代理到根目录 `.env` 中 `BLOG_DEV_API_TARGET` 指定的 Go 服务（默认 8080 端口）。后端也需要单独启动。

生产构建运行 `npm run build`，由 Go 服务读取 `dist/`。`npm test` 检查 Markdown 渲染与 HTML 净化。编辑器和文章阅读页共用 `src/markdown.ts`，页面分别放在 `src/views/`；登录状态在 `src/composables/useEditorAuth.ts`。
