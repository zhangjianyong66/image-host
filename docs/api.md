# 图床上传 API

## 地址与鉴权

生产地址：`https://image.zhangjianyong.top`。

所有 Token 上传请求必须携带一个请求头：

```http
Authorization: Bearer <上传令牌>
```

上传令牌由服务管理员配置，不提供在线申请或查询接口。登录密码、登录 Cookie 和查询参数中的令牌均不能替代这个请求头。请通过 HTTPS 调用，不要在 URL、日志、截图或 Git 中暴露令牌。

## 服务端配置

在 ecs2 的 `/etc/image-host/image-host.env` 中配置：

```dotenv
NEWAPI_MEDIA_UPLOAD_TOKENS=替换为随机令牌
```

使用 `openssl rand -hex 32` 生成 64 字符随机令牌，并将同一枚令牌配置到调用工具。配置文件建议权限为 `0600`。修改后重启 `image-host.service` 生效。

- 支持多枚令牌，以英文逗号分隔；配置项首尾空白会被去除。
- 每枚至少 32 字符，可包含英文字母、数字、`-._~+/`，以及只出现在末尾的 `=`。
- 不允许空列表项，例如末尾多写一个逗号；不合法配置会阻止服务启动。
- 未配置或仅为空白时，Token 上传被禁用，新接口均返回 `401`。
- 令牌无自动过期时间。轮换时先同时配置新旧令牌并重启，更新客户端，再移除旧令牌并重启。
- 原有 `UPLOAD_PASSWORD`、`SESSION_SECRET` 等启动配置仍然必需。

本地程序只读取环境变量，不自动解析 `.env`。配置写入本地 `.env` 后，通过 shell 加载：

```bash
set -a
. ./.env
set +a
```

以下调用示例假设已加载配置，且令牌列表没有额外空白。`${NEWAPI_MEDIA_UPLOAD_TOKENS%%,*}` 选取第一枚令牌；HTTP 请求中只能发送一枚令牌。

## 单文件上传

`POST /api/v1/media/images`

请求采用 `multipart/form-data`，必须且只能包含一个文件字段，字段名为 `file`，需携带文件名与正确的文件 MIME 类型。不要手动设置整个请求的 `Content-Type`，由上传客户端生成 multipart boundary。

```bash
curl --fail-with-body \
  'https://image.zhangjianyong.top/api/v1/media/images' \
  -H "Authorization: Bearer ${NEWAPI_MEDIA_UPLOAD_TOKENS%%,*}" \
  -F 'file=@./picture.png;type=image/png'
```

成功状态为 `201 Created`，响应 `Content-Type: application/json; charset=utf-8`：

```json
{
  "filename": "picture.png",
  "url": "https://image.zhangjianyong.top/images/0123456789abcdef0123456789abcdef.png"
}
```

`filename` 为客户端文件名的基本名称；服务端不使用它存储文件。`url` 是服务端生成随机文件名后的公开绝对地址，可直接使用。

## 批量上传

`POST /api/v1/media/images/batch`

同样采用 `multipart/form-data`，包含 1 至 9 个重复的 `file` 文件字段，不支持其他表单字段。顺序与请求中的文件顺序一致。

```bash
curl --fail-with-body \
  'https://image.zhangjianyong.top/api/v1/media/images/batch' \
  -H "Authorization: Bearer ${NEWAPI_MEDIA_UPLOAD_TOKENS%%,*}" \
  -F 'file=@./one.png;type=image/png' \
  -F 'file=@./two.jpg;type=image/jpeg'
```

成功状态为 `201 Created`：

```json
{
  "items": [
    {
      "filename": "one.png",
      "url": "https://image.zhangjianyong.top/images/0123456789abcdef0123456789abcdef.png"
    },
    {
      "filename": "two.jpg",
      "url": "https://image.zhangjianyong.top/images/abcdef0123456789abcdef0123456789.jpg"
    }
  ]
}
```

服务端逐文件校验并在私有目录暂存，全部成功后发布。任一文件校验或保存失败，整批返回错误，并清理本次暂存及已发布文件，不返回部分成功结果，不影响已有文件。这是请求失败后的回滚，不代表服务器异常关机时具备多文件事务保证。

## 支持类型与大小

路径虽然包含 `images`，两个接口都支持以下媒体和文档。`1 MiB = 1,048,576` 字节，大小上限包含等于边界的文件。

