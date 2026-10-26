# 功能清单（Function Tree） — lab-management-system-go

> **全体系唯一锚点。** 需求、流程、设计、测试都引用这里的 ID。
> 不在这里的 ID 是悬空引用，L5 门会拦。**改功能，先改这份表。**

## 编号规则

| 层级 | 名称 | 格式 | 含义 |
|---|---|---|---|
| 一级 | 功能模块 | `M0x` | 业务域边界，通常对应一级菜单（实际命名见各仓模块总览） |
| 二级 | 功能 | `M0x.F0y` | 一个完整业务步骤 / 独立闭环流程 / 数据管理页面 |
| 三级 | 功能子项 | `M0x.F0y.I0z` | 技术交付单元 / 权限挂载点。对应一个 API 接口、页面组件、图表区块或权限控制点 |

**硬规则**

1. 编号单调递增，永不复用。废弃改状态，不删行。
2. 子项编号必须以父级为前缀。
3. 一个子项 = 一个权限点。权限码即 ID，不另起一套编码。
4. 拆不出子项的功能 → 它其实是子项，往上并。子项超 20 个 → 它其实是模块，往下拆。

**状态**：`规划` | `开发中` | `已上线` | `已废弃`
**子项类型**：`页面` | `标签页` | `查询` | `按钮` | `报表` | `接口`

## 本仓角色

**契约 BASE 仓**（spec §3）。F+I 级：I 级为全家族定标源；消费仓 ⊆ BASE。
范围：M00 租户管理 / M01 认证 / M02 资源 / M03 试验过程 / M04 基础数据(码表) / M05 统计 / M06 检测能力。
多租户：一个用户关联多个租户，登录后选一个进去；租户真相源在 saas 身份平台。
M97/M98/M99/M96（infra/契约专属段）不进 BASE，由各消费仓自管。

## 模块总览

| 模块 ID | 模块名称 | 说明 | 状态 |
|---|---|---|---|
| M00 | 租户管理 | 当前用户关联租户列表、登录选租户、切换租户 | 已上线 |
| M01 | 认证管理 | 权限管理（RBAC/路由守卫/动态菜单）、认证（登录/SSO/JWT） | 开发中 |
| M02 | 资源管理 | 合同管理 | 开发中 |
| M03 | 试验过程管理 | 接样 → 任务分配 → 数据录入 → 报告审核 → 批准 → 发放 → 归档 | 开发中 |
| M04 | 基础数据 | 型号/规格/等级/牌号维护 | 已上线 |
| M05 | 数据统计 | 报告汇总表（按报告名称） | 开发中 |
| M06 | 检测能力 | 检测专项/项目/参数/标准/计算方法/技术要求/报告名称/参数界面 | 开发中 |


---

## M00 租户管理

| 功能 ID | 功能名称 | 说明 | 状态 |
|---|---|---|---|
| M00.F01 | 当前用户会话 | 当前用户信息 + 关联租户列表 + 当前选中租户（GET /auth/me） | 已上线 |
| M00.F02 | 登录选租户 | 登录后选择租户，换发携带 tenant_id claim 的 token（POST /auth/switch-tenant） | 已上线 |

### M00.F01 当前用户会话

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M00.F01.I01 | 当前会话 | 接口 | 前端+后端 | GET /api/auth/me：user + 关联租户列表 + currentTenantId（token tenant_id claim，缺省 TENANT-001） | 已上线 |


### M00.F02 登录选租户

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M00.F02.I01 | 选租户换发 | 接口 | 前端+后端 | POST /api/auth/switch-tenant：校验租户归属后换发携带 tenant_id claim 的 token | 已上线 |


---

## M01 认证管理

| 功能 ID | 功能名称 | 说明 | 状态 |
|---|---|---|---|
| M01.F04 | 权限管理 | RBAC 角色权限、路由守卫、权限指令、动态菜单（身份平台下发） | 开发中 |
| M01.F05 | 认证管理 | OAuth 2.0 授权码 SSO（client_secret 后端持）+ lab 自家 JWT 签发，saas token 不出 lab 后端 | 开发中 |

### M01.F04 权限管理

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M01.F04.I01 | 动态菜单 | 接口 | 前端+后端 | GET /api/auth/menus：按角色下发导航树（5 根节点，镜像 lab-msw） | 已上线 |
| M01.F04.I02 | 权限集 | 接口 | 前端+后端 | GET /api/auth/permissions：RBAC 权限串列表（admin 全量 11 项） | 已上线 |
| M01.F04.I03 | 路由守卫（未登录/无权限拦截） | 标签页 | 前端+后端 | 前端路由守卫 useRequireAuth 钩子（react/vue 仓实现）；本仓 BASE 登记仓内未挂 entry，react/vue 仓 useRequireAuth 5+ 处引用作为产品线 anchor | 开发中 |
| M01.F04.I04 | 动态菜单（lab 侧边栏）          | 标签页 | 前端+后端 | 前端 useSidebarContainer 钩子锚点（nextjs 仓 `<aside>` 实现），无后端端点对应 | 开发中 |


