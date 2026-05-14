-- costrict_stat 数据库 - 初始化脚本

-- 切换到 costrict_stat 数据库
\connect costrict_stat

-- 启用 pgcrypto 扩展（gen_random_uuid 函数需要）
CREATE EXTENSION IF NOT EXISTS pgcrypto;

