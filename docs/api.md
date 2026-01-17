# Sub2API 接口文档

> 说明：该文档按模块分组，基于当前路由注册与 Handler/DTO 定义整理。
> 本文档尚未补全所有请求/响应示例，仅列出可确认的字段与描述。

## 通用

- GET `/health`
  - 说明：健康检查。
  - 响应：`{ "status": "ok" }`

- POST `/api/event_logging/batch`
  - 说明：Claude Code 遥测日志（服务端直接返回 200）。

- GET `/setup/status`
  - 说明：常规模式下返回 `needs_setup=false`。

## Auth

- POST `/api/v1/auth/register`
  - 说明：用户注册。
  - 请求：`RegisterRequest`
    - `email` string, 必填, 邮箱
    - `password` string, 必填, 最少 6
    - `verify_code` string, 可选
    - `turnstile_token` string, 可选
    - `promo_code` string, 可选
  - 响应：`AuthResponse`
    - `access_token` string
    - `token_type` string
    - `user` dto.User

- POST `/api/v1/auth/login`
  - 说明：用户登录。
  - 请求：`LoginRequest`
    - `email` string, 必填
    - `password` string, 必填
    - `turnstile_token` string, 可选
  - 响应：`AuthResponse`

- POST `/api/v1/auth/send-verify-code`
  - 说明：发送邮箱验证码。
  - 请求：`SendVerifyCodeRequest`
    - `email` string, 必填
    - `turnstile_token` string, 可选
  - 响应：`SendVerifyCodeResponse`
    - `message` string
    - `countdown` number

- POST `/api/v1/auth/validate-promo-code`
  - 说明：校验优惠码。
  - 请求：`ValidatePromoCodeRequest`
    - `code` string, 必填
  - 响应：`ValidatePromoCodeResponse`
    - `valid` boolean
    - `bonus_amount` number
    - `error_code` string
    - `message` string

- GET `/api/v1/auth/oauth/linuxdo/start`
  - 说明：LinuxDo OAuth 开始。

- GET `/api/v1/auth/oauth/linuxdo/callback`
  - 说明：LinuxDo OAuth 回调。

- GET `/api/v1/settings/public`
  - 说明：公开设置。
  - 响应：`dto.PublicSettings`

- GET `/api/v1/auth/me`
  - 说明：获取当前用户。
  - 认证：JWT
  - 响应：`dto.User` + `run_mode`

## User

- GET `/api/v1/user/profile`
  - 说明：获取个人资料。
  - 认证：JWT
  - 响应：`dto.User`

- PUT `/api/v1/user/password`
  - 说明：修改密码。
  - 认证：JWT
  - 请求：`ChangePasswordRequest`
    - `old_password` string
    - `new_password` string

- PUT `/api/v1/user`
  - 说明：更新个人资料。
  - 认证：JWT
  - 请求：`UpdateProfileRequest`
    - `username` string

- GET `/api/v1/keys`
  - 说明：API Key 列表。
  - 认证：JWT

- GET `/api/v1/keys/:id`
  - 说明：API Key 详情。
  - 认证：JWT

- POST `/api/v1/keys`
  - 说明：创建 API Key。
  - 认证：JWT
  - 请求：`CreateAPIKeyRequest`
    - `name` string
    - `group_id` number
    - `custom_key` string
    - `ip_whitelist` string[]
    - `ip_blacklist` string[]

- PUT `/api/v1/keys/:id`
  - 说明：更新 API Key。
  - 认证：JWT
  - 请求：`UpdateAPIKeyRequest`
    - `name` string
    - `group_id` number
    - `status` string
    - `ip_whitelist` string[]
    - `ip_blacklist` string[]

- DELETE `/api/v1/keys/:id`
  - 说明：删除 API Key。
  - 认证：JWT

- GET `/api/v1/groups/available`
  - 说明：获取用户可用分组。
  - 认证：JWT

- GET `/api/v1/usage`
  - 说明：用量记录列表。
  - 认证：JWT

- GET `/api/v1/usage/:id`
  - 说明：用量记录详情。
  - 认证：JWT

- GET `/api/v1/usage/stats`
  - 说明：用量统计。
  - 认证：JWT

- GET `/api/v1/usage/dashboard/stats`
  - 说明：用户仪表盘统计。
  - 认证：JWT

- GET `/api/v1/usage/dashboard/trend`
  - 说明：用户用量趋势。
  - 认证：JWT

