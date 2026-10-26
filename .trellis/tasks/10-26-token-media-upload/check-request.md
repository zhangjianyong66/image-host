# 全量独立检查请求

你是 trellis-check 子代理，请直接审查当前任务所有改动并修复，不再派生代理。之前无语义字符串消息请忽略。

Active task: .trellis/tasks/10-26-token-media-upload

读取 prd/design/implement/check.jsonl 及相关规范。重点检查 NEWAPI_MEDIA_UPLOAD_TOKENS 配置、Bearer 鉴权、单文件/批量上传响应、内存限制、暂存和发布失败回滚、现有 Cookie 上传兼容与测试边界。对可证实问题补充测试并修复，不扩大范围。不要查看或输出 .env 密钥，不提交、不部署。你不独占工作区，不撤销别人改动。全部实现已完成，主代理只管理任务文档和规范。

已通过 go test、vet、race、静态 Linux 构建及真实媒体工具端到端上传。检查后按需重跑受影响检查，报告问题、修复和最终验证。