### M01.F05 认证管理

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M01.F05.I01 | 密码登录 | 接口 | 前端+后端 | POST /api/auth/login：用户名+密码校验，签发 access/refresh token + 租户列表。**2026-09-17 标记 已废弃**（SSO M01.F05.I02/I03 替代）；tsp op 暂留，9 仓 ServiceLogin/live_smoke/contract-test 13 注解迁移完成后一并删 op + LoginRequest/Response model | 已废弃 |
| M01.F05.I02 | SSO 跳转 | 接口 | 前端+后端 | GET /api/auth/sso/authorize：RFC 6749 §10.12 标准 state（前端生成、原样透传 saas 回显、前端比对），forward saas POST /api/v1/oauth/authorize 拿 code | 已上线 |
| M01.F05.I03 | SSO 回调 | 接口 | 前端+后端 | POST /api/auth/sso/callback:saas POST /api/v1/oauth/token 用一次性 code 换 token,再 /me/whoami + /me/tenants 拿 user,membership 信 saas;首次 SSO 按 email upsert 到 lab directory;state 校验在前端回跳比对 | 已上线 |
| M01.F05.I04 | 刷新 token | 接口 | 前端+后端 | POST /api/auth/refresh:lab refresh token 是 HS256 JWT(typ=refresh),内嵌 saas refresh token;调 saas POST /api/v1/oauth/token grantType=refresh_token 续,再签新 lab JWT | 已上线 |
| M01.F05.I05 | 登出 | 接口 | 前端+后端 | POST /api/auth/logout：无状态 JWT 服务端无 session，前端清存储 | 已上线 |
| M01.F05.I06 | 原生登录 | 接口 | 前端+后端 | POST /api/auth/native-login：非浏览器客户端（iOS/Android 原生表单）密码通道，REQ-2026-003 Q4-C 人裁（2026-09-28，非恢复已废弃 I01 web 密码端点）；REQ-2026-023 变更（2026-10-11 人裁方案 A）：校验委托 saas IdP 代理——请求体账密调 saas /api/v1/auth/login（clientId=lab-management），lab 自家 JWT 重签与 LoginResponse 契约形状不变，LAB_AUTH_DEV_PASSWORD 全家族退役 | 已上线 |


---

## M02 资源管理

| 功能 ID | 功能名称 | 说明 | 状态 |
|---|---|---|---|
| M02.F01 | 合同管理 | 合同 CRUD、工程信息维护 | 开发中 |

### M02.F01 合同管理

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M02.F01.I01 | 合同列表 | 接口 | 前端+后端 | GET /api/contracts?keyword=&status=：按 tenant 收口 + 2 过滤，返回 Contract[] | 已上线 |
| M02.F01.I02 | 合同详情 | 接口 | 前端+后端 | GET /api/contracts/{id}：404 if 不存在 | 已上线 |
| M02.F01.I03 | 创建合同 | 接口 | 前端+后端 | POST /api/contracts：code/clientUnit/projectName/constructionUnit/witnessUnit/witness 必填，status 默认 ACTIVE | 已上线 |
| M02.F01.I04 | 更新合同 | 接口 | 前端+后端 | PUT /api/contracts/{id}：PATCH 语义 | 开发中 |
| M02.F01.I05 | 删除合同 | 接口 | 前端+后端 | DELETE /api/contracts/{id}：204；如果有接样引用 FK RESTRICT 拒 | 已上线 |


---

## M03 试验过程管理

| 功能 ID | 功能名称 | 说明 | 状态 |
|---|---|---|---|
| M03.F01 | 接样管理 | 接样单 CRUD、报告类别关联、流程状态 | 开发中 |
| M03.F02 | 任务分配 | 接样提交后安排检测人员/计划日期，提交进入数据录入；任务字段挂 SampleReceipt | 开发中 |
| M03.F03 | 数据录入 | 样品检测数据录入 | 开发中 |
| M03.F05 | 报告审核 | 报告审核流程 | 开发中 |
| M03.F06 | 报告批准 | 报告批准流程 | 开发中 |
| M03.F07 | 报告发放 | 报告发放流程 | 开发中 |
| M03.F08 | 报告归档 | 报告归档流程 | 开发中 |
| M03.F09 | 接样单详情 | 接样单查看（接样信息+样品信息+检测数据） | 已上线 |

### M03.F01 接样管理

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M03.F01.I01 | 接样单列表 | 接口 | 前端+后端 | GET /api/receipts?contractId=&flowStatus=&keyword=：按 tenant 收口 + 3 过滤 | 已上线 |
| M03.F01.I02 | 接样单详情 | 接口 | 前端+后端 | GET /api/receipts/{id}：返回 SampleReceipt（含 flow_history）；**shared-with: M03.F05.I02 / M03.F06.I02 / M03.F07.I02 / M03.F08.I02 / M03.F09.I01** | 已上线 |
| M03.F01.I03 | 创建接样单 | 接口 | 前端+后端 | POST /api/receipts：contract_id FK 必存在；flow_status=receiving 起步；flow_history=[] | 已上线 |
| M03.F01.I04 | 更新接样单 | 接口 | 前端+后端 | PUT /api/receipts/{id}：PATCH 语义 | 已上线 |
| M03.F01.I05 | 删除接样单 | 接口 | 前端+后端 | DELETE /api/receipts/{id}：CASCADE 删除下属 samples | 开发中 |
| M03.F01.I06 | 接样单流程历史 | 接口 | 前端+后端 | GET /api/receipts/{id}/history：返回 FlowHistoryEntry[]（jsonb 展开为 List） | 已上线 |
| M03.F01.I07 | 接样单 ext 字段补录 | 接口 | 前端+后端 | PUT /api/samples/{id}/ext：UpdateSampleExtRequest{ext} 按当前类别 extFields 补录持久化到 Sample.ext | 已上线 |
| M03.F01.I08 | 接样-提交 | 接口 | 前端+后端 | POST /api/receipts/receiving/act，body.action={SUBMIT、RETURN、WITHDRAW} 三动作统一（2026-09-18：I09 退回/I10 撤回语义并入本行；RETURN 前置阶段退回无前置则 422、WITHDRAW 提交人撤回回到原阶段；7 阶段全 act 模式） | 已上线 |
| M03.F01.I09 | 接样-退回 | 接口 | 前端+后端 | 2026-09-18 标记 已废弃：语义并入 M03.F01.I08（act 端点以 body.action=RETURN 区分，无独立端点） | 已废弃 |
| M03.F01.I10 | 接样-撤回 | 接口 | 前端+后端 | 2026-09-18 标记 已废弃：语义并入 M03.F01.I08（act 端点以 body.action=WITHDRAW 区分，无独立端点） | 已废弃 |