- GET `/api/v1/usage/dashboard/models`
  - 说明：用户模型用量统计。
  - 认证：JWT

- POST `/api/v1/usage/dashboard/api-keys-usage`
  - 说明：批量 API Key 用量统计。
  - 认证：JWT
  - 请求：`BatchAPIKeysUsageRequest`
    - `api_key_ids` number[]

- POST `/api/v1/redeem`
  - 说明：兑换卡密。
  - 认证：JWT
  - 请求：`RedeemRequest`
    - `code` string

- GET `/api/v1/redeem/history`
  - 说明：兑换记录。
  - 认证：JWT

- GET `/api/v1/subscriptions`
  - 说明：订阅列表。
  - 认证：JWT

- GET `/api/v1/subscriptions/active`
  - 说明：活跃订阅。
  - 认证：JWT

- GET `/api/v1/subscriptions/progress`
  - 说明：订阅进度。
  - 认证：JWT

- GET `/api/v1/subscriptions/summary`
  - 说明：订阅摘要。
  - 认证：JWT

## Admin

- GET `/api/v1/admin/dashboard/stats`
  - 说明：管理后台统计总览。

- GET `/api/v1/admin/dashboard/realtime`
  - 说明：实时指标。

- GET `/api/v1/admin/dashboard/trend`
  - 说明：用量趋势。

- GET `/api/v1/admin/dashboard/models`
  - 说明：模型统计。

- GET `/api/v1/admin/dashboard/api-keys-trend`
  - 说明：API Key 用量趋势。

- GET `/api/v1/admin/dashboard/users-trend`
  - 说明：用户用量趋势。

- POST `/api/v1/admin/dashboard/users-usage`
  - 说明：批量用户用量。
  - 请求：`BatchUsersUsageRequest`

- POST `/api/v1/admin/dashboard/api-keys-usage`
  - 说明：批量 API Key 用量。
  - 请求：`BatchAPIKeysUsageRequest`

- POST `/api/v1/admin/dashboard/aggregation/backfill`
  - 说明：回填聚合统计。
  - 请求：`DashboardAggregationBackfillRequest`

- GET `/api/v1/admin/users`
  - 说明：用户列表。

- GET `/api/v1/admin/users/:id`
  - 说明：用户详情。

- POST `/api/v1/admin/users`
  - 说明：创建用户。
  - 请求：`CreateUserRequest`

- PUT `/api/v1/admin/users/:id`
  - 说明：更新用户。
  - 请求：`UpdateUserRequest`

- DELETE `/api/v1/admin/users/:id`
  - 说明：删除用户。

- POST `/api/v1/admin/users/:id/balance`
  - 说明：调整余额。
  - 请求：`UpdateBalanceRequest`

- GET `/api/v1/admin/users/:id/api-keys`
  - 说明：获取用户 API Key。

- GET `/api/v1/admin/users/:id/usage`
  - 说明：获取用户用量统计。

- GET `/api/v1/admin/users/:id/attributes`
  - 说明：获取用户属性。

- PUT `/api/v1/admin/users/:id/attributes`
  - 说明：更新用户属性。
  - 请求：`UpdateUserAttributesRequest`

- GET `/api/v1/admin/groups`
  - 说明：分组列表。

- GET `/api/v1/admin/groups/all`
  - 说明：全部分组（无分页）。

- GET `/api/v1/admin/groups/:id`
  - 说明：分组详情。

- POST `/api/v1/admin/groups`
  - 说明：创建分组。
  - 请求：`CreateGroupRequest`

- PUT `/api/v1/admin/groups/:id`
  - 说明：更新分组。
  - 请求：`UpdateGroupRequest`

- DELETE `/api/v1/admin/groups/:id`
  - 说明：删除分组。

- GET `/api/v1/admin/groups/:id/stats`
  - 说明：分组统计。

- GET `/api/v1/admin/groups/:id/api-keys`
  - 说明：分组 API Key。

- GET `/api/v1/admin/accounts`
  - 说明：账号列表。

- GET `/api/v1/admin/accounts/:id`
  - 说明：账号详情。

- POST `/api/v1/admin/accounts`
  - 说明：创建账号。
  - 请求：`CreateAccountRequest`

- POST `/api/v1/admin/accounts/sync/crs`
  - 说明：从 CRS 同步账号。
  - 请求：`SyncFromCRSRequest`

- PUT `/api/v1/admin/accounts/:id`
  - 说明：更新账号。
  - 请求：`UpdateAccountRequest`

