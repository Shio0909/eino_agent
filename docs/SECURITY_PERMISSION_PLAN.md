# 安全与权限实施方案（必做）

## 目标

把项目从“可用 Demo”升级为“可上线的后端系统雏形”：

- 接口鉴权（谁可以访问）
- 权限控制（可以做什么）
- 租户隔离（访问谁的数据）
- 审计日志（做过什么）

## 最小可交付（2~3 周）

## 第一阶段：鉴权与基础权限（P0）

1. 增加登录与 JWT 中间件
   - Access Token（短期）+ Refresh Token（长期）
   - 中间件注入 `user_id`、`tenant_id` 到上下文

2. 路由分级
   - 公共接口：`/health`
   - 普通用户：chat / kb 读写 / session
   - 管理员：settings 修改、模型管理

3. 禁止匿名修改配置
   - `PUT /api/v1/settings` 必须管理员角色

## 第二阶段：租户隔离（P0）

1. 去除硬编码 tenant_id
2. Repository 层所有查询都带 tenant 条件
3. 新建资源强制绑定 tenant_id
4. 增加越权测试（A 租户不能读 B 租户数据）

## 第三阶段：审计与安全基线（P1）

1. 审计日志
   - 记录：谁、何时、做了什么、对象 ID、结果
   - 重点接口：settings、knowledge-base、document、model

2. 安全基线
   - CORS 白名单化
   - 请求体大小限制
   - 关键接口限流
   - 敏感字段脱敏（API Key）

3. 密钥治理
   - 默认不把明文 API Key 持久化到 yaml
   - 采用 env 引用或加密存储

## 里程碑验收

通过以下清单即视为完成：

- 未携带 JWT 无法访问受保护接口
- 非管理员无法修改 settings
- 跨 tenant 访问被拦截
- 所有关键写操作有审计日志
- 安全检查脚本通过（CORS、限流、敏感字段）

## 当前已落地（2026-02-21）

- 鉴权与角色：`/api/v1/auth/login`、`/api/v1/auth/me`、`AuthRequired`、`RequireRole`
- 角色分级：管理员可修改 settings；普通用户可访问会话/知识库等业务接口
- 租户隔离：知识库、文档、会话、聊天（携带 session_id）已做 tenant/owner 校验
- 历史兼容路由加固：`/api/chat`、`/api/chat/stream` 也纳入鉴权
- 审计日志：关键写操作落地到 `data/audit/audit.log`（JSON Lines）

## 尚未完成（建议下一批）

- Access/Refresh 双 token 体系（目前为单 access token）
- CORS 白名单、限流、请求体大小限制
- API Key 持久化加密或 env 引用治理

## 简历可用描述模板

- 设计并落地 JWT + RBAC + 租户隔离模型，覆盖配置与知识库核心接口
- 建立审计日志链路，支持关键操作追踪与问题回溯
- 完成安全基线加固（CORS 白名单、限流、敏感配置治理）
