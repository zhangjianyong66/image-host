# Journal - zhangjianyong (Part 1)

> AI development session journal
> Started: 2026-07-29

---



## Session 1: 实现并部署 Go 图床

**Date**: 2026-07-29
**Task**: 实现并部署 Go 图床

### Summary

完成仅密码登录、本地图片上传、公开链接展示复制，并将服务部署到 ecs2 的 image.zhangjianyong.top；补齐 Nginx、systemd 和 SSL 自动续期。

### Main Changes

- 新增 Go 标准库单体服务、嵌入式前端、上传校验和自动化测试
- 部署 image-host.service 并将 image.zhangjianyong.top 切换到 127.0.0.1:18090
- 将 image 域名加入现有证书续期清单并完成 Let’s Encrypt 真实续期

### Git Commits

(No commits - planning session)

### Testing

- [OK] go test ./...、go test -race -cover ./...、go vet ./...、gofmt 和 Linux amd64 静态构建通过
- [OK] 线上登录上传、公开访问、服务重启持久化、Nginx 配置和公网证书验证通过

### Status

[OK] **Completed**

### Next Steps

- 项目根目录不是 Git 仓库，本次没有代码提交；需要时初始化或关联现有仓库后提交


## Session 2: 支持文档文件上传并部署 ECS2

**Date**: 2026-08-14
**Task**: 支持文档文件上传并部署 ECS2
**Branch**: `main`

### Summary

实现安全的文档上传、公开访问策略和 50 MiB 反向代理限制，并已部署到 ECS2。

### Main Changes

- 新增 PDF、EPUB、TXT、MD、DOCX、XLSX、PPTX 上传及服务端内容校验。
- 图片保留预览，文档使用文件信息与公开链接；更新部署模板和上传契约。
- 发布 ECS2 新版本并将 Nginx 请求体上限调整为 51 MiB。

### Git Commits

| Hash | Message |
|------|---------|
| `b355327` | (see git log) |

### Testing

- [OK] go test -race ./...、go vet ./...、静态 Linux amd64 构建、Node 语法检查。
- [OK] 线上 HTTPS 登录、PDF 上传与公开访问、既有 PNG 公开访问均通过。

### Status

[OK] **Completed**


## Session 3: 补全项目开发规范

**Date**: 2026-08-14
**Task**: 补全项目开发规范
**Branch**: `main`

### Summary

基于现有 Go 服务和原生前端实现，补全后端与前端 Trellis 开发规范，记录目录、错误处理、日志、测试、状态管理与类型边界。

### Git Commits

| Hash | Message |
|------|---------|
| `94eb28d` | (see git log) |

### Status

[OK] **Completed**


## Session 4: 完成令牌媒体上传接口与ecs2部署
<!-- trellis-session: v=2 fp=cfcf30c60fa2d068 -->

**Date**: 2026-10-27
**Task**: 完成令牌媒体上传接口与ecs2部署
**Branch**: `feat/token-media-upload`

### Summary

完成NEWAPI_MEDIA_UPLOAD_TOKENS鉴权、单文件及批量上传接口、测试和接口文档，部署ecs2并通过公网上传验证。

### Git Commits

| Hash | Message |
|------|---------|
| `140874a` | feat(upload): 支持配置令牌的媒体上传接口并补齐部署文档 |

### Testing

- [OK] go test、go vet、race及Linux静态构建通过；真实客户端、公网52MiB上传和测试文件清理通过。

### Status

[OK] **Completed**