### M03.F02 任务分配

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M03.F02.I01 | 任务分配 | 接口 | 前端+后端 | PUT /api/receipts/{id}/task：AssignTaskRequest 设 assigneeId/Name/plannedTestDate；非 receiving 阶段不自动 advance | 已上线 |
| M03.F02.I02 | 任务编辑（客户端视角 anchor） | 按钮 | 前端+后端 | 安排弹窗维护 assigneeName/assigneeId/plannedTestDate（前端弹窗 UI 动作，调 PUT /api/receipts/{id}/task 共 I01 端点；保留 ID 作为客户端视角 anchor，react/vue/nextjs 三仓 data-fn=I01）。**2026-09-17 标记 已废弃**（无独立 .tsp 端点、无 entry/tests 挂载；前端按钮 anchor 仍可用但不挂在 I02 ID 上） | 已废弃 |
| M03.F02.I03 | 任务取消（清空分配） | 按钮 | 前端+后端 | 清空 assignee/assigneeId/plannedTestDate（调 PUT /api/receipts/{id}/task 共 I01 端点），前端按钮仅 UI 层。**2026-09-17 标记 已废弃**（同 I02：无独立 .tsp 端点、无 entry/tests 挂载） | 已废弃 |
| M03.F02.I04 | 任务分配三态过滤器 | 按钮 | 仅前端 | 全部/未提交/已提交：按 flowStatus 过滤任务分配列表（前端过滤器，触发 GET /api/receipts?flowStatus=…） | 开发中 |
| M03.F02.I05 | 任务分配-提交 | 接口 | 前端+后端 | POST /api/receipts/assigning/act，body.action={SUBMIT、RETURN、WITHDRAW} 三动作统一（2026-09-18：I06 退回/I07 撤回语义并入本行；RETURN 退回到 receiving；7 阶段全 act 模式） | 已上线 |
| M03.F02.I06 | 任务分配-退回 | 接口 | 前端+后端 | 2026-09-18 标记 已废弃：语义并入 M03.F02.I05（act 端点以 body.action=RETURN 区分，无独立端点） | 已废弃 |
| M03.F02.I07 | 任务分配-撤回 | 接口 | 前端+后端 | 2026-09-18 标记 已废弃：语义并入 M03.F02.I05（act 端点以 body.action=WITHDRAW 区分，无独立端点） | 已废弃 |

### M03.F03 数据录入

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M03.F03.I01 | 样品列表 | 接口 | 前端+后端 | GET /api/samples?receiptId=&keyword=：tenant + 2 过滤 | 已上线 |
| M03.F03.I02 | 样品详情 | 接口 | 前端+后端 | GET /api/samples/{id} | 已上线 |
| M03.F03.I03 | 创建样品 | 接口 | 前端+后端 | POST /api/samples：receipt_id FK 必存在；ext 默认 {} | 已上线 |
| M03.F03.I04 | 更新样品 | 接口 | 前端+后端 | PUT /api/samples/{id}：PATCH 语义 | 已上线 |
| M03.F03.I05 | 删除样品 | 接口 | 前端+后端 | DELETE /api/samples/{id}：204 | 开发中 |
| M03.F03.I06 | 检测记录列表 | 接口 | 前端+后端 | GET /api/test-records?sampleId=&page=&pageSize=：tenant 收口 + sampleId 过滤 + 分页；data-fn=nextjs/react/vue 仓 test-records 页面 | 已上线 |
| M03.F03.I07 | 检测记录详情 | 接口 | 前端+后端 | GET /api/test-records/{id}：返回 TestRecord | 已上线 |
| M03.F03.I08 | 创建检测记录 | 接口 | 前端+后端 | POST /api/test-records：sampleId/parameterCode/requirement/result 必填；tenant 从 token claim 注入 | 已上线 |
| M03.F03.I09 | 更新检测记录 | 接口 | 前端+后端 | PUT /api/test-records/{id}：PATCH 语义，未传字段保留 | 已上线 |
| M03.F03.I10 | 删除检测记录 | 接口 | 前端+后端 | DELETE /api/test-records/{id}：204 if exists | 已上线 |
| M03.F03.I11 | 检测记录改判 | 接口 | 前端+后端 | PATCH /api/test-records/{id}/verdict：人工改判（2026-09-18 以 .tsp 为准修正动词，原误写 PUT；M03.F05/F06 报告流程可触发） | 已上线 |
| M03.F03.I12 | 数据录入-提交 | 接口 | 前端+后端 | POST /api/receipts/data-entry/act，body.action={SUBMIT、RETURN、WITHDRAW} 三动作统一（2026-09-18：I13 退回/I14 撤回语义并入本行；RETURN 退回到 assigning；7 阶段全 act 模式） | 已上线 |
| M03.F03.I13 | 数据录入-退回 | 接口 | 前端+后端 | 2026-09-18 标记 已废弃：语义并入 M03.F03.I12（act 端点以 body.action=RETURN 区分，无独立端点） | 已废弃 |
| M03.F03.I14 | 数据录入-撤回 | 接口 | 前端+后端 | 2026-09-18 标记 已废弃：语义并入 M03.F03.I12（act 端点以 body.action=WITHDRAW 区分，无独立端点） | 已废弃 |
| M03.F03.I15 | 数据录入三态过滤器 | 按钮 | 仅前端 | 全部/未提交/已提交：按 flowStatus 过滤数据录入列表（前端过滤器，触发 GET /api/receipts?flowStatus=…；镜像 M03.F02.I04） | 开发中 |

