# ecs2 部署验证

## 版本与备份
- 当前发布：/opt/image-host/releases/20261026155833，通过 current 软链接切换。
- 旧发布：/opt/image-host/releases/20260921104717。
- 备份：/opt/image-host/backups/20261026155833，包含原始环境配置、Nginx配置、previous-release及rollback.sh；目录0700。
- 回滚命令：通过 ssh ecs2 执行 bash /opt/image-host/backups/20261026155833/rollback.sh。恢复旧二进制、令牌配置与网关限制，保留上传文件。
- 本地 .env 的令牌已安全同步到服务端配置，权限0600；未输出或记录真实密钥。保留生产已有令牌。
- API文档副本：/opt/image-host/releases/20261026155833/api.md。

## 实际基础设施
- Nginx 实际二进制 /usr/sbin/nginx，运行配置 /usr/local/nginx/conf/nginx.conf；默认systemd nginx单元未激活，必须显式传-c。
- 最初按历史二进制路径检查失败，自动回滚；之后发现 story 站点缺失 fullchain.pem 链接阻止全局配置检查。
- 验证 archive/fullchain4.pem 有效期至2027-01-13，与现有cert和私钥匹配后，恢复 ../../archive/story.zhangjianyong.top/fullchain4.pem 链接；未修改该站点配置及证书内容。
- 图床站点 client_max_body_size 从51m调整为1802m，nginx -t通过后reload，服务active且enabled。

## 公网验收
- HTTPS healthz 返回ok，两个Token接口错误令牌401，旧Cookie接口拒绝Bearer。
- 使用真实 newapi_media.py 上传单文件及双文件批量，返回公网URL，匿名逐字节读取一致。
- 52MiB合成MP4校验样本上传成功，HEAD文件长度一致，验证原51MiB网关限制已解除。
- 正确令牌加畸形请求返回400 JSON。
- 已定点清理本次4个测试文件并验证404，没有删除其他上传文件。
- 本地test/vet/race/静态Linux构建均通过。
