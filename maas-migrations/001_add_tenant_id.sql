-- MaaS 多租户数据隔离迁移脚本 - new-api
-- 为核心业务表添加 tenant_id 字段
-- 注意：new-api 使用 GORM AutoMigrate，添加 struct 字段后 AutoMigrate 会自动创建列
-- 此 SQL 脚本用于手动迁移场景（不通过 GORM AutoMigrate）
-- 幂等性：使用 IF NOT EXISTS / ADD COLUMN IF NOT EXISTS

-- tokens 表：添加 tenant_id
-- MySQL: ALTER TABLE tokens ADD COLUMN IF NOT EXISTS tenant_id BIGINT DEFAULT 0;
-- PostgreSQL:
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'tokens' AND column_name = 'tenant_id') THEN
        ALTER TABLE tokens ADD COLUMN tenant_id BIGINT DEFAULT 0;
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_tokens_tenant_id ON tokens (tenant_id);

-- channels 表：添加 tenant_id
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'channels' AND column_name = 'tenant_id') THEN
        ALTER TABLE channels ADD COLUMN tenant_id BIGINT DEFAULT 0;
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_channels_tenant_id ON channels (tenant_id);

-- logs 表：添加 tenant_id
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'logs' AND column_name = 'tenant_id') THEN
        ALTER TABLE logs ADD COLUMN tenant_id BIGINT DEFAULT 0;
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_logs_tenant_id ON logs (tenant_id);

-- users 表：添加 tenant_id
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'tenant_id') THEN
        ALTER TABLE users ADD COLUMN tenant_id BIGINT DEFAULT 0;
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_users_tenant_id ON users (tenant_id);

-- abilities 表：添加 tenant_id
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'abilities' AND column_name = 'tenant_id') THEN
        ALTER TABLE abilities ADD COLUMN tenant_id BIGINT DEFAULT 0;
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_abilities_tenant_id ON abilities (tenant_id);

-- 验证迁移
SELECT table_name, column_name FROM information_schema.columns
WHERE column_name = 'tenant_id' AND table_schema = 'public'
ORDER BY table_name;