### M03.F05 报告审核

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M03.F05.I01 | 审核队列 | 接口 | 仅前端 | GET /api/receipts/review/queue；2026-09-17 共享端点删除，前端列表筛选替代 | 已废弃 |
| M03.F05.I02 | 报告审核-查看详情 | 按钮 | 仅前端 | 审核详情页打开动作（前端按钮调 GET /api/receipts/{id}，与 5 共享 I 共端点）。2026-09-17 交付列：前端+后端 → 仅前端 | 已上线 |
| M03.F05.I03 | 报告审核-通过/退回（act 共端点） | 接口 | 前端+后端 | POST /api/receipts/review/act：FlowActionRequest{action=SUBMIT 或 RETURN} 单条审核操作（body.action 区分）。2026-09-17 标记 已废弃（ID 层面）；2026-09-18 端点锚定收敛至 M03.F05.I07（act 端点 body.action 三动作统一），本行无独立端点 | 已废弃 |
| M03.F05.I04 | 报告审核三态过滤器 | 按钮 | 仅前端 | 全部/未提交/已提交：按 flowStatus 过滤报告审核列表（前端过滤器，触发 GET /api/receipts?flowStatus=…；nextjs 仓已实现 8 处 filterDataFn + 1 个 fnTest；react/vue 仓 page 存在但 prop 未挂） | 开发中 |
| M03.F05.I05 | 报告审核-批量提交 | 接口 | 仅后端 | POST /api/receipts/review/batch-submit：FlowActionRequest{ids[], action=SUBMIT} 批量推进 review→approval。2026-09-17 标记 已废弃，删 op（未实现） | 已废弃 |
| M03.F05.I06 | 报告审核-批量退回 | 接口 | 仅后端 | POST /api/receipts/review/batch-return：FlowActionRequest{ids[], action=RETURN} 批量退回 approval→review。2026-09-17 标记 已废弃，删 op（未实现） | 已废弃 |
| M03.F05.I07 | 报告审核-提交 | 接口 | 前端+后端 | POST /api/receipts/review/act，body.action={SUBMIT、RETURN、WITHDRAW} 三动作统一（2026-09-18：I08 退回/I09 撤回语义并入本行；4 阶段全 act 合并方案 B） | 已上线 |
| M03.F05.I08 | 报告审核-退回 | 接口 | 前端+后端 | 2026-09-18 标记 已废弃：语义并入 M03.F05.I07（act 端点以 body.action=RETURN 区分，无独立端点） | 已废弃 |
| M03.F05.I09 | 报告审核-撤回 | 接口 | 前端+后端 | 2026-09-18 标记 已废弃：语义并入 M03.F05.I07（act 端点以 body.action=WITHDRAW 区分，无独立端点） | 已废弃 |


### M03.F06 报告批准

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M03.F06.I01 | 报告阶段审批推进 | 接口 | 仅前端 | GET /api/receipts/approve/queue；2026-09-17 共享端点删除，前端列表筛选替代 | 已废弃 |
| M03.F06.I02 | 报告批准-查看详情 | 按钮 | 前端+后端 | 批准详情页打开动作（前端按钮调 GET /api/receipts/{id}，与 5 共享 I 共端点） | 已上线 |
| M03.F06.I03 | 报告批准-批准/退回（act 共端点） | 接口 | 前端+后端 | POST /api/receipts/approve/act：FlowActionRequest{action=SUBMIT 或 RETURN} 单条批准操作。2026-09-17 标记 已废弃（ID 层面）；2026-09-18 端点锚定收敛至 M03.F06.I05（act 端点 body.action 三动作统一），本行无独立端点 | 已废弃 |
| M03.F06.I04 | 报告批准三态过滤器 | 按钮 | 仅前端 | 全部/未提交/已提交：按 flowStatus 过滤报告批准列表（前端过滤器，触发 GET /api/receipts?flowStatus=…） | 开发中 |
| M03.F06.I05 | 报告批准-提交 | 接口 | 前端+后端 | POST /api/receipts/approve/act，body.action={SUBMIT、RETURN、WITHDRAW} 三动作统一（2026-09-18：I06 退回/I07 撤回语义并入本行；4 阶段全 act 合并方案 B） | 已上线 |
| M03.F06.I06 | 报告批准-退回 | 接口 | 前端+后端 | 2026-09-18 标记 已废弃：语义并入 M03.F06.I05（act 端点以 body.action=RETURN 区分，无独立端点） | 已废弃 |
| M03.F06.I07 | 报告批准-撤回 | 接口 | 前端+后端 | 2026-09-18 标记 已废弃：语义并入 M03.F06.I05（act 端点以 body.action=WITHDRAW 区分，无独立端点） | 已废弃 |


