# 实施计划

## 应用实现

- [x] 初始化 Go 1.22+ 模块，建立 `cmd/`、`internal/`、`web/` 和 `deploy/` 目录。
- [x] 实现配置读取、仅密码登录、HMAC 登录 Cookie、会话检查和退出接口。
- [x] 实现请求大小限制、文件头校验、随机命名、独占写入、本地持久化和公开图片响应。
- [x] 实现登录页、受保护上传页、拖放/选择上传、预览、结果链接和复制操作。
- [x] 增加 `.gitignore`、上传目录占位文件、环境变量示例和中文 README。
- [x] 增加 Go 自动化测试，覆盖鉴权、Cookie 篡改/过期、合法上传、公开读取、类型/大小/空文件拒绝及路径安全。
- [x] 增加 systemd 和 Nginx 部署模板，确保与 ecs2 的实际路径和代理方式一致。

## 本地验证

```bash
gofmt -w .
go vet ./...
go test ./...
UPLOAD_PASSWORD=test-password SESSION_SECRET=test-session-secret COOKIE_SECURE=false go run ./cmd/image-host
```

- 未登录访问 `/upload` 进入登录流程。
- 正确密码可上传 JPEG、PNG、GIF、WebP，错误密码和非法文件被拒绝。
- 清除 Cookie 后仍能打开已上传图片链接。
- 服务重启后图片仍可访问。

## ecs2 部署

- [x] 交叉编译 Linux amd64 静态二进制并校验产物。
- [x] 在服务器创建独立发布目录、`imagehost` 用户、上传目录和 root 可读环境文件。
- [x] 生成随机 `SESSION_SECRET` 和初始上传密码，安装并启动 `image-host.service`。
- [x] 在本机端口 `18090` 完成健康检查、登录、上传和公开读取冒烟测试。
- [x] 备份现有 Nginx 站点配置，安装新代理配置，执行 `/usr/local/nginx/sbin/nginx -t` 后 reload。
- [x] 通过 `https://image.zhangjianyong.top` 验证完整流程，并确认共享旧后端服务仍在运行。
- [x] 将域名补入 `/root/ssl_auto_renew/domains.conf`，备份原文件并执行目标证书续期/验证。
- [x] 核对线上证书日期、cron 配置、续期日志和 Nginx worker reload 时间。

## 回滚点

- Nginx 切换前不影响线上；新服务失败时直接停止部署，不修改现有站点。
- Nginx 切换后若验证失败，恢复带时间戳的原站点配置并 reload。
- 不删除旧静态资源，不停止共享的 `web-projects-hub-backend.service`。
- 新服务发布回滚通过切换 `/opt/image-host/current` 到上一版本并重启完成，上传目录不变。

## 完成前检查

- [x] PRD 中没有未决问题。
- [x] `implement.jsonl` 和 `check.jsonl` 包含真实的规范与服务器研究上下文。
- [x] 用户已批准包含 Go、直接替换域名和线上部署的最终规划摘要。
