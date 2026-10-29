# 170 开发与运行手册

## 工作目录

- SSH 主机：170
- 项目目录：/home/hanruoyu/DIS-reconstruct
- 项目用户：hanruoyu
- 远程仓库：git@github.com:han-ry19/DIS-reconstruct.git

代码构建、测试、部署和实验统一在 170 执行。

## 开发入口

在 170 登录项目用户后执行：

    cd /home/hanruoyu/DIS-reconstruct
    GOTOOLCHAIN=local /usr/bin/go test ./...
    GOTOOLCHAIN=local /usr/bin/go run ./cmd/dis-api

API 当前提供 /healthz 健康检查，默认监听 8080 端口。当前代码包含领域模型、仓储接口及服务入口骨架；CLI、同步服务和链码业务接口待接通。Go 包编译检查已通过，目前没有自动化测试用例。

## 系统接通顺序

身份注册与公钥查询 → 数据登记 → 授权申请与决定 → Fabric 提交 → CoreDNS 同步 → 完整业务用例。

每一步围绕相同业务场景接通，并在运行文档中补充实际命令和验收结果。
