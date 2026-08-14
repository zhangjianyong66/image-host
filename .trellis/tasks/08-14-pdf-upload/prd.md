# 支持上传文档文件

## Goal

在保持现有图片上传能力和安全校验不变的前提下，允许已登录用户上传 PDF、EPUB、TXT、Markdown（MD）、DOCX、XLSX、PPTX，并获得可公开访问的链接。

## Confirmed Facts

- 现有上传端点为 `POST /api/images`，表单字段名为 `image`；一次请求只能上传一个文件。
- 浏览器和服务端目前仅允许 JPEG、PNG、GIF、WebP，服务端同时校验 multipart 的声明 MIME 类型和文件内容检测结果。
- 所有上传文件当前共用 `uploads` 目录，通过随机文件名保存，并以 `/images/{name}` 公开访问。
- 图片上限为 10 MiB，页面以图片预览和图片结果展示为中心。

## Requirements

- 支持 PDF、EPUB、TXT、MD、DOCX、XLSX、PPTX，并保持 JPEG、PNG、GIF、WebP 的兼容。
- 不支持 DOC、XLS、PPT 等旧版二进制 Office 文件，也不支持未列出的格式。
- 所有二进制文档均必须通过服务端的文件签名或容器结构校验；TXT 和 MD 必须声明为允许的文本类型且内容为有效 UTF-8（可带 UTF-8 BOM）。
- 文档单文件上限为 50 MiB；既有图片继续维持 10 MiB 上限。
- 上传页面对所有文档仅显示文件类型、名称和大小；上传完成后显示文档状态和公开链接，不提供内容预览。
- PDF、TXT、MD 的公开链接在浏览器内直接打开；EPUB、DOCX、XLSX、PPTX 的公开链接强制下载。
- 既有 `POST /api/images`、multipart 字段 `image` 和公开路径 `/images/{随机文件名}` 保持兼容。
- 上传成功后返回可复制的公开链接。
- 完成验证后部署至 ECS2 的现有 `image-host.service`，并确保反向代理允许 50 MiB 文档请求。

## Acceptance Criteria

- [ ] 已登录用户可从上传页面选择或拖放每种已支持格式并成功上传。
- [ ] 服务端拒绝 MIME 类型与文件签名、容器结构或 UTF-8 文本内容不一致的文件，以及不受支持的格式。
- [ ] 图片上传、公开访问和既有 10 MiB 限制不发生回归。
- [ ] 支持的文档可在不超过 50 MiB 时上传，超过上限时会被浏览器和服务端拒绝。
- [ ] 每种支持格式都会被保存为随机文件名与正确扩展名，并能通过返回的公开链接访问。
- [ ] 文档选中态与上传完成态不会尝试将非图片文件作为图片渲染。
- [ ] PDF、TXT、MD 以正确的内容类型直接打开；EPUB、DOCX、XLSX、PPTX 携带下载响应头。
- [ ] ECS2 上的新静态 Linux amd64 二进制通过 `image-host.service` 运行，现有上传目录保持不变。
- [ ] ECS2 Nginx 将请求体上限提高到至少 51 MiB，配置测试通过后重载；HTTPS 下的登录、文档上传和公开访问冒烟测试通过。

## Out of Scope

- 文档内容预览、在线编辑、格式转换、缩略图和病毒扫描。
- 旧版 Office 二进制格式及其他未列出的格式。
- 修改既有图片的格式范围、大小上限、公开 URL 或 API 路径。
