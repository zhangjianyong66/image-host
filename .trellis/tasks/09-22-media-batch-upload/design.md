# 技术设计

## API 契约

继续使用 `POST /api/images`，通过会话 Token 认证。请求为 `multipart/form-data`，最多包含 9 个名为 `image` 的文件部分。

响应统一为 JSON：

```json
{
  "results": [
    {"filename": "a.png", "status": "uploaded", "url": "https://host/images/abc.png"},
    {"filename": "b.mp4", "status": "failed", "error": "文件不能超过 200 MiB"}
  ]
}
```

单个文件失败不影响其他文件；服务端只持久化通过校验的文件。批量超过 9 个、请求无法解析或未认证属于请求级错误，分别返回 400、413 或 401；文件级 MIME、内容和大小问题写入对应 result。

## 后端边界

- 将单文件读取抽取为可复用的 multipart part 校验函数。
- 批量读取时限制文件数量为 9，并为每个 part 独立捕获校验错误。
- 请求体上限按 9 个视频上限加 multipart 开销重新计算，仍使用 `http.MaxBytesReader`。
- 新增音频 MIME：`audio/mpeg`、`audio/wav`、`audio/ogg`、`audio/mp4`；新增视频 MIME：`video/mp4`、`video/webm`、`video/quicktime`。
- 音频上限 50 MiB，视频上限 200 MiB；扩展名、公开响应 `Content-Type` 和 stored-name 白名单同步更新。
- 对音视频执行签名/容器校验：MP3/WAV/OGG/M4A 与 MP4/WebM/MOV 的最小魔数或容器头校验，避免只信任请求 MIME。

## 前端边界

- `input[type=file]` 增加 `multiple`，accept 同步覆盖新 MIME 和扩展名。
- 保持最多 9 个的选择校验，展示文件列表和逐项状态；上传进度显示总体进度。
- 图片继续预览，音视频显示文件类型占位信息；每个成功项提供可复制 URL，失败项显示错误。

## 兼容与迁移

不提供旧响应格式兼容层。迁移文档应指导客户端：将单个 `image` 字段改为可重复字段、读取 `results` 数组、按 `status` 处理成功和失败，并识别 401/400/413。旧客户端在升级前会因无法读取 `results` 而需要更新。

## 回滚

如上线后发现问题，回滚应用版本即可；上传目录中的新媒体文件仍可按已登记扩展名公开访问。不得通过删除上传文件进行回滚。
