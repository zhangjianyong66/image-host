# Image Host

一个轻量的本地文件托管服务：输入密码后上传图片或文档，上传成功后展示可复制的公开链接；链接无需登录即可访问。

## 功能

- 仅密码登录，不需要用户名
- 支持 JPEG、PNG、GIF、WebP、PDF、EPUB、TXT、MD、DOCX、XLSX、PPTX
- 图片最大 10 MiB；文档最大 50 MiB
- 服务端校验浏览器声明类型与文件签名、容器结构或 UTF-8 文本内容
- 服务端生成随机文件名，本地持久化存储
- 图片上传后预览；文档显示文件信息与可复制的公开链接
- PDF、TXT、MD 可直接在浏览器打开；EPUB 和现代 Office 文档会下载

## 本地运行

需要 Go 1.22 或更高版本。

```bash
export UPLOAD_PASSWORD='替换为上传密码'
export SESSION_SECRET='替换为至少32字符的随机密钥'
export COOKIE_SECURE=false
go run ./cmd/image-host
```

浏览器打开 `http://127.0.0.1:8080`。默认上传目录为 `./uploads`，可通过以下环境变量调整：

| 变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `UPLOAD_PASSWORD` | 是 | 无 | 上传页面的登录密码 |
| `SESSION_SECRET` | 是 | 无 | 至少 32 字符，用于签名登录 Cookie |
| `LISTEN_ADDR` | 否 | `127.0.0.1:8080` | HTTP 监听地址 |
| `UPLOAD_DIR` | 否 | `./uploads` | 上传文件持久化目录 |
| `PUBLIC_BASE_URL` | 否 | `http://localhost:8080` | 返回给用户的公开链接前缀 |
| `COOKIE_SECURE` | 否 | `true` | HTTPS 部署设为 `true`，本地 HTTP 设为 `false` |

## 检查与构建

```bash
gofmt -w .
go vet ./...
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o image-host ./cmd/image-host
```

## 服务器部署布局

- 二进制：`/opt/image-host/current/image-host`
- 配置：`/etc/image-host/image-host.env`
- 上传文件：`/var/lib/image-host/uploads`
- systemd：`/etc/systemd/system/image-host.service`
- Nginx：`/usr/local/nginx/conf/conf.d/image.zhangjianyong.top.conf`

仓库的 `deploy/` 包含与 ecs2 当前环境对应的 systemd 和 Nginx 配置模板。生产配置示例：

```bash
LISTEN_ADDR=127.0.0.1:18090
UPLOAD_DIR=/var/lib/image-host/uploads
UPLOAD_PASSWORD=替换为强密码
SESSION_SECRET=替换为随机密钥
PUBLIC_BASE_URL=https://image.zhangjianyong.top
COOKIE_SECURE=true
```

上传目录必须放在发布目录之外并定期备份。当前 MVP 没有磁盘配额、文件删除、病毒扫描和内容审核功能，应监控磁盘使用量。

## SSL 续期

ecs2 使用 root cron 和 `/root/ssl_auto_renew/ssl_auto_renew.sh` 管理证书。`image.zhangjianyong.top` 必须登记为：

```text
image.zhangjianyong.top:/usr/local/nginx/conf/conf.d/image.zhangjianyong.top.conf
```

Nginx 的 ACME 验证目录为 `/var/www/letsencrypt`。续期后应使用 `/usr/local/nginx/sbin/nginx -t` 检查配置，并通过 deploy hook reload Nginx。
