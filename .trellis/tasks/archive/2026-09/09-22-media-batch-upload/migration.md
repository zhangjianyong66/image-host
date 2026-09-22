# 客户端迁移说明：批量媒体上传 API

## 变化

`POST /api/images` 不再返回单文件 `{ "url": "..." }`，改为返回 `results` 数组。请求仍需携带登录后获得的 Token Cookie。

## 请求示例

```bash
curl -b cookie.txt -F 'image=@one.png' -F 'image=@two.mp4' https://example.test/api/images
```

最多提交 9 个 `image` 字段。支持图片、原有文档，以及 MP3/WAV/OGG/M4A 音频和 MP4/WebM/MOV 视频。

## 响应示例

```json
{
  "results": [
    {"filename":"one.png","status":"uploaded","url":"https://example.test/images/....png"},
    {"filename":"two.mp4","status":"failed","error":"文件内容与声明的类型不一致"}
  ]
}
```

客户端应遍历 `results`：`status=uploaded` 时使用 `url`，`status=failed` 时展示 `error`。结果顺序与上传顺序一致。

## 请求级状态码

- `201 Created`：请求已处理，数组中可能同时存在成功和失败项。
- `400 Bad Request`：multipart 无法解析、没有文件或字段错误。
- `401 Unauthorized`：Token 缺失、过期或无效。
- `413 Request Entity Too Large`：单文件或整个请求超过限制。

旧客户端若只读取顶层 `url`，需要改为读取 `results[].url`；服务端不提供旧响应兼容字段。
