-- AI Coding 指标看板 - 数据库初始化脚本
-- 包含：project_metrics, repo_metrics, correction_history

CREATE TABLE IF NOT EXISTS project_metrics (
    id SERIAL PRIMARY KEY,
    project_id VARCHAR(500) NOT NULL,
    analysis_date DATE NOT NULL,
    query_start_date DATE NOT NULL,
    query_end_date DATE NOT NULL,

    -- AI 预估原始值（不可修改，用于审计）
    raw_ai_estimated_days DECIMAL(10,2),
    raw_total_cost DECIMAL(10,2),
    raw_total_code_lines BIGINT,
    raw_task_count INTEGER,

    -- 用户纠正值（可修改）
    corrected_ai_estimated_days DECIMAL(10,2),
    correction_reason TEXT,
    corrected_by VARCHAR(100),
    corrected_at TIMESTAMP,

    -- 实际耗时（系统自动计算）
    actual_start_time TIMESTAMP,
    actual_end_time TIMESTAMP,
    total_lead_time_ms BIGINT,
    total_process_time_ms BIGINT,
    user_count INTEGER,

    -- 提效比例（自动计算）
    efficiency_ratio_lead DECIMAL(10,2),
    efficiency_ratio_process DECIMAL(10,2),

    -- 成本相关
    api_cost DECIMAL(10,2),
    daily_rate DECIMAL(10,2) DEFAULT 400.00,
    cost_saving DECIMAL(10,2),
    roi DECIMAL(10,2),

    -- 溯源文件
    analysis_file_path VARCHAR(500),

    -- 元信息
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(project_id, analysis_date, query_start_date, query_end_date)
);

CREATE TABLE IF NOT EXISTS repo_metrics (
    id SERIAL PRIMARY KEY,
    repo_id VARCHAR(500) NOT NULL,
    analysis_date DATE NOT NULL,
    query_start_date DATE NOT NULL,
    query_end_date DATE NOT NULL,

    -- Git 分析数据
    git_commit_count INTEGER,
    git_contributor_count INTEGER,
    git_lines_added BIGINT,
    git_lines_deleted BIGINT,
    git_files_changed INTEGER,

    -- AI 预估（Task + Git 双重分析）
    raw_ai_estimated_days_from_task DECIMAL(10,2),
    raw_ai_estimated_days_from_git DECIMAL(10,2),
    raw_ai_estimated_days_final DECIMAL(10,2),

    -- 用户纠正值
    corrected_ai_estimated_days DECIMAL(10,2),
    correction_reason TEXT,
    corrected_by VARCHAR(100),
    corrected_at TIMESTAMP,

    -- 实际耗时
    actual_start_time TIMESTAMP,
    actual_end_time TIMESTAMP,
    total_lead_time_ms BIGINT,
    total_process_time_ms BIGINT,

    -- 提效比例
    efficiency_ratio_lead DECIMAL(10,2),
    efficiency_ratio_process DECIMAL(10,2),

    -- 成本
    api_cost DECIMAL(10,2),
    daily_rate DECIMAL(10,2) DEFAULT 400.00,
    cost_saving DECIMAL(10,2),
    roi DECIMAL(10,2),

    -- 溯源
    analysis_file_path VARCHAR(500),
    git_analysis_file_path VARCHAR(500),

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(repo_id, analysis_date, query_start_date, query_end_date)
);

