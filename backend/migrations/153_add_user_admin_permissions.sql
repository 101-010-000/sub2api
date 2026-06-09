-- 用户可委派后台权限。
-- 空数组表示没有后台权限；role=admin 仍然自动拥有全部后台权限。

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS admin_permissions JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE users
SET admin_permissions = '[]'::jsonb
WHERE admin_permissions IS NULL;