### M03.F07 报告发放

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M03.F07.I01 | 报告发放队列 | 接口 | 仅前端 | GET /api/receipts/issuance/queue；2026-09-17 共享端点删除，前端列表筛选替代 | 已废弃 |
| M03.F07.I02 | 报告发放-查看详情 | 按钮 | 前端+后端 | 发放详情页打开动作（前端按钮调 GET /api/receipts/{id}，与 5 共享 I 共端点） | 已上线 |
| M03.F07.I03 | 报告发放-发放/退回（act 共端点） | 接口 | 前端+后端 | POST /api/receipts/issuance/act：FlowActionRequest{action=SUBMIT 或 RETURN} 单条发放操作。2026-09-17 标记 已废弃（ID 层面）；2026-09-18 端点锚定收敛至 M03.F07.I05（act 端点 body.action 三动作统一），本行无独立端点 | 已废弃 |
| M03.F07.I04 | 报告发放三态过滤器 | 按钮 | 仅前端 | 全部/未提交/已提交：按 flowStatus 过滤报告发放列表（前端过滤器，触发 GET /api/receipts?flowStatus=…） | 开发中 |
| M03.F07.I05 | 报告发放-提交 | 接口 | 前端+后端 | POST /api/receipts/issuance/act，body.action={SUBMIT、RETURN、WITHDRAW} 三动作统一（2026-09-18：I06 退回/I07 撤回语义并入本行；4 阶段全 act 合并方案 B） | 已上线 |
| M03.F07.I06 | 报告发放-退回 | 接口 | 前端+后端 | 2026-09-18 标记 已废弃：语义并入 M03.F07.I05（act 端点以 body.action=RETURN 区分，无独立端点） | 已废弃 |
| M03.F07.I07 | 报告发放-撤回 | 接口 | 前端+后端 | 2026-09-18 标记 已废弃：语义并入 M03.F07.I05（act 端点以 body.action=WITHDRAW 区分，无独立端点） | 已废弃 |


### M03.F08 报告归档

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M03.F08.I01 | 报告归档队列 | 接口 | 仅前端 | GET /api/receipts/archived/queue；2026-09-17 共享端点删除，前端列表筛选替代 | 已废弃 |
| M03.F08.I02 | 报告归档-查看详情 | 按钮 | 前端+后端 | 归档详情页打开动作（前端按钮调 GET /api/receipts/{id}，与 5 共享 I 共端点） | 已上线 |
| M03.F08.I03 | 报告归档-归档/退回（act 共端点） | 接口 | 前端+后端 | POST /api/receipts/archived/act：FlowActionRequest{action=SUBMIT 或 RETURN} 单条归档操作。2026-09-17 标记 已废弃（ID 层面）；2026-09-18 端点锚定收敛至 M03.F08.I05（act 端点 body.action 三动作统一），本行无独立端点 | 已废弃 |
| M03.F08.I04 | 报告归档三态过滤器 | 按钮 | 仅前端 | 全部/未提交/已提交：按 flowStatus 过滤报告归档列表（前端过滤器，触发 GET /api/receipts?flowStatus=…） | 开发中 |
| M03.F08.I05 | 报告归档-提交 | 接口 | 前端+后端 | POST /api/receipts/archived/act，body.action={SUBMIT、RETURN、WITHDRAW} 三动作统一（2026-09-18：I06 退回/I07 撤回语义并入本行；4 阶段全 act 合并方案 B） | 已上线 |
| M03.F08.I06 | 报告归档-退回 | 接口 | 前端+后端 | 2026-09-18 标记 已废弃：语义并入 M03.F08.I05（act 端点以 body.action=RETURN 区分，无独立端点） | 已废弃 |
| M03.F08.I07 | 报告归档-撤回 | 接口 | 前端+后端 | 2026-09-18 标记 已废弃：语义并入 M03.F08.I05（act 端点以 body.action=WITHDRAW 区分，无独立端点） | 已废弃 |


### M03.F09 接样单详情

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M03.F09.I01 | 接样单详情聚合 | 按钮 | 前端+后端 | 客户端「打开详情」按钮（nextjs/react/vue 三仓真实按钮 + 测试断言），调 GET /api/receipts/{id}；与 M03.F01.I02 / F05.I02 / F06.I02 / F07.I02 / F08.I02 共端点 | 已上线 |
| M03.F09.I02 | 详情页 | 页面 | 仅前端 | 展示接样信息、样品列表、检测数据（三仓前端 Card/div 容器实现）；检测参数显示为「名称(单位)」、报告类别显示为报告简称。2026-09-17 标记 已废弃（ID 层面，前端 UI 不变；type=页面 无 .tsp 端点） | 已废弃 |
| M03.F09.I03 | 报告预览（详情页） | 按钮 | 仅前端 | 详情页标题栏按钮（三仓前端 + ReportPreviewModal 组件），复用 ReportPreviewModal 按 receipt.categoryCode 找模板 docx 渲染 | 已上线 |


---

## M04 基础数据

| 功能 ID | 功能名称 | 说明 | 状态 |
|---|---|---|---|
| M04.F06 | 型号维护 | InspectionModel 实体码表维护，列表按检测专项过滤 | 已上线 |
| M04.F07 | 规格维护 | InspectionSpec 实体码表维护，列表按检测专项过滤 | 已上线 |
| M04.F08 | 等级维护 | InspectionGrade 实体码表维护，列表按检测专项过滤 | 已上线 |
| M04.F09 | 牌号维护 | InspectionBrand 实体码表维护，列表按检测专项过滤 | 已上线 |

### M04.F06 型号维护

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M04.F06.I01 | 型号列表 | 接口 | 前端+后端 | GET /api/catalog/models?page=&pageSize=&inspectionObjectCode=&keyword=：`Page<InspectionModel>`，按 tenant 收口 + 2 过滤 | 已上线 |
| M04.F06.I02 | 创建型号 | 接口 | 前端+后端 | POST /api/catalog/models：body CreateCatalogEntryRequest（code/name 必填 + 可选 inspectionObjectCode/remark/sortOrder），返回 InspectionModel | 已上线 |
| M04.F06.I03 | 更新型号 | 接口 | 前端+后端 | PUT /api/catalog/models/{code}：body UpdateCatalogEntryRequest（PATCH 语义，未传字段保留），404 if 不存在 | 已上线 |
| M04.F06.I04 | 删除型号 | 接口 | 前端+后端 | DELETE /api/catalog/models/{code}：204；FK 被 technical_requirements.model 引用时 DB SET NULL | 已上线 |


