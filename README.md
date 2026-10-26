# Image Host

一个轻量的本地文件托管服务：输入密码后批量上传媒体或文档，上传成功后展示可复制的公开链接；链接无需登录即可访问。

## 功能

- 仅密码登录，不需要用户名
- 支持最多 9 个文件批量上传：JPEG、PNG、GIF、WebP、MP3、WAV、OGG、M4A、MP4、WebM、MOV、PDF、EPUB、TXT、MD、DOCX、XLSX、PPTX
- 图片最大 10 MiB；音频和文档最大 50 MiB；视频最大 200 MiB
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
| `NEWAPI_MEDIA_UPLOAD_TOKENS` | 否 | 空（禁用） | API 上传令牌，每个至少 32 字符，多个用逗号分隔 |

## Token 上传接口

完整请求、响应、类型限制和错误说明见 [上传 API 文档](docs/api.md)。

自动化工具可使用 `Authorization: Bearer <token>` 上传，无需网页登录。
在生产配置文件 `/etc/image-host/image-host.env` 中增加：

```dotenv
NEWAPI_MEDIA_UPLOAD_TOKENS=替换为随机令牌
```

使用 `openssl rand -hex 32` 生成随机令牌，填写到上面的配置中，并将同一枚令牌设置为调用工具的 `NEWAPI_MEDIA_UPLOAD_TOKENS`。不要使用示例文字作为真实令牌，也不要把令牌提交到 Git。配置文件应限制访问权限；修改后需要重启图床服务才能生效。

配置支持逗号分隔的多个令牌，例如轮换时暂时同时保留新旧令牌；每枚至少 32 个字符，只允许英文字母、数字、`-._~+/` 和末尾的 `=`。空配置禁用这两个接口，错误或缺失令牌返回 `401`。令牌没有自动过期时间，删除配置中的对应令牌并重启即可撤销。

本地可将配置写入 `.env`（不要提交），与上面的 `UPLOAD_PASSWORD`、`SESSION_SECRET` 等配置一起加载。若 `.env` 仅包含上传令牌，启动前还需设置原有的上传密码和会话签名密钥。程序本身只读取环境变量，不会自动读取 `.env`：

```bash
set -a
. ./.env
set +a
go run ./cmd/image-host
```

单文件接口使用 multipart 字段 `file`：

```bash
curl --fail-with-body https://image.zhangjianyong.top/api/v1/media/images \
  -H "Authorization: Bearer ${NEWAPI_MEDIA_UPLOAD_TOKENS%%,*}" \
  -F 'file=@./picture.png;type=image/png'
```

成功返回 `201` 和 `{"filename":"picture.png","url":"https://image.zhangjianyong.top/images/0123456789abcdef0123456789abcdef.png"}`。

批量接口支持 1–9 个重复的 `file` 字段：

```bash
curl --fail-with-body https://image.zhangjianyong.top/api/v1/media/images/batch \
  -H "Authorization: Bearer ${NEWAPI_MEDIA_UPLOAD_TOKENS%%,*}" \
  -F 'file=@./one.png;type=image/png' \
  -F 'file=@./two.png;type=image/png'
```

成功返回 `201` 和 `{"items":[{"filename":"one.png","url":"..."},{"filename":"two.png","url":"..."}]}`，顺序与请求一致。虽然接口路径包含 `images`，同样支持本项目允许的音频、视频和文档，大小限制与网页上传一致。

请求先在私有临时目录逐文件校验和暂存，全部成功后发布；任何一项校验或保存失败，整批返回错误并清理本次文件。格式、字段或数量错误返回 `400`，超出大小限制返回 `413`，类型不支持或内容不匹配返回 `415`，存储失败返回 `500`，错误响应统一为 `{"error":"错误说明"}`。这与网页接口 `/api/images` 的逐项成功/失败结果不同；网页仍通过登录 Cookie 上传。

媒体生成工具配置：

```dotenv
IMAGE_HOST_BASE_URL=https://image.zhangjianyong.top
NEWAPI_MEDIA_UPLOAD_TOKENS=与服务端配置一致的令牌
```

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

Nginx 的 ACME 验证目录为 `/var/www/letsencrypt`。续期后应使用 `/usr/sbin/nginx -c /usr/local/nginx/conf/nginx.conf -t` 检查配置，并通过 deploy hook reload Nginx。