- DELETE `/api/v1/admin/accounts/:id`
  - 说明：删除账号。

- POST `/api/v1/admin/accounts/:id/test`
  - 说明：账号连通性测试。
  - 请求：`TestAccountRequest`

- POST `/api/v1/admin/accounts/:id/refresh`
  - 说明：刷新账号凭证。

- POST `/api/v1/admin/accounts/:id/refresh-tier`
  - 说明：刷新 Gemini Google One tier。

- GET `/api/v1/admin/accounts/:id/stats`
  - 说明：账号统计。

- POST `/api/v1/admin/accounts/:id/clear-error`
  - 说明：清理账号错误状态。

- GET `/api/v1/admin/accounts/:id/usage`
  - 说明：账号用量。

- GET `/api/v1/admin/accounts/:id/today-stats`
  - 说明：账号今日统计。

- POST `/api/v1/admin/accounts/:id/clear-rate-limit`
  - 说明：清理账号限流状态。

- GET `/api/v1/admin/accounts/:id/temp-unschedulable`
  - 说明：查询临时不可调度状态。

- DELETE `/api/v1/admin/accounts/:id/temp-unschedulable`
  - 说明：清理临时不可调度状态。

- POST `/api/v1/admin/accounts/:id/schedulable`
  - 说明：设置账号可调度状态。
  - 请求：`SetSchedulableRequest`

- GET `/api/v1/admin/accounts/:id/models`
  - 说明：获取账号可用模型列表。

- POST `/api/v1/admin/accounts/batch`
  - 说明：批量创建账号（占位实现）。

- POST `/api/v1/admin/accounts/batch-update-credentials`
  - 说明：批量更新账号 credentials 字段。
  - 请求：`BatchUpdateCredentialsRequest`

- POST `/api/v1/admin/accounts/batch-refresh-tier`
  - 说明：批量刷新 Gemini Google One tier。
  - 请求：`BatchRefreshTierRequest`

- POST `/api/v1/admin/accounts/bulk-update`
  - 说明：批量更新账号。
  - 请求：`BulkUpdateAccountsRequest`

- POST `/api/v1/admin/accounts/generate-auth-url`
  - 说明：Claude OAuth 授权 URL。
  - 请求：`GenerateAuthURLRequest`

- POST `/api/v1/admin/accounts/generate-setup-token-url`
  - 说明：Claude Setup Token 授权 URL。
  - 请求：`GenerateAuthURLRequest`

- POST `/api/v1/admin/accounts/exchange-code`
  - 说明：Claude OAuth 换取 token。
  - 请求：`ExchangeCodeRequest`

- POST `/api/v1/admin/accounts/exchange-setup-token-code`
  - 说明：Claude Setup Token 换取 token。
  - 请求：`ExchangeCodeRequest`

- POST `/api/v1/admin/accounts/cookie-auth`
  - 说明：Claude Cookie 登录。
  - 请求：`CookieAuthRequest`

- POST `/api/v1/admin/accounts/setup-token-cookie-auth`
  - 说明：Claude Setup Token Cookie 登录。
  - 请求：`CookieAuthRequest`

- POST `/api/v1/admin/openai/generate-auth-url`
  - 说明：OpenAI OAuth 授权 URL。
  - 请求：`OpenAIGenerateAuthURLRequest`

- POST `/api/v1/admin/openai/exchange-code`
  - 说明：OpenAI OAuth 换取 token。
  - 请求：`OpenAIExchangeCodeRequest`

- POST `/api/v1/admin/openai/refresh-token`
  - 说明：OpenAI OAuth 刷新 token。
  - 请求：`OpenAIRefreshTokenRequest`

- POST `/api/v1/admin/openai/accounts/:id/refresh`
  - 说明：刷新 OpenAI OAuth 账号。

- POST `/api/v1/admin/openai/create-from-oauth`
  - 说明：通过 OAuth 创建 OpenAI 账号。

- POST `/api/v1/admin/gemini/oauth/auth-url`
  - 说明：Gemini OAuth 授权 URL。
  - 请求：`GeminiGenerateAuthURLRequest`

- POST `/api/v1/admin/gemini/oauth/exchange-code`
  - 说明：Gemini OAuth 换取 token。
  - 请求：`GeminiExchangeCodeRequest`

- GET `/api/v1/admin/gemini/oauth/capabilities`
  - 说明：Gemini OAuth capabilities。