### M04.F07 规格维护

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M04.F07.I01 | 规格列表 | 接口 | 前端+后端 | GET /api/catalog/specs?page=&pageSize=&inspectionObjectCode=&keyword=：`Page<InspectionSpec>` | 已上线 |
| M04.F07.I02 | 创建规格 | 接口 | 前端+后端 | POST /api/catalog/specs | 已上线 |
| M04.F07.I03 | 更新规格 | 接口 | 前端+后端 | PUT /api/catalog/specs/{code} | 已上线 |
| M04.F07.I04 | 删除规格 | 接口 | 前端+后端 | DELETE /api/catalog/specs/{code} | 已上线 |


### M04.F08 等级维护

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M04.F08.I01 | 等级列表 | 接口 | 前端+后端 | GET /api/catalog/grades?page=&pageSize=&inspectionObjectCode=&keyword=：`Page<InspectionGrade>` | 已上线 |
| M04.F08.I02 | 创建等级 | 接口 | 前端+后端 | POST /api/catalog/grades | 已上线 |
| M04.F08.I03 | 更新等级 | 接口 | 前端+后端 | PUT /api/catalog/grades/{code} | 已上线 |
| M04.F08.I04 | 删除等级 | 接口 | 前端+后端 | DELETE /api/catalog/grades/{code} | 已上线 |


### M04.F09 牌号维护

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M04.F09.I01 | 牌号列表 | 接口 | 前端+后端 | GET /api/catalog/brands?page=&pageSize=&inspectionObjectCode=&keyword=：`Page<InspectionBrand>` | 已上线 |
| M04.F09.I02 | 创建牌号 | 接口 | 前端+后端 | POST /api/catalog/brands | 已上线 |
| M04.F09.I03 | 更新牌号 | 接口 | 前端+后端 | PUT /api/catalog/brands/{code} | 已上线 |
| M04.F09.I04 | 删除牌号 | 接口 | 前端+后端 | DELETE /api/catalog/brands/{code} | 已上线 |


---

## M05 数据统计

| 功能 ID | 功能名称 | 说明 | 状态 |
|---|---|---|---|
| M05.F01 | 报告汇总 | 按报告类别输出试验报告汇总表 | 规划 |

### M05.F01 报告汇总

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M05.F01.I01 | 报告汇总 | 查询 | 前端+后端 | GET /api/summary?categoryCode=&dateFrom=&dateTo=：categoryCode=ALL 不过滤，否则按报告类别过滤当前租户接样单；输出 SummaryData{summaryName, columns(6), rows}；data-fn=nextjs/react/vue 仓 SummaryPage | 规划 |
| M05.F01.I02 | 仪表盘容器 | 页面 | 仅前端 | SummaryPage 最外层 layout 锚点，包裹 I03/I04/I05 三区块 | 规划 |
| M05.F01.I03 | 核心指标卡 | 查询 | 前端+后端 | 今日试验总数 + 检测合格率（按材料类型 concrete/rebar/sand）+ 报告产出量（已生成/已签发/待审核）；GET /api/summary/stats 扩展 todayTestCount/qualifiedRateByMaterial/reportOutputByStatus | 规划 |
| M05.F01.I04 | 任务状态漏斗 | 报表 | 前端+后端 | 6 段实时计数：待取样→已收样→试验中→报告编制→待审核→已签发；GET /api/summary/stats 扩展 funnelByStage:{pending_collect, received, testing, reporting, reviewing, issued} | 规划 |
| M05.F01.I05 | 见证取样跟踪 | 报表 | 前端+后端 | 见证率（合同需见证的接样单中已完成见证的比例）+ 见证到位情况明细；GET /api/summary/stats 扩展 witnessStats:{requireWitness, witnessed, witnessRate, details[]} | 规划 |
| M05.F01.I06 | 仪表盘统计基础端点 | 查询 | 前端+后端 | GET /api/summary/stats 基础字段：contractCount/receiptCount/sampleCount + 报告状态 3 桶（draft=receiving+task+data_entry；reviewing=review+approval；issued=issuance+archived）+ pendingTaskCount。ADR-0033 阶段二从后端仓 M05.F02.I01 下沉 BASE（四方已上线的事实先行入树，I03/I04 是本端点的扩展字段） | 已上线 |


---

## M06 检测能力

| 功能 ID | 功能名称 | 说明 | 状态 |
|---|---|---|---|
| M06.F01 | 检测专项 | InspectionSpecialty CRUD（检测能力字典根） | 开发中 |
| M06.F02 | 检测项目 | InspectionObject CRUD + 专项/参数关联 | 开发中 |
| M06.F03 | 检测参数 | InspectionParameter CRUD + 标准/参数关联 | 开发中 |
| M06.F04 | 检测标准 | InspectionStandard CRUD（含状态：active/superseded/draft） | 已上线 |
| M06.F05 | 计算方法 | CalculationMethod 维护（复合主键，算法类型 + 公式） | 已上线 |
| M06.F06 | 技术要求 | TechnicalRequirement 维护，按四维度匹配；brand/model/grade/spec 改为 FK 引用实体 | 已上线 |
| M06.F07 | 报告名称 | InspectionReportName CRUD + extFields 模板 + 关联标准/参数 | 已上线 |
| M06.F08 | 参数界面 | ParamInterface 维护 + 参数↔界面 link | 已上线 |

