# 更新日志

格式照 `v<MAJOR>.<MINOR>.<PATCH>-<YYYYMMDD>`（tag 即放行：全量回归绿后打 tag）。

## v0.2.0-20261027 (2026-10-27)

REQ-2026-004 lab-go 全量业务切片——骨架态推到全量业务面（配方照 REQ-2026-003 saas-go，语义源换 lab rails 同门）：

- auth 代理链（Task 3-5）：账密校验代理 saas IdP（4xx→401 INVALID_CREDENTIALS / 5xx→502 SAAS_UNAVAILABLE）、lab 自签 JWT 无 aud、refresh 7d rotate-once、服务账号快照 UserDirectory/SaasMenuMapper
- 检测能力字典面（Task 6-9）：inspection_* 20 表 CRUD + 树/枚举/级联/关联面（字典主键=业务 code）
- 样品全流程面（Task 10-11）：samples/test-records 12 端点 + receipts CRUD/七段 act 状态机/5.57 三态 filter/assignTask + summary 汇总/仪表盘（stats 键 raw 不 camel，rails render json: 镜像）+ frontend-bind 404 占位
- 全量 route-parity：shared openapi 64 path / 120 op 全挂接（allowlist 仅 GET /health）
- 验收：gate L0-L5 全绿 + strict 幂等 PASS + rails L4 交叉绿 + lab CT live 2-way（springboot,go）394/394 收敛 + live 探针 native-login→me→menus→samples→stats 全 200

## v0.1.1-20261026 (2026-10-26)

REQ-2026-002 进 prod deploy 链（ktor/laravel 先例五件套）：

- deploy 五件套：Dockerfile（golang:1.27-alpine builder + CGO_ENABLED=0 静态二进制，alpine:3 runtime）+ .dockerignore + ci.yml（fmt/vet/build/test + 镜像装配冒烟 build+run+/health+405 守卫；deploy job tag 触发）+ deploy/ 脚本 + nginx vhost 模板 + setup-vps.sh
- 绑定地址 `127.0.0.1` → `:port`（容器可达；ktor Netty 默认 0.0.0.0 先例）
- `https://{domain}` 上线（X13={port}，host=container 单层映射，ADR-0018）

## v0.1.0-20261026 (2026-10-26)

REQ-2026-001 健康检查 + 端口装配最小切片——骨架可跑首航：

- `internal/config`：`SERVER_PORT` fail-fast 装配（缺键/非数字/超范围报错，禁默认值兜底）
- `internal/server`：`GET /health` → `{"ok":true,"mode":"go"}`（家族探活契约形状）+ 405 守卫
- `cmd/server`：组合根 `New(addr)`（实机指纹：http.Server 零值 Addr 落默认 :80，必须显式传地址）
- 删 `internal/harness` 空仓锚点包（首个装配包落地）
- live 验收：GET 200 / POST 405 / 缺 env exit 1；门禁 L0-L5 全绿（16 条目 0 软告警）
