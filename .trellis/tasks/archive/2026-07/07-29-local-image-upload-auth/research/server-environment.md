# ecs2 服务器环境调研

## 已验证环境

- SSH 别名 `ecs2` 指向 `ecs2.zhangjianyong.top`，登录用户为 root。
- 系统为 Alibaba Cloud Linux 3，架构为 x86_64，根分区约有 16 GiB 可用空间。
- 服务器未安装 Go 工具链，应用需要在开发机交叉编译或上传工具链；本任务采用交叉编译静态二进制。
- `127.0.0.1:18090` 当前未被占用。

## Nginx

- Nginx 1.24.0 源码安装在 `/usr/local/nginx`，未注册 systemd 服务，也不在默认 PATH。
- 主配置 `/usr/local/nginx/conf/nginx.conf` 引入 `/usr/local/nginx/conf/conf.d/*.conf`。
- 进程正常监听 80/443，当前配置测试通过，全局 `client_max_body_size` 为 50 MiB。
- `image.zhangjianyong.top` 当前配置位于 `/usr/local/nginx/conf/conf.d/image.zhangjianyong.top.conf`。
- 当前站点将 HTTP 跳转到 HTTPS，静态资源来自 `/usr/share/nginx/image`，`/api/` 转发到 `127.0.0.1:18080`。
- 该站点当前标题为“ChatGPT 图片生成操作台”，用户已决定由新图床直接替换。

## 现有服务影响

- `127.0.0.1:18080` 对应 `web-projects-hub-backend.service`。
- 该后端和 `/opt/web-projects-hub` 目录同时被其他域名使用，不能因替换 image 域名而停止或删除。
- 新图床应使用独立端口和 systemd 单元，仅替换 image 域名的 Nginx 路由。

## SSL 自动续期

- Certbot 1.22.0 管理证书，没有 `certbot.timer`；`crond` 处于启用和运行状态。
- root crontab 每天 02:00 执行 `/root/ssl_auto_renew/ssl_auto_renew.sh apply auto`，每周日 03:00 执行证书检查和日志清理。
- Certbot deploy hook 为 `/etc/letsencrypt/renewal-hooks/deploy/reload-local-nginx.sh`，使用绝对路径检查并 reload Nginx。
- `image.zhangjianyong.top` 证书使用 webroot `/var/www/letsencrypt`，Nginx 已公开对应 ACME challenge 路径。
- 当前证书到期时间为 2026-08-12，但该域名遗漏在 `/root/ssl_auto_renew/domains.conf`，自定义定时任务不会处理它。
- 2026-07-29 的自动任务完成 30/32 个清单域名；现有其他域名失败不阻止 Certbot deploy hook 对成功续期证书执行 reload，但会使自定义脚本整体报告失败。

## 部署约束

- 修改服务器前必须备份 image 域名 Nginx 配置和 SSL 域名清单。
- Nginx 操作使用 `/usr/local/nginx/sbin/nginx -t` 与 `-s reload`，不能使用 `systemctl reload nginx`。
- 证书直接引用 `/etc/letsencrypt/live/image.zhangjianyong.top/`，无需复制到 Nginx 私有证书目录。