- POST `/api/v1/admin/antigravity/oauth/auth-url`
  - 说明：Antigravity OAuth 授权 URL。
  - 请求：`AntigravityGenerateAuthURLRequest`

- POST `/api/v1/admin/antigravity/oauth/exchange-code`
  - 说明：Antigravity OAuth 换取 token。
  - 请求：`AntigravityExchangeCodeRequest`

- GET `/api/v1/admin/proxies`
  - 说明：代理列表。

- GET `/api/v1/admin/proxies/all`
  - 说明：全部代理。

- GET `/api/v1/admin/proxies/:id`
  - 说明：代理详情。

- POST `/api/v1/admin/proxies`
  - 说明：创建代理。
  - 请求：`CreateProxyRequest`

- PUT `/api/v1/admin/proxies/:id`
  - 说明：更新代理。
  - 请求：`UpdateProxyRequest`

- DELETE `/api/v1/admin/proxies/:id`
  - 说明：删除代理。

- POST `/api/v1/admin/proxies/:id/test`
  - 说明：测试代理连通性。

- GET `/api/v1/admin/proxies/:id/stats`
  - 说明：代理统计。

- GET `/api/v1/admin/proxies/:id/accounts`
  - 说明：代理账号列表。

- POST `/api/v1/admin/proxies/batch-delete`
  - 说明：批量删除代理。

- POST `/api/v1/admin/proxies/batch`
  - 说明：批量创建代理。

- GET `/api/v1/admin/redeem-codes`
  - 说明：卡密列表。

- GET `/api/v1/admin/redeem-codes/stats`
  - 说明：卡密统计。

- GET `/api/v1/admin/redeem-codes/export`
  - 说明：导出卡密 CSV。

- GET `/api/v1/admin/redeem-codes/:id`
  - 说明：卡密详情。

- POST `/api/v1/admin/redeem-codes/generate`
  - 说明：生成卡密。
  - 请求：`GenerateRedeemCodesRequest`

- DELETE `/api/v1/admin/redeem-codes/:id`
  - 说明：删除卡密。

- POST `/api/v1/admin/redeem-codes/batch-delete`
  - 说明：批量删除卡密。

- POST `/api/v1/admin/redeem-codes/:id/expire`
  - 说明：过期卡密。

- GET `/api/v1/admin/promo-codes`
  - 说明：优惠码列表。

- GET `/api/v1/admin/promo-codes/:id`
  - 说明：优惠码详情。

- POST `/api/v1/admin/promo-codes`
  - 说明：创建优惠码。
  - 请求：`CreatePromoCodeRequest`

- PUT `/api/v1/admin/promo-codes/:id`
  - 说明：更新优惠码。
  - 请求：`UpdatePromoCodeRequest`

- DELETE `/api/v1/admin/promo-codes/:id`
  - 说明：删除优惠码。

- GET `/api/v1/admin/promo-codes/:id/usages`
  - 说明：优惠码使用记录。

- GET `/api/v1/admin/settings`
  - 说明：系统设置。

- PUT `/api/v1/admin/settings`
  - 说明：更新系统设置。
  - 请求：`UpdateSettingsRequest`

- POST `/api/v1/admin/settings/test-smtp`
  - 说明：测试 SMTP 连接。
  - 请求：`TestSMTPRequest`

- POST `/api/v1/admin/settings/send-test-email`
  - 说明：发送测试邮件。
  - 请求：`SendTestEmailRequest`

- GET `/api/v1/admin/settings/admin-api-key`
  - 说明：获取 Admin API Key 状态。

- POST `/api/v1/admin/settings/admin-api-key/regenerate`
  - 说明：重新生成 Admin API Key。

- DELETE `/api/v1/admin/settings/admin-api-key`
  - 说明：删除 Admin API Key。

- GET `/api/v1/admin/settings/stream-timeout`
  - 说明：流超时配置。

- PUT `/api/v1/admin/settings/stream-timeout`
  - 说明：更新流超时配置。
  - 请求：`UpdateStreamTimeoutSettingsRequest`

- GET `/api/v1/admin/ops/concurrency`
  - 说明：并发统计。

- GET `/api/v1/admin/ops/account-availability`
  - 说明：账号可用性。

- GET `/api/v1/admin/ops/realtime-traffic`
  - 说明：实时流量统计。

- GET `/api/v1/admin/ops/alert-rules`
  - 说明：告警规则列表。