### M06.F01 检测专项

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M06.F01.I01 | 专项列表 | 接口 | 前端+后端 | GET /api/inspection/specialties?page=&pageSize=&keyword=：`Page<InspectionSpecialty>`，按 code/name 模糊匹配，平台级 | 已上线 |
| M06.F01.I02 | 创建专项 | 接口 | 前端+后端 | POST /api/inspection/specialties：code/officialNo/name 必填；isOfficial/enabled 默认 true；sortOrder 默认 0 | 已上线 |
| M06.F01.I03 | 更新专项 | 接口 | 前端+后端 | PUT /api/inspection/specialties/{code}：PATCH 语义，未传字段保留 | 已上线 |
| M06.F01.I04 | 删除专项 | 接口 | 前端+后端 | DELETE /api/inspection/specialties/{code}：204 if exists，否则 404 | 已上线 |
| M06.F01.I05 | 项目↔标准 link | 接口 | 前端+后端 | POST /api/inspection/links/object-standard：建立 object→standard(role) 关联，role 必填（TESTING/JUDGMENT） | 已上线 |
| M06.F01.I06 | 项目↔标准 unlink | 接口 | 前端+后端 | DELETE /api/inspection/links/object-standard：404 if 不存在 | 已上线 |
| M06.F01.I07 | 项目↔标准 列表 | 接口 | 前端+后端 | GET /api/inspection/links/object-standard?inspectionObjectCode=&role=：`Page<ObjectStandardLink>`，按 objectCode/role 过滤 | 开发中 |


### M06.F02 检测项目

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M06.F02.I01 | 项目列表 | 接口 | 前端+后端 | GET /api/inspection/objects?page=&pageSize=&inspectionSpecialtyCode=&keyword=：`Page<InspectionObject>`，按 code/name 模糊 + 专项过滤 | 已上线 |
| M06.F02.I02 | 创建项目 | 接口 | 前端+后端 | POST /api/inspection/objects：code/inspectionSpecialtyCode/sourceProjectNo/sourceProjectName/name 必填；isOptionalForQualification 默认 false；isOfficial/enabled 默认 true | 已上线 |
| M06.F02.I03 | 更新项目 | 接口 | 前端+后端 | PUT /api/inspection/objects/{code}：PATCH 语义 | 已上线 |
| M06.F02.I04 | 删除项目 | 接口 | 前端+后端 | DELETE /api/inspection/objects/{code}：204 if exists | 已上线 |
| M06.F02.I05 | 专项↔项目 link | 接口 | 前端+后端 | POST /api/inspection/links/specialty-object：建立 specialty→object 关联，remark 可选 | 已上线 |
| M06.F02.I06 | 专项↔项目 unlink | 接口 | 前端+后端 | DELETE /api/inspection/links/specialty-object：404 if 不存在 | 已上线 |
| M06.F02.I07 | 项目↔参数 link | 接口 | 前端+后端 | POST /api/inspection/links/object-parameter：建立 object→parameter 关联，qualificationLevel 默认 QUALIFIED，sourcePage/remark 可选 | 已上线 |
| M06.F02.I08 | 项目↔参数 unlink | 接口 | 前端+后端 | DELETE /api/inspection/links/object-parameter：404 if 不存在 | 已上线 |
| M06.F02.I09 | 专项↔项目 列表 | 接口 | 前端+后端 | GET /api/inspection/links/specialty-object?inspectionSpecialtyCode=：`Page<SpecialtyObjectLink>`，按专项 code 过滤 | 开发中 |
| M06.F02.I10 | 项目↔参数 列表 | 接口 | 前端+后端 | GET /api/inspection/links/object-parameter?inspectionObjectCode=&inspectionParameterCode=：`Page<ObjectParameterLink>`，按 objectCode/parameterCode 过滤 | 开发中 |


### M06.F03 检测参数

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M06.F03.I01 | 参数列表 | 接口 | 前端+后端 | GET /api/inspection/parameters?page=&pageSize=&keyword=&sourceType=：`Page<InspectionParameter>`，按 code/name 模糊 + sourceType 过滤（official/custom） | 已上线 |
| M06.F03.I02 | 创建参数 | 接口 | 前端+后端 | POST /api/inspection/parameters：code/name/rawName/canonicalName 必填；sourceType 默认 OFFICIAL；aliases 默认 [] | 已上线 |
| M06.F03.I03 | 更新参数 | 接口 | 前端+后端 | PUT /api/inspection/parameters/{code}：PATCH 语义；aliases 传则整体替换 | 已上线 |
| M06.F03.I04 | 删除参数 | 接口 | 前端+后端 | DELETE /api/inspection/parameters/{code}：204 if exists | 已上线 |
| M06.F03.I05 | 标准↔参数 link | 接口 | 前端+后端 | POST /api/inspection/links/standard-parameter：建立 standard→parameter 关联 | 已上线 |
| M06.F03.I06 | 标准↔参数 unlink | 接口 | 前端+后端 | DELETE /api/inspection/links/standard-parameter：404 if 不存在 | 已上线 |
| M06.F03.I07 | 参数↔界面 unlink | 接口 | 前端+后端 | DELETE /api/param-interfaces/links：404 if 不存在 | 已上线 |
| M06.F03.I08 | 标准↔参数 列表 | 接口 | 前端+后端 | GET /api/inspection/links/standard-parameter?inspectionStandardCode=&inspectionParameterCode=：`Page<StandardParameterLink>`，按 standardCode/parameterCode 过滤 | 开发中 |


