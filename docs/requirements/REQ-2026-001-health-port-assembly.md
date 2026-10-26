# REQ-2026-001 Go 端健康检查 + 端口装配最小切片（骨架可跑，saas-go 复刻）

| 项 | 值 |
|---|---|
| 提出人 | zcqiand |
| 提出日期 | 2026-10-26 |
| 优先级 | P1 |
| 状态 | 已验收（2026-10-26：live 探针 GET /health → `{"mode":"go","ok":true}`、POST 405、L0-L5 全绿；自动化验收，人裁复核随检视） |
| 关联 ADR | —（基础设施端点不进契约面，ktor REQ-2026-001/003 先例） |

## 1. 需求描述

saas-identity-platform-go REQ-2026-001（健康检查 + 端口装配最小切片）的原样复刻——saas 先行验证全流程，lab 照配方复刻（ktor/laravel/miniprogram 家族先例同款）。

代码同源：`internal/config`（SERVER_PORT fail-fast）+ `internal/server`（GET /health → `{"ok":true,"mode":"go"}`）+ `cmd/server` 组合根，仅模块导入路径与槽位端口不同（5211）。包含 saas 侧实机抓到的 `New(addr)` 指纹修复（http.Server 零值 Addr 落 :80）。

### 澄清记录

| 疑问 | 澄清结论 | 澄清人 | 日期 |
|---|---|---|---|
| 复刻范围与顺序 | saas-go 先行，lab-go 原样复刻（选项确认） | zcqiand | 2026-10-26 |
| /health 是否挂功能 ID / 动功能树 | 不挂不动——基础设施端点影响功能数 0（ktor REQ-2026-001/003 台账先例；与 saas-go 同判） | 依 ktor 台账先例（如不认可，退回本表重议） | 2026-10-26 |

## 2. 验收标准

| 编号 | 场景（给定） | 操作（当） | 预期（则） |
|---|---|---|---|
| AC-1 | .env 含 `SERVER_PORT=5211` | 启动服务 | 进程监听 `127.0.0.1:5211`；端口值只来自 env，代码无默认值兜底 |
| AC-2 | 服务已启动 | `GET /health` | 200 且 body 为 `{"ok":true,"mode":"go"}`（家族探活契约形状） |
| AC-3 | .env 缺 `SERVER_PORT` | 启动服务 | fail-fast 退出非零，错误信息指明缺失键名 |
| AC-4 | 仓内实现与测试就位 | `python scripts/gate.py -p lab-management-system-go` | L0-L5 全绿；trace.json 由 harness_trace.py 产出，/health 相关测试 fns 为空数组 |
| AC-5 | `internal/server` 等包落地 | `go vet ./...` | `./...` 非空通过，`internal/harness` 锚点包删除 |

## 3. 任务拆解

| 任务 ID | 任务描述 | 类型 | 负责人 | 预估 | 状态 |
|---|---|---|---|---|---|
| T-1 | saas-go 四文件复制 + 模块导入路径改 `lab-management-system-go` + 槽位端口 5211 | 开发 | Claude | 0.2h | 已完成 |
| T-2 | 删 `internal/harness` 锚点包 + 测试绿（config 4 例 + server 2 例同源） | 开发 | Claude | 0.1h | 已完成 |
| T-3 | live 验收（5211 探针）+ 门禁 L0-L5 收口 + 提交推送 + gitlink bump | 收口 | Claude | 0.3h | 已完成 |

## 4. 功能影响（需求与功能对齐的唯一位置）

| 功能 ID | 功能名称 | 影响类型 | 说明 | 关联任务 |
|---|---|---|---|---|
| （无） | — | — | 基础设施端点不进契约面/功能树，影响功能数 0（ktor REQ-2026-001/003 台账先例） | — |

## 5. 流程影响

无（docs/design/flow-function-map.md 尚未建立，本需求为纯工程设施）。

## 6. 风险与回滚

| 风险 | 影响面 | 缓解 | 回滚方式 |
|---|---|---|---|
| 5211 端口被占 | 本地启动失败 | 绑定失败即退出非零并报错，不重试不换端口 | 停占进程或改 .env 本地值（家族槽位不变） |
| 复刻漂移（与 saas-go 代码不同步） | 双仓配方分叉 | 四文件直接复制仅改模块路径；后续需求同样 saas 先行 lab 复刻 | diff 双仓同名文件核对 |