- POST `/api/v1/admin/ops/alert-rules`
  - 说明：创建告警规则。

- PUT `/api/v1/admin/ops/alert-rules/:id`
  - 说明：更新告警规则。

- DELETE `/api/v1/admin/ops/alert-rules/:id`
  - 说明：删除告警规则。

- GET `/api/v1/admin/ops/alert-events`
  - 说明：告警事件列表。

- GET `/api/v1/admin/ops/alert-events/:id`
  - 说明：告警事件详情。

- PUT `/api/v1/admin/ops/alert-events/:id/status`
  - 说明：更新告警事件状态。

- POST `/api/v1/admin/ops/alert-silences`
  - 说明：创建告警静默。

- GET `/api/v1/admin/ops/email-notification/config`
  - 说明：告警通知邮件配置。

- PUT `/api/v1/admin/ops/email-notification/config`
  - 说明：更新告警通知邮件配置。

- GET `/api/v1/admin/ops/runtime/alert`
  - 说明：告警运行时配置。

- PUT `/api/v1/admin/ops/runtime/alert`
  - 说明：更新告警运行时配置。

- GET `/api/v1/admin/ops/advanced-settings`
  - 说明：运维高级配置。

- PUT `/api/v1/admin/ops/advanced-settings`
  - 说明：更新运维高级配置。

- GET `/api/v1/admin/ops/settings/metric-thresholds`
  - 说明：指标阈值。

- PUT `/api/v1/admin/ops/settings/metric-thresholds`
  - 说明：更新指标阈值。

- GET `/api/v1/admin/ops/ws/qps`
  - 说明：QPS 实时 WebSocket。

- GET `/api/v1/admin/ops/errors`
  - 说明：错误日志列表。

- GET `/api/v1/admin/ops/errors/:id`
  - 说明：错误详情。

- GET `/api/v1/admin/ops/errors/:id/retries`
  - 说明：错误重试记录。

- POST `/api/v1/admin/ops/errors/:id/retry`
  - 说明：重试错误请求。
  - 请求：`opsRetryRequest`

- PUT `/api/v1/admin/ops/errors/:id/resolve`
  - 说明：更新错误状态。
  - 请求：`opsResolveRequest`

- GET `/api/v1/admin/ops/request-errors`
  - 说明：请求错误列表。

- GET `/api/v1/admin/ops/request-errors/:id`
  - 说明：请求错误详情。

- GET `/api/v1/admin/ops/request-errors/:id/upstream-errors`
  - 说明：请求错误关联的上游错误。

- POST `/api/v1/admin/ops/request-errors/:id/retry-client`
  - 说明：重试客户端请求。

- POST `/api/v1/admin/ops/request-errors/:id/upstream-errors/:idx/retry`
  - 说明：重试某个上游请求。

- PUT `/api/v1/admin/ops/request-errors/:id/resolve`
  - 说明：更新请求错误状态。

- GET `/api/v1/admin/ops/upstream-errors`
  - 说明：上游错误列表。

- GET `/api/v1/admin/ops/upstream-errors/:id`
  - 说明：上游错误详情。

- POST `/api/v1/admin/ops/upstream-errors/:id/retry`
  - 说明：重试上游错误请求。

- PUT `/api/v1/admin/ops/upstream-errors/:id/resolve`
  - 说明：更新上游错误状态。

- GET `/api/v1/admin/ops/requests`
  - 说明：请求明细。

- GET `/api/v1/admin/ops/dashboard/overview`
  - 说明：运维总览。

- GET `/api/v1/admin/ops/dashboard/throughput-trend`
  - 说明：运维吞吐趋势。

- GET `/api/v1/admin/ops/dashboard/latency-histogram`
  - 说明：运维延迟分布。

- GET `/api/v1/admin/ops/dashboard/error-trend`
  - 说明：运维错误趋势。

- GET `/api/v1/admin/ops/dashboard/error-distribution`
  - 说明：运维错误分布。

- GET `/api/v1/admin/system/version`
  - 说明：版本信息。

- GET `/api/v1/admin/system/check-updates`
  - 说明：检查更新。

- POST `/api/v1/admin/system/update`
  - 说明：执行更新。

- POST `/api/v1/admin/system/rollback`
  - 说明：回滚版本。

- POST `/api/v1/admin/system/restart`
  - 说明：重启服务。

- GET `/api/v1/admin/subscriptions`
  - 说明：订阅列表。

