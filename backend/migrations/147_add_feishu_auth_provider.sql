ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_signup_source_check;

ALTER TABLE users
    ADD CONSTRAINT users_signup_source_check
    CHECK (signup_source IN ('email', 'linuxdo', 'wechat', 'oidc', 'github', 'google', 'dingtalk', 'feishu'));

ALTER TABLE auth_identities
    DROP CONSTRAINT IF EXISTS auth_identities_provider_type_check;

ALTER TABLE auth_identities
    ADD CONSTRAINT auth_identities_provider_type_check
    CHECK (provider_type IN ('email', 'linuxdo', 'wechat', 'oidc', 'github', 'google', 'dingtalk', 'feishu'));

ALTER TABLE auth_identity_channels
    DROP CONSTRAINT IF EXISTS auth_identity_channels_provider_type_check;

ALTER TABLE auth_identity_channels
    ADD CONSTRAINT auth_identity_channels_provider_type_check
    CHECK (provider_type IN ('email', 'linuxdo', 'wechat', 'oidc', 'github', 'google', 'dingtalk', 'feishu'));

ALTER TABLE pending_auth_sessions
    DROP CONSTRAINT IF EXISTS pending_auth_sessions_provider_type_check;

ALTER TABLE pending_auth_sessions
    ADD CONSTRAINT pending_auth_sessions_provider_type_check
    CHECK (provider_type IN ('email', 'linuxdo', 'wechat', 'oidc', 'github', 'google', 'dingtalk', 'feishu'));

ALTER TABLE user_provider_default_grants
    DROP CONSTRAINT IF EXISTS user_provider_default_grants_provider_type_check;

ALTER TABLE user_provider_default_grants
    ADD CONSTRAINT user_provider_default_grants_provider_type_check
    CHECK (provider_type IN ('email', 'linuxdo', 'wechat', 'oidc', 'github', 'google', 'dingtalk', 'feishu'));

INSERT INTO settings (key, value)
VALUES
    ('feishu_connect_enabled', 'false'),
    ('feishu_connect_app_id', ''),
    ('feishu_connect_app_secret', ''),
    ('feishu_connect_authorize_url', 'https://accounts.feishu.cn/open-apis/authen/v1/authorize'),
    ('feishu_connect_token_url', 'https://open.feishu.cn/open-apis/authen/v2/oauth/token'),
    ('feishu_connect_userinfo_url', 'https://open.feishu.cn/open-apis/authen/v1/user_info'),
    ('feishu_connect_scopes', ''),
    ('feishu_connect_redirect_url', ''),
    ('feishu_connect_frontend_redirect_url', '/auth/feishu/callback'),
    ('auth_source_default_feishu_balance', '0'),
    ('auth_source_default_feishu_concurrency', '5'),
    ('auth_source_default_feishu_subscriptions', '[]'),
    ('auth_source_default_feishu_grant_on_signup', 'false'),
    ('auth_source_default_feishu_grant_on_first_bind', 'false'),
    ('auth_source_default_feishu_platform_quotas', '{}')
ON CONFLICT (key) DO NOTHING;