| 类型 | 文件格式 | multipart 文件 MIME 类型 | 单文件上限 |
| --- | --- | --- | --- |
| 图片 | JPEG | `image/jpeg` | 10 MiB |
| 图片 | PNG | `image/png` | 10 MiB |
| 图片 | GIF | `image/gif` | 10 MiB |
| 图片 | WebP | `image/webp` | 10 MiB |
| 音频 | MP3 | `audio/mpeg` | 50 MiB |
| 音频 | WAV | `audio/wav` | 50 MiB |
| 音频 | OGG | `audio/ogg` | 50 MiB |
| 音频 | M4A | `audio/mp4` | 50 MiB |
| 视频 | MP4 | `video/mp4` | 200 MiB |
| 视频 | WebM | `video/webm` | 200 MiB |
| 视频 | MOV | `video/quicktime` | 200 MiB |
| 文档 | PDF | `application/pdf` | 50 MiB |
| 文档 | EPUB | `application/epub+zip` | 50 MiB |
| 文档 | TXT | `text/plain` | 50 MiB |
| 文档 | Markdown | `text/markdown` | 50 MiB |
| 文档 | DOCX | `application/vnd.openxmlformats-officedocument.wordprocessingml.document` | 50 MiB |
| 文档 | XLSX | `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` | 50 MiB |
| 文档 | PPTX | `application/vnd.openxmlformats-officedocument.presentationml.presentation` | 50 MiB |

空文件、SVG 和其他类型不支持。服务端同时检查声明的 MIME 类型、文件签名或容器结构；文本要求有效 UTF-8。仅修改后缀或 MIME 类型不能绕过校验。

单文件接口整个请求体上限为 202 MiB，批量接口为 1802 MiB，均包含 multipart 开销。Nginx 模板允许最大 1802 MiB 的请求体，实际仍按每个接口和文件的上限校验。大文件还会受服务器上传超时和磁盘空间约束，客户端超时应按网络状况设置。

## 错误响应

应用返回的错误格式统一为：

```json
{"error":"上传令牌缺失或无效"}
```

| 状态码 | 含义 | 常见原因 |
| --- | --- | --- |
| `400 Bad Request` | 请求不合法 | 非 multipart、缺少文件、字段名错误、文件为空、单文件接口提交多项、批量超过 9 项、请求被截断 |
| `401 Unauthorized` | 未通过 Token 鉴权 | 未配置令牌、请求头缺失或重复、错误令牌、仅发送登录 Cookie；响应包含 `WWW-Authenticate: Bearer` |
| `413 Request Entity Too Large` | 大小超限 | 单个文件或整个请求体超过上限 |
| `415 Unsupported Media Type` | 文件类型不支持 | MIME 类型不在允许列表、内容与声明不匹配 |
| `500 Internal Server Error` | 服务端存储失败 | 暂存、写盘或发布文件失败 |

反向代理也可能直接返回错误，例如 Nginx 的 `413` 或网关超时，此时响应不保证为上述 JSON。网络中断或客户端未收到响应时，上传可能已经成功；接口没有幂等键，直接重试可能产生重复文件。

## 读取公开文件

对上传返回的 `url` 发起 `GET` 即可，不需要令牌或 Cookie。文件以随机名称持久化，不随普通服务重启删除。PDF、TXT 和 Markdown 默认可在浏览器打开；EPUB 和 Office 文档默认下载。无效或不存在的文件地址返回 `404`。

没有文件删除或令牌查询接口。公开链接不应作为私密文件访问控制。

## 媒体工具配置

调用工具的环境配置为：

```dotenv
IMAGE_HOST_BASE_URL=https://image.zhangjianyong.top
NEWAPI_MEDIA_UPLOAD_TOKENS=与服务端一致的上传令牌
```

`IMAGE_HOST_BASE_URL` 配置在调用端；图床服务返回链接的地址由服务端 `PUBLIC_BASE_URL` 配置。服务端和客户端使用相同的令牌值，客户端可配置多枚令牌用于依次尝试。

## 与网页上传接口的区别

| 项目 | Token 上传 | 网页上传 |
| --- | --- | --- |
| 路径 | `/api/v1/media/images`、`/api/v1/media/images/batch` | `/api/images` |
| 鉴权 | `Authorization: Bearer <token>` | 密码登录后获得的 `image_host_session` Cookie |
| 文件字段 | `file` | `image` |
| 成功响应 | 顶层 `url` 或 `items[].url` | `results[]` 含 `status`、`url` 或 `error` |
| 批量失败 | 整批失败并回滚本次文件 | 保留成功项，逐项返回结果 |

Token 不用于网页登录，登录 Cookie 也不用于 Token 上传。现有网页上传行为保持兼容。
