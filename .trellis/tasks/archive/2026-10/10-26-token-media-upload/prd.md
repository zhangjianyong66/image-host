# 配置令牌的媒体上传接口

## 需求与验收
- 新增 POST /api/v1/media/images 与 /api/v1/media/images/batch。
- 单文件与最多九文件批量使用 multipart file 字段，分别返回 url 与有序 items[].url。
- 使用 Authorization: Bearer 令牌，配置名 NEWAPI_MEDIA_UPLOAD_TOKENS，支持逗号分隔。
- 未配置、错误令牌、仅有 Cookie 一律拒绝新接口；保留旧 Cookie 接口。
- 复用既有类型、大小、内容与随机文件名规则；非法请求无文件残留。
- 更新配置文件示例与说明，通过完整 Go 检查及构建。
- 生成独立接口文档，部署到 ecs2；先备份版本与配置，验证公网单文件、批量上传及鉴权，记录回滚方法。
