# Journal - zhangjianyong (Part 1)

> AI development session journal
> Started: 2026-07-29

---



## Session 1: 实现并部署 Go 图床

**Date**: 2026-07-29
**Task**: 实现并部署 Go 图床

### Summary

完成仅密码登录、本地图片上传、公开链接展示复制，并将服务部署到 ecs2 的 image.zhangjianyong.top；补齐 Nginx、systemd 和 SSL 自动续期。

### Main Changes

- 新增 Go 标准库单体服务、嵌入式前端、上传校验和自动化测试
- 部署 image-host.service 并将 image.zhangjianyong.top 切换到 127.0.0.1:18090
- 将 image 域名加入现有证书续期清单并完成 Let’s Encrypt 真实续期

### Git Commits

(No commits - planning session)

### Testing

- [OK] go test ./...、go test -race -cover ./...、go vet ./...、gofmt 和 Linux amd64 静态构建通过
- [OK] 线上登录上传、公开访问、服务重启持久化、Nginx 配置和公网证书验证通过

### Status

[OK] **Completed**

### Next Steps

- 项目根目录不是 Git 仓库，本次没有代码提交；需要时初始化或关联现有仓库后提交
