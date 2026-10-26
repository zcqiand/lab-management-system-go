# REQ-2026-002 进 prod deploy 链（ktor/laravel 先例五件套；saas-go 复刻）

| 项 | 值 |
|---|---|
| 提出人 | zcqiand（人裁 2026-10-26「go 双仓进 prod deploy 链」） |
| 提出日期 | 2026-10-26 |
| 优先级 | P1 |
| 状态 | 已验收 |
| 关联 ADR | ADR-0018（host=container 单层端口）；suite-hard-rules §1（禁 env 兜底） |

## 1. 需求描述

用户原话：「进 prod deploy 链 (Recommended)」（AskUserQuestion 三选一，go 双仓下一步）。

理解：saas-go（REQ-2026-002 先行）复刻到本仓——X13=5211，deploy 五件套
（Dockerfile + ci.yml + deploy 脚本 + nginx vhost 模板 + setup-vps.sh），
tag push → CI → VPS → `https://lab-go.xiangru.uk` 上线。

### 澄清记录

| 疑问 | 澄清结论 | 澄清人 | 日期 |
|---|---|---|---|
| （复刻仓）与 saas-go 的差异面 | 仅端口 5211/域名 lab-go.xiangru.uk/仓名；其余逐字同构——双 go 仓 env 契约同形（只有 SERVER_PORT），无 PG/JWT | Claude | 2026-10-26 |

## 2. 验收标准

| 编号 | 场景（给定） | 操作（当） | 预期（则） |
|---|---|---|---|
| AC-1 | 仓内 deploy 五件套齐全 | `ls deploy/ Dockerfile .github/workflows/ci.yml` | 五件在盘，`sh -n` 语法过 |
| AC-2 | push tag `v*` | CI test job | gofmt/vet/build/test + 镜像装配冒烟（build+run+/health 200）全绿 |
| AC-3 | CI test 绿 | deploy job | 镜像 push → VPS 起容器 → `/health` 120s 内 200 |
| AC-4 | 部署完成 | `curl https://lab-go.xiangru.uk/health` | `{"ok":true,"mode":"go"}` |
| AC-5 | 本地门禁 | `python scripts/gate.py -p lab-management-system-go` | L0-L5 全绿 |

## 3. 任务拆解

| 任务 ID | 任务描述 | 类型 | 负责人 | 预估 | 状态 |
|---|---|---|---|---|---|
| T-1 | 绑定地址 `:%d`（saas-go T-1 同款，容器可达） | 代码 | Claude | 5min | 已完成 |
| T-2 | 五件套复刻（copy+sed：5111→5211、saas-go→lab-go、仓名） | 基础设施 | Claude | 20min | 已完成 |
| T-3 | GitHub secrets/vars 补配 + tag 首航（saas-go 绿后串行推） + live 探针收账 | 运维 | Claude | 15min | 已完成 |

## 4. 功能影响（需求与功能对齐的唯一位置）

基础设施需求：影响功能数 **0**（同 saas-go REQ-2026-002）。

| 功能 ID | 功能名称 | 影响类型 | 说明 | 关联任务 |
|---|---|---|---|---|
| — | — | — | 基础设施端点 `/health` 不挂 fn ID；绑定地址为装配参数非功能语义 | — |

## 5. 流程影响

无。

## 6. 风险与回滚

| 风险 | 影响面 | 缓解 | 回滚方式 |
|---|---|---|---|
| 双仓并发 deploy 撞 VPS prune | 两仓 deploy job | tag 串行推（saas 绿后再 lab） | rerun failed job |
| 复刻 sed 残留 saas 字样 | 部署面 | grep 逐文件核对（sed 链 `saas\.YOUR_DOMAIN` 通配键属家族模板固定件保留） | 同 commit 修 |