### M06.F04 检测标准

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M06.F04.I01 | 标准列表 | 接口 | 前端+后端 | GET /api/inspection/standards?page=&pageSize=&keyword=&status=：`Page<InspectionStandard>`，按 code/name 模糊 + status 过滤（active/superseded/draft） | 已上线 |
| M06.F04.I02 | 创建标准 | 接口 | 前端+后端 | POST /api/inspection/standards：code/name 必填；status 默认 ACTIVE | 已上线 |
| M06.F04.I03 | 更新标准 | 接口 | 前端+后端 | PUT /api/inspection/standards/{code}：PATCH 语义 | 已上线 |
| M06.F04.I04 | 删除标准 | 接口 | 前端+后端 | DELETE /api/inspection/standards/{code}：204 if exists | 已上线 |
| M06.F04.I05 | 项目↔报告名称 unlink | 接口 | 前端+后端 | DELETE /api/report-names/links/object：404 if 不存在 | 已上线 |
| M06.F04.I06 | 报告名称↔参数 unlink | 接口 | 前端+后端 | DELETE /api/report-names/links/parameter：404 if 不存在 | 已上线 |
| M06.F04.I07 | 报告名称↔标准 unlink | 接口 | 前端+后端 | DELETE /api/report-names/links/standard：404 if 不存在 | 已上线 |


### M06.F05 计算方法

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M06.F05.I01 | 计算方法列表 | 接口 | 前端+后端 | GET /api/calculation-methods?inspectionObjectCode=&inspectionParameterCode=：平台级（无 tenant 过滤） | 已上线 |
| M06.F05.I02 | 计算方法详情 | 接口 | 前端+后端 | GET /api/calculation-methods/{inspectionObjectCode}/{inspectionParameterCode}：复合主键 | 已上线 |
| M06.F05.I03 | 创建计算方法 | 接口 | 前端+后端 | POST /api/calculation-methods：body CreateCalculationMethodRequest，algorithmType 默认 MANUAL、specimenCount 默认 1 | 已上线 |
| M06.F05.I04 | 更新计算方法 | 接口 | 前端+后端 | PUT /api/calculation-methods/{...}：PATCH 语义 | 已上线 |
| M06.F05.I05 | 删除计算方法 | 接口 | 前端+后端 | DELETE /api/calculation-methods/{...}：204 | 已上线 |


### M06.F06 技术要求

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M06.F06.I01 | 技术要求列表 | 接口 | 前端+后端 | GET /api/technical-requirements?inspectionObjectCode=&inspectionParameterCode=&judgmentStandardCode=&verificationStatus=：tenant 收口 + 4 过滤 | 已上线 |
| M06.F06.I02 | 技术要求详情 | 接口 | 前端+后端 | GET /api/technical-requirements/{object}/{param}/{standard}：复合三键 | 已上线 |
| M06.F06.I03 | 创建技术要求 | 接口 | 前端+后端 | POST /api/technical-requirements：tenant 从 token claim 注入；默认值 numeric/≥/manual/draft | 已上线 |
| M06.F06.I04 | 更新技术要求 | 接口 | 前端+后端 | PUT /api/technical-requirements/{...}：PATCH 语义 | 已上线 |
| M06.F06.I05 | 删除技术要求 | 接口 | 前端+后端 | DELETE /api/technical-requirements/{...}：204 | 已上线 |


### M06.F07 报告名称

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M06.F07.I01 | 报告名称列表 | 接口 | 前端+后端 | GET /api/report-names?page=&pageSize=&keyword=：`Page<InspectionReportName>`，按 code/name 模糊 | 已上线 |
| M06.F07.I02 | 报告名称详情 | 接口 | 前端+后端 | GET /api/report-names/{code}：返回 InspectionReportName（含 extFields 反序列化的 `List<ExtFieldDef>`） | 已上线 |
| M06.F07.I03 | 创建报告名称 | 接口 | 前端+后端 | POST /api/report-names：code/name 必填；extFields 默认 [] | 已上线 |
| M06.F07.I04 | 更新报告名称 | 接口 | 前端+后端 | PUT /api/report-names/{code}：PATCH 语义 | 已上线 |
| M06.F07.I05 | 删除报告名称 | 接口 | 前端+后端 | DELETE /api/report-names/{code}：204 if exists | 已上线 |
| M06.F07.I06 | 项目↔报告名称 link | 接口 | 前端+后端 | POST /api/report-names/links/object：建立 object→report-name 关联，remark 可选 | 已上线 |
| M06.F07.I07 | 报告名称↔标准 link | 接口 | 前端+后端 | POST /api/report-names/links/standard：建立 report-name→standard(role) 关联，role 必填 | 已上线 |
| M06.F07.I08 | 报告名称↔参数 link | 接口 | 前端+后端 | POST /api/report-names/links/parameter：建立 report-name→parameter 关联 | 已上线 |


### M06.F08 参数界面

| 子项 ID | 名称 | 类型 | 交付 | 说明 | 状态 |
|---|---|---|---|---|---|
| M06.F08.I01 | 参数界面列表 | 接口 | 前端+后端 | GET /api/param-interfaces?page=&pageSize=&keyword=：`Page<ParamInterface>`，按 code/name 模糊 | 已上线 |
| M06.F08.I02 | 参数界面详情 | 接口 | 前端+后端 | GET /api/param-interfaces/{code}：返回 ParamInterface（含 config 反序列化的 Map<String,Object>） | 已上线 |
| M06.F08.I03 | 创建参数界面 | 接口 | 前端+后端 | POST /api/param-interfaces：code/componentPath 必填；config 默认 {} | 已上线 |
| M06.F08.I04 | 更新参数界面 | 接口 | 前端+后端 | PUT /api/param-interfaces/{code}：PATCH 语义 | 已上线 |
| M06.F08.I05 | 删除参数界面 | 接口 | 前端+后端 | DELETE /api/param-interfaces/{code}：204 if exists | 已上线 |
| M06.F08.I06 | 参数↔界面 link | 接口 | 前端+后端 | POST /api/param-interfaces/links：建立 parameter→interface 关联，reportNameCode/config 可选（config 走 jsonb） | 已上线 |