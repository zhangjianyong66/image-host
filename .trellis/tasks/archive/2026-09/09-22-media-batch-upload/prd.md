# 扩展图床批量上传与媒体类型

## Goal

放宽批量上传数量并支持图片、音频、视频文件上传

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
# 扩展图床批量上传与媒体类型

## 目标

在现有 Token 会话认证的图床服务中，解除当前单文件上传限制，支持一次选择并上传最多 9 个文件；同时保留现有图片和文档能力，新增音频、视频文件类型。

## 已确认事实

- `POST /api/images` 目前要求认证后提交单个 multipart `image` 文件。
- 当前支持 JPEG、PNG、GIF、WebP、PDF、EPUB、TXT、MD、DOCX、XLSX、PPTX。
- 图片最大 10 MiB，其他文档最大 50 MiB；后端会校验声明 MIME 与内容特征。
- 前端 `web/upload.html` / `web/upload.js` 当前只支持选择、预览和上传一个文件。
- 会话认证由登录密码换取签名 Cookie Token，上传接口通过该 Token 认证。

## 当前需求

- 批量上传上限从当前单文件提升为最多 9 个文件。
- 前端支持一次选择/拖放多个文件并提交。
- 上传文件类型增加音频和视频；具体格式、大小限制和浏览器展示方式待确认。
- 保持上传接口需要 Token 认证。

## 验收标准（初稿）

- 已认证用户可一次提交 1 至 9 个合法文件并获得每个文件的公开 URL。
- 第 10 个文件会被明确拒绝，且不会超出服务端配置的批量上限。
- 未认证请求仍返回未授权，不因批量接口改变认证行为。
- 图片、音频、视频上传后公开 URL 返回正确的 `Content-Type`，现有文档类型不回归。
- 非法 MIME、内容不匹配、超大小和空文件不会被持久化。
- 前端能展示批量选择状态、各文件上传结果和可复制 URL。

## 待确认决策

- 批量请求中一个文件失败时，成功文件保留并逐项报告失败（部分成功）。
- 接口不保留旧的单文件响应兼容；通过迁移文档指导旧客户端升级。
- 音频和视频的具体格式与单文件大小上限。
- 批量上传是否需要保持单个文件独立进度，还是只需要总体进度。

### 已确认的媒体范围

- 音频：MP3、WAV、OGG、M4A，单文件上限 50 MiB。
- 视频：MP4、WebM、MOV，单文件上限 200 MiB。

## 迁移约束

- `POST /api/images` 升级为批量协议，客户端提交多个同名 `image` multipart 部分。
- 响应统一为逐项结果数组；旧客户端读取顶层 `url` 的行为不再保证。
- 旧客户端迁移文档必须包含请求示例、响应示例、状态码和逐项错误处理方式。
