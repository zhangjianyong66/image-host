# 验证记录

- 实现代理：gofmt、go test ./...、go vet ./...、go test -race ./...、Linux amd64 静态构建及 git diff --check 通过。
- 主代理独立使用真实 newapi-media-generation/scripts/newapi_media.py 的 upload_media 调用本地构建服务，使用本地配置令牌；单文件、双文件批量、三个URL匿名读取、错误令牌401、无暂存目录残留均通过。
- 本地 .env 已生成 NEWAPI_MEDIA_UPLOAD_TOKENS，权限0600，已验证Git忽略；未输出或跟踪令牌。
- 后续用户授权后已部署 ecs2，详见 research/deployment.md。
- 独立 check 代理未发现生产代码缺陷；补充确定性发布冲突回归，验证返回500、回滚本次已发布文件、保留预存文件及清理暂存目录。再次通过 test、vet、race、静态构建和 diff 检查。
- 开发及验收已完成，代码位于 feat/token-media-upload；用户已确认提交，按流程提交后归档，真实 .env 不在提交范围内。