- GET `/api/v1/admin/subscriptions/:id`
  - 说明：订阅详情。

- GET `/api/v1/admin/subscriptions/:id/progress`
  - 说明：订阅进度。

- POST `/api/v1/admin/subscriptions/assign`
  - 说明：分配订阅。
  - 请求：`AssignSubscriptionRequest`

- POST `/api/v1/admin/subscriptions/bulk-assign`
  - 说明：批量分配订阅。
  - 请求：`BulkAssignSubscriptionRequest`

- POST `/api/v1/admin/subscriptions/:id/extend`
  - 说明：延长订阅。
  - 请求：`ExtendSubscriptionRequest`

- DELETE `/api/v1/admin/subscriptions/:id`
  - 说明：撤销订阅。

- GET `/api/v1/admin/groups/:id/subscriptions`
  - 说明：分组订阅列表。

- GET `/api/v1/admin/users/:id/subscriptions`
  - 说明：用户订阅列表。

- GET `/api/v1/admin/usage`
  - 说明：用量列表。

- GET `/api/v1/admin/usage/stats`
  - 说明：用量统计。

- GET `/api/v1/admin/usage/search-users`
  - 说明：搜索用户。

- GET `/api/v1/admin/usage/search-api-keys`
  - 说明：搜索 API Key。

- GET `/api/v1/admin/user-attributes`
  - 说明：用户属性定义列表。

- POST `/api/v1/admin/user-attributes`
  - 说明：创建用户属性定义。

- POST `/api/v1/admin/user-attributes/batch`
  - 说明：批量获取用户属性。

- PUT `/api/v1/admin/user-attributes/reorder`
  - 说明：重排用户属性。

- PUT `/api/v1/admin/user-attributes/:id`
  - 说明：更新用户属性定义。

- DELETE `/api/v1/admin/user-attributes/:id`
  - 说明：删除用户属性定义。

## Gateway

- POST `/v1/messages`
  - 说明：Claude 兼容消息接口。
  - 认证：API Key

- POST `/v1/messages/count_tokens`
  - 说明：Token 计数。
  - 认证：API Key

- GET `/v1/models`
  - 说明：模型列表。
  - 认证：API Key

- GET `/v1/usage`
  - 说明：用量统计。
  - 认证：API Key

- POST `/v1/responses`
  - 说明：OpenAI Responses API。
  - 认证：API Key

- GET `/v1beta/models`
  - 说明：Gemini v1beta 模型列表。
  - 认证：API Key

- GET `/v1beta/models/:model`
  - 说明：Gemini v1beta 模型详情。
  - 认证：API Key

- POST `/v1beta/models/*modelAction`
  - 说明：Gemini v1beta 操作接口。
  - 认证：API Key

- POST `/responses`
  - 说明：OpenAI Responses API（无前缀别名）。
  - 认证：API Key

- GET `/antigravity/models`
  - 说明：Antigravity 模型列表。
  - 认证：API Key

- POST `/antigravity/v1/messages`
  - 说明：Antigravity Claude 兼容消息接口。
  - 认证：API Key

- POST `/antigravity/v1/messages/count_tokens`
  - 说明：Antigravity Token 计数。
  - 认证：API Key

- GET `/antigravity/v1/models`
  - 说明：Antigravity 模型列表。
  - 认证：API Key

- GET `/antigravity/v1/usage`
  - 说明：Antigravity 用量统计。
  - 认证：API Key

- GET `/antigravity/v1beta/models`
  - 说明：Antigravity Gemini v1beta 模型列表。
  - 认证：API Key

- GET `/antigravity/v1beta/models/:model`
  - 说明：Antigravity Gemini v1beta 模型详情。
  - 认证：API Key

- POST `/antigravity/v1beta/models/*modelAction`
  - 说明：Antigravity Gemini v1beta 操作接口。
  - 认证：API Key

## Setup

- GET `/setup/status`
  - 说明：获取安装状态。

- POST `/setup/test-db`
  - 说明：测试数据库连接。
  - 请求：`TestDatabaseRequest`

- POST `/setup/test-redis`
  - 说明：测试 Redis 连接。
  - 请求：`TestRedisRequest`

- POST `/setup/install`
  - 说明：执行安装。
  - 请求：`InstallRequest`

## 附录

- 通用响应包裹：`Response`（`backend/internal/pkg/response/response.go`）
- Middleware 错误响应：`ErrorResponse`（`backend/internal/server/middleware/middleware.go`）
