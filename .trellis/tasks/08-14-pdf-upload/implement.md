# 支持上传文档文件：执行计划

## 实施步骤

1. 重构 `internal/app/upload.go` 的类型模型、文件读取与持久化命名逻辑，使其覆盖图片和已确认的文档格式，同时维持现有 API 路径和字段名。
2. 实现 PDF、UTF-8 文本、EPUB 及 OpenXML（DOCX/XLSX/PPTX）的受限内容校验，并按图片 10 MiB、文档 50 MiB 返回一致的 `400`、`413`、`415` 错误。
3. 扩展公开文件名白名单与响应 MIME/`Content-Disposition` 逻辑。
4. 调整 `web/upload.html`、`web/upload.js` 和必要样式，支持所有格式的选择、拖放、分类型大小校验与无预览文档状态。
5. 更新 `internal/app/app_test.go`，覆盖所有允许类型、伪造/不一致内容、边界大小、公开响应头、路径限制和图片回归。
6. 更新 README 与项目上传契约，说明支持格式、大小限制、公开访问策略和上传目录不再仅存图片。
7. 构建静态 Linux amd64 二进制，发布至 ECS2 的新版本目录；原子切换 `current` 软链接并重启 `image-host.service`。
8. 备份 ECS2 Nginx 站点配置，将 `client_max_body_size` 从 `11m` 调整为 `51m`，语法检查通过后重载；执行 HTTPS 冒烟测试。

## 验证

- `gofmt -w internal/app/*.go`
- `go vet ./...`
- `go test ./...`
- 手动检查上传页：图片仍有预览；文档显示文件信息与链接；拖放、复制、重选和失败提示可用。
- ECS2：检查 `image-host.service` 为 active，Nginx 配置测试成功，并通过 `https://image.zhangjianyong.top` 完成登录、上传、文档公开访问和既有图片访问。

## 高风险文件与回滚点

- `internal/app/upload.go`：类型识别、请求上限和公开访问均集中于此；每完成一个校验分支先补对应测试。
- `web/upload.js`：不得对文档创建或赋值图片预览 URL。
- 若部署后出现兼容性问题，回滚服务二进制即可恢复现有图片行为；新增文档文件会保留在上传目录但旧版不会公开提供。
- 部署前记录 `current` 软链接目标并备份 Nginx 配置；重启服务或重载 Nginx 后的冒烟测试失败即按对应备份回滚。
