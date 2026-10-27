# REQ-2026-004 lab-go 全量业务切片

| 项 | 值 |
|---|---|
| 提出人 | zcqiand |
| 提出日期 | 2026-10-27 |
| 优先级 | P1 |
| 状态 | 已交付（2026-10-27，v0.2.0-20261027） |
| 关联 ADR | — |
| 先例 | REQ-2026-003（saas-go 全量切片，v0.2.0-20261027，配方已验） |

## 1. 需求描述

> 用户原话：「saas+lab go版本，继续所有需求」（saas-go 已随 REQ-2026-003 交付，本单是 lab-go 独立批）

理解：lab-go 仓从骨架态（仅 /health）推到全量业务面，与 rails/springboot/fastapi 同门对齐——shared 契约 64 path / 120 op / 12 tag 全实现、全量 route-parity、DB-First entity 生成链（24 表）、lab CT 注册 go:5211、gate 全绿。配方照搬 REQ-2026-003 已验链（oapi-codegen std-http-server + GORM），语义源换成 **lab rails 同门**。

### 与 saas-go 的三分叉（本单核心风险面）

1. **auth 代理链**（REQ-2026-023 同款）：账密校验代理 saas IdP（clientId=`lab-management`），saas 4xx→401 INVALID_CREDENTIALS 不透传、5xx/不可达→502 SAAS_UNAVAILABLE；lab 自签 JWT **无 aud claim**（家族有意差异），refresh 内嵌 saas_refresh_token 7d rotate-once；菜单/用户走服务账号快照（UserDirectory/SaasMenuMapper 镜像）
2. **域模型**：无 tenant/user/client 实体——样品全流程（sample_receipt 44 列七段 flow receiving→archived、sample 22 列含 ext jsonb、test_record、contract）+ 检测能力字典（inspection_* 20 表，字典主键=业务 code 如 SP01 非 UUID）；单租户，租户是 JWT claim + 种子字面量 TENANT-001/002/003
3. **种子**：真源=shared `seeds/`（26 json，lab_dev 23 表 4377 行快照）；测试库 `lab_test`（127.0.0.1:54321）

### 澄清记录

| 疑问 | 澄清结论 | 澄清人 | 日期 |
|---|---|---|---|
| 边界 | = rails 同门全量面（120 op parity）+ lab CT go 注册；不含 PKCE 类家族裁定项 | Claude（inline 执行，「继续」授权） | 2026-10-27 |

## 2. 验收标准

| 编号 | 场景（给定） | 操作（当） | 面预期（则） |
|---|---|---|---|
| AC-1 | shared openapi 120 op | 跑 go 仓 route-parity 契约门 | ✅ 全部路由挂接、无契约外路由（allowlist 仅 GET /health） |
| AC-2 | 本机 PG lab_dev | 跑 strict 幂等门 | ✅ 双脚本复放字节幂等，生成区与 HEAD 一致（check_codegen_idempotent.py --strict exit 0） |
| AC-3 | lab_test 种子在位 | 跑 go test ./... | ✅ Feature 链全绿且事务回滚不污染（suite gate L4 exit 0；rails gate L4 交叉仍绿） |
| AC-4 | 本机起 lab-go :5211 连 lab_dev + saas IdP :5101 | native-login→me→菜单冒烟 | ✅ live 探针全 200；lab CT live 2-way（springboot,go）收敛 |

## 3. 任务拆解

见 suite 仓 `docs/superpowers/plans/2026-10-27-go-lab-full-slice.md`（Task 1-12）。

## 4. 功能影响

| 功能 ID | 功能名称 | 影响类型 | 说明 | 关联任务 |
|---|---|---|---|---|
| M00-M06 全部活跃 I 项（lab shared 树 160 项） | 租户/认证/资源/试验过程/基础数据/统计/检测能力 | 新增 | lab-go 侧首交付，I 级随实现逐段 mirror 登记 | Task 2-11 |

## 5. 人工验收记录

人工验收通过（验收人：用户，2026-10-27 明示「验收通过」）。范围=本批 7 条已实现 I 级：M02.F01.I04、M03.F01.I05、M03.F03.I05、M06.F01.I07、M06.F02.I09、M06.F02.I10、M06.F03.I08（handler/`fn:` 测试注解/CT live 394/394 trace 全实证，rails 同 ID 均已上线）。机器面佐证：lab CT live 2-way（springboot,go）394/394 exit 0；live 探针 :5211 native-login→me→menus→samples→stats 全 200。M01.F04.I03/I04 与 6 条前端过滤器行为前端仓交付面（nextjs 已上线），后端仓镜像状态维持开发中，不在本批。