CREATE TABLE IF NOT EXISTS correction_history (
    id SERIAL PRIMARY KEY,
    dimension VARCHAR(20) NOT NULL,       -- 'project' 或 'repo'
    dimension_id VARCHAR(500) NOT NULL,
    analysis_date DATE NOT NULL,
    field_name VARCHAR(100) NOT NULL,     -- 修改的字段
    old_value TEXT,
    new_value TEXT,
    reason TEXT,
    corrected_by VARCHAR(100),
    corrected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS task_commit_mapping (
    id SERIAL PRIMARY KEY,
    repo_id VARCHAR(500) NOT NULL,
    task_id VARCHAR(100) NOT NULL,
    commit_hash VARCHAR(40) NOT NULL,
    user_id VARCHAR(100),
    match_score DECIMAL(5,2),
    match_reason TEXT,
    code_source VARCHAR(20) NOT NULL,
    analysis_date DATE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(repo_id, task_id, commit_hash)
);

CREATE TABLE IF NOT EXISTS code_attribution (
    id SERIAL PRIMARY KEY,
    repo_id VARCHAR(500) NOT NULL,
    commit_hash VARCHAR(40) NOT NULL,
    task_id VARCHAR(100),
    our_ai_code_lines BIGINT DEFAULT 0,
    human_code_lines BIGINT DEFAULT 0,
    total_added_lines BIGINT DEFAULT 0,
    analysis_date DATE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(repo_id, commit_hash, task_id)
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'project_metrics' AND column_name = 'our_ai_code_lines') THEN
        ALTER TABLE project_metrics ADD COLUMN our_ai_code_lines BIGINT DEFAULT 0;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'project_metrics' AND column_name = 'human_code_lines') THEN
        ALTER TABLE project_metrics ADD COLUMN human_code_lines BIGINT DEFAULT 0;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'project_metrics' AND column_name = 'user_manual_days') THEN
        ALTER TABLE project_metrics ADD COLUMN user_manual_days DECIMAL(10,2);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'project_metrics' AND column_name = 'user_manual_days_reason') THEN
        ALTER TABLE project_metrics ADD COLUMN user_manual_days_reason TEXT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'project_metrics' AND column_name = 'user_manual_days_by') THEN
        ALTER TABLE project_metrics ADD COLUMN user_manual_days_by VARCHAR(100);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'project_metrics' AND column_name = 'user_manual_days_at') THEN
        ALTER TABLE project_metrics ADD COLUMN user_manual_days_at TIMESTAMP;
    END IF;
END $$;

-- 虚拟组
CREATE TABLE IF NOT EXISTS virtual_groups (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    dimension VARCHAR(20) NOT NULL,  -- project/repo/user/org1/org2/org3/org4
    member_keys TEXT[] NOT NULL,     -- PostgreSQL 数组
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 收藏
CREATE TABLE IF NOT EXISTS favorites (
    id SERIAL PRIMARY KEY,
    dimension VARCHAR(20) NOT NULL,
    item_key VARCHAR(500) NOT NULL,     -- 真实key 或 "vg_{id}" 表示虚拟组
    display_name VARCHAR(200),
    virtual_group_id INTEGER REFERENCES virtual_groups(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(dimension, item_key)
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'repo_metrics' AND column_name = 'our_ai_code_lines') THEN
        ALTER TABLE repo_metrics ADD COLUMN our_ai_code_lines BIGINT DEFAULT 0;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'repo_metrics' AND column_name = 'human_code_lines') THEN
        ALTER TABLE repo_metrics ADD COLUMN human_code_lines BIGINT DEFAULT 0;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'repo_metrics' AND column_name = 'ai_other_code_lines') THEN
        ALTER TABLE repo_metrics ADD COLUMN ai_other_code_lines BIGINT DEFAULT 0;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'repo_metrics' AND column_name = 'unknown_code_lines') THEN
        ALTER TABLE repo_metrics ADD COLUMN unknown_code_lines BIGINT DEFAULT 0;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'repo_metrics' AND column_name = 'mapped_task_count') THEN
        ALTER TABLE repo_metrics ADD COLUMN mapped_task_count INTEGER DEFAULT 0;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'repo_metrics' AND column_name = 'user_manual_days') THEN
        ALTER TABLE repo_metrics ADD COLUMN user_manual_days DECIMAL(10,2);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'repo_metrics' AND column_name = 'user_manual_days_reason') THEN
        ALTER TABLE repo_metrics ADD COLUMN user_manual_days_reason TEXT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'repo_metrics' AND column_name = 'user_manual_days_by') THEN
        ALTER TABLE repo_metrics ADD COLUMN user_manual_days_by VARCHAR(100);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'repo_metrics' AND column_name = 'user_manual_days_at') THEN
        ALTER TABLE repo_metrics ADD COLUMN user_manual_days_at TIMESTAMP;
    END IF;
END $$;

-- ============================================================
-- 迁移：清理已废弃的 costrict_* 表
-- ============================================================
DROP TABLE IF EXISTS costrict_projects CASCADE;
DROP TABLE IF EXISTS costrict_commits CASCADE;
DROP TABLE IF EXISTS costrict_tasks CASCADE;
DROP TABLE IF EXISTS costrict_task_conversations CASCADE;
