# 技术设计

## 架构与边界

应用采用单个 Go 进程，Nginx 负责公网 TLS 终止与反向代理：

```text
浏览器 -> Nginx :80/:443 -> image-host :18090 -> /var/lib/image-host/uploads
                         -> 嵌入二进制的前端资源
```

- `cmd/image-host/main.go` 负责读取配置、创建持久化目录、启动 HTTP 服务和优雅退出。
- `internal/app` 负责路由、登录 Cookie、上传校验、文件持久化和页面资源响应。
- `web/` 存放通过 `embed` 打进二进制的 HTML、CSS、JavaScript，不需要在服务器单独部署前端文件。
- `deploy/` 提供 systemd 和 Nginx 配置模板；实际服务器配置以已验证的 `/usr/local/nginx` 布局为准。
- `/var/lib/image-host/uploads` 是服务器唯一持久化数据目录；发布目录只存放二进制，不存放图片。

## 接口与数据流

### 登录

1. `POST /api/login` 接收 JSON `{ "password": string }`。
2. 服务端对输入和 `UPLOAD_PASSWORD` 分别计算 SHA-256 后进行恒定时间比较。
3. 成功后写入带到期时间并由 `SESSION_SECRET` HMAC-SHA256 签名的 Cookie；失败返回 `401`。
4. `POST /api/logout` 写入过期 Cookie。
5. `GET /api/session` 返回当前是否已登录。

Cookie 在生产部署中设置 `HttpOnly`、`Secure`、`SameSite=Strict`，签名有效期默认 24 小时。服务端不保存会话数据库。

### 上传与公开访问

1. `POST /api/images` 要求有效登录 Cookie，接收字段名为 `image` 的单个 multipart 文件。
2. 请求体由 `http.MaxBytesReader` 限制，图片内容最多 10 MiB。
3. 服务端同时检查客户端 MIME 声明和 JPEG、PNG、GIF、WebP 文件头。
4. 校验通过后生成密码学随机文件名，使用独占创建避免覆盖，再写入上传目录。
5. 返回 `201` 和公开的绝对 HTTPS URL，前端在结果区展示该 URL，并提供带状态反馈的一键复制按钮。
6. `GET /images/<name>` 无需登录，使用安全文件名白名单从上传目录读取。
7. `GET /healthz` 用于本机部署和 Nginx 切换前的健康检查。

## 配置契约

- `LISTEN_ADDR`：默认 `127.0.0.1:8080`，服务器设置为 `127.0.0.1:18090`。
- `UPLOAD_DIR`：默认 `./uploads`，服务器设置为 `/var/lib/image-host/uploads`。
- `UPLOAD_PASSWORD`：必填，仅存放在 root 可读的 `/etc/image-host/image-host.env`。
- `SESSION_SECRET`：必填，使用部署时生成的高强度随机值。
- `PUBLIC_BASE_URL`：服务器设置为 `https://image.zhangjianyong.top`，保证代理后生成稳定公开链接。
- `COOKIE_SECURE`：服务器设置为 `true`，本地 HTTP 开发可设为 `false`。

## Nginx 与 SSL

- 保留 80 端口的 `/.well-known/acme-challenge/`，其他请求跳转 HTTPS。
- 443 端口继续引用 `/etc/letsencrypt/live/image.zhangjianyong.top/{fullchain,privkey}.pem`。
- HTTPS 的 `/` 全量反代到 `127.0.0.1:18090`，传递 `Host`、`X-Real-IP`、`X-Forwarded-For` 和 `X-Forwarded-Proto`。
- 站点级 `client_max_body_size` 设置为 11 MiB，应用仍以 10 MiB 图片内容为最终限制。
- 向 `/root/ssl_auto_renew/domains.conf` 增加该域名和 Nginx 配置路径，执行目标域名续期验证并确认 deploy hook 重载成功。

## 部署、切换与回滚

1. 本机执行 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build`。
2. 上传到 `/opt/image-host/releases/<release>/image-host`，更新 `/opt/image-host/current` 软链接。
3. 创建专用 `imagehost` 系统用户、持久化上传目录、环境文件和 `image-host.service`。
4. 启动新服务并通过 `127.0.0.1:18090/healthz` 验证，不影响线上旧页面。
5. 备份当前 `image.zhangjianyong.top.conf`，写入新代理配置，先执行 Nginx `-t`，通过后 reload。
6. 验证域名登录、上传、公开图片及现有共享后端服务后完成切换。
7. 旧的 `/usr/share/nginx/image` 和 `web-projects-hub-backend.service` 均保留；回滚只需恢复 Nginx 备份并 reload。

旧后端同时被其他域名使用，任何部署或回滚步骤都不得停止、禁用或覆盖 `web-projects-hub-backend.service`。

## 安全与错误处理

- 缺少密码或会话密钥时拒绝启动，仓库和日志中不保存凭据。
- 文件名完全由服务端生成，拒绝 SVG、扩展名路径、额外 multipart 文件和无法识别的内容。
- 校验失败不会在上传目录保留文件，写入失败返回通用错误且记录不含敏感信息的服务端日志。
- API 统一返回 `{ "error": "面向用户的错误信息" }`；大小超限为 `413`，未登录为 `401`，类型错误为 `415`。
- systemd 服务使用专用低权限用户并启用基础加固选项，只允许写入上传目录。
