-- costrict_stat 数据库 - 初始化脚本

-- 切换到 costrict_stat 数据库
\connect costrict_stat

CREATE TABLE IF NOT EXISTS tasks (
    task_id VARCHAR(500) PRIMARY KEY,
    user_id VARCHAR(255),
    user_name VARCHAR(255),
    client_id VARCHAR(255),
    client_ide VARCHAR(100),
    client_version VARCHAR(100),
    client_os VARCHAR(100),
    client_os_version VARCHAR(100),
    caller VARCHAR(100),
    repo_addr TEXT,
    repo_branch VARCHAR(500),
    work_dir TEXT,
    work_dir_id VARCHAR(500),
    diff_lines INT,
    start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ,
    upstream_tokens BIGINT,
    downstream_tokens BIGINT,
    cost FLOAT8,
    task_real_minutes FLOAT8,
    task_real_minutes_reason TEXT,
    task_real_minutes_manual FLOAT8,
    task_real_minutes_reason_manual TEXT,
    task_ancient_minutes FLOAT8,
    task_ancient_minutes_reason TEXT,
    task_ancient_minutes_manual FLOAT8,
    task_ancient_minutes_reason_manual TEXT,
    efficiency_ratio FLOAT8,
    title VARCHAR(200),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS task_conversations (
    id SERIAL PRIMARY KEY,
    task_id VARCHAR(500) NOT NULL,
    request_id VARCHAR(500) NOT NULL,
    sender VARCHAR(50),
    prompt_mode VARCHAR(50),
    mode VARCHAR(100),
    model VARCHAR(200),
    start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ,
    process_time BIGINT,
    process_ttft BIGINT,
    upstream_tokens BIGINT,
    downstream_tokens BIGINT,
    cost FLOAT8,
    request_content TEXT,
    response_content TEXT,
    user_input TEXT,
    diff TEXT,
    diff_lines BIGINT,
    error_code VARCHAR(100),
    error_reason TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(task_id, request_id)
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_tasks_user_id ON tasks(user_id);
CREATE INDEX IF NOT EXISTS idx_tasks_work_dir_id ON tasks(work_dir_id);
CREATE INDEX IF NOT EXISTS idx_tasks_start_time ON tasks(start_time);
CREATE INDEX IF NOT EXISTS idx_task_conversations_task_id ON task_conversations(task_id);
CREATE INDEX IF NOT EXISTS idx_task_conversations_start_time ON task_conversations(start_time);

-- commits 表
CREATE TABLE IF NOT EXISTS commits (
    commit_id VARCHAR(500) PRIMARY KEY,
    commit_time TIMESTAMPTZ,
    repo_addr TEXT,
    repo_branch VARCHAR(500),
    git_user_name VARCHAR(255),
    git_user_email VARCHAR(255),
    user_id VARCHAR(255),
    user_name VARCHAR(255),
    client_id VARCHAR(255),
    work_dir TEXT,
    diff_lines INT,
    commit_ancient_minutes FLOAT8,
    commit_ancient_minutes_reason TEXT,
    commit_ancient_minutes_manual FLOAT8,
    commit_ancient_minutes_reason_manual TEXT,
    task_ids JSONB,
    task_ids_silica JSONB,
    upstream_tokens BIGINT,
    downstream_tokens BIGINT,
    cost FLOAT8,
    silica FLOAT8,
    commit_real_ai_minutes FLOAT8,
    commit_real_ancient_minutes FLOAT8,
    commit_real_minutes FLOAT8,
    commit_real_minutes_reason TEXT,
    commit_real_minutes_manual FLOAT8,
    commit_real_minutes_reason_manual TEXT,
    comment VARCHAR(150),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_commits_repo_addr ON commits(repo_addr);
CREATE INDEX IF NOT EXISTS idx_commits_repo_addr_branch ON commits(repo_addr, repo_branch);
CREATE INDEX IF NOT EXISTS idx_commits_user_id ON commits(user_id);
CREATE INDEX IF NOT EXISTS idx_commits_commit_time ON commits(commit_time);

-- repos 表
CREATE TABLE IF NOT EXISTS repos (
    repo_id VARCHAR(500) PRIMARY KEY,
    repo_addr TEXT NOT NULL,
    repo_branch VARCHAR(500) NOT NULL,
    start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ,
    commit_ids JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_repos_repo_addr ON repos(repo_addr);
CREATE INDEX IF NOT EXISTS idx_repos_repo_addr_branch ON repos(repo_addr, repo_branch);

-- projects 表（虚拟项目）
CREATE TABLE IF NOT EXISTS projects (
    project_id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    name VARCHAR(500) NOT NULL,
    description TEXT,
    repos JSONB DEFAULT '[]',
    task_ids JSONB DEFAULT '[]',
    task_ids_silica JSONB DEFAULT '[]',
    start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ,
    start_time_manual TIMESTAMPTZ,
    end_time_manual TIMESTAMPTZ,
    upstream_tokens BIGINT DEFAULT 0,
    downstream_tokens BIGINT DEFAULT 0,
    cost FLOAT8 DEFAULT 0,
    project_ancient_minutes FLOAT8,
    project_ancient_minutes_reason TEXT,
    project_ancient_minutes_manual FLOAT8,
    project_ancient_minutes_reason_manual TEXT,
    project_real_process_minutes FLOAT8,
    project_real_process_minutes_reason TEXT,
    project_real_process_minutes_manual FLOAT8,
    project_real_process_minutes_reason_manual TEXT,
    project_real_lead_minutes FLOAT8,
    project_real_lead_minutes_reason TEXT,
    project_real_lead_minutes_manual FLOAT8,
    project_real_lead_minutes_reason_manual TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name);
CREATE INDEX IF NOT EXISTS idx_projects_updated_at ON projects(updated_at);

-- user_productivity 表（用户生产力预聚合）
CREATE TABLE IF NOT EXISTS user_productivity (
    user_productivity_id VARCHAR(500) PRIMARY KEY,
    create_time TIMESTAMPTZ,
    user_id VARCHAR(255),
    user_name VARCHAR(255),
    task_ids JSONB,
    work_dir_ids JSONB,
    task_diff_lines INT,
    upstream_tokens BIGINT,
    downstream_tokens BIGINT,
    cost FLOAT8,
    task_real_minutes FLOAT8,
    task_ancient_minutes FLOAT8,
    task_efficiency_ratio FLOAT8,
    commit_ids JSONB,
    commit_diff_lines INT,
    commit_ancient_minutes FLOAT8,
    commit_real_ai_minutes FLOAT8,
    commit_real_ancient_minutes FLOAT8,
    commit_real_minutes FLOAT8,
    commit_efficiency_ratio FLOAT8,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_productivity_user_id ON user_productivity(user_id);
CREATE INDEX IF NOT EXISTS idx_user_productivity_create_time ON user_productivity(create_time);

-- user_groups 表（虚拟用户组）
CREATE TABLE IF NOT EXISTS user_groups (
    group_id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    name VARCHAR(500) NOT NULL,
    org_name VARCHAR(200) DEFAULT '',
    user_ids JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_groups_name ON user_groups(name);
