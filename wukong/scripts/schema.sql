-- ========================================
-- 悟空（Wukong）多智能体系统 - 数据库建表SQL
-- 数据库: PostgreSQL
-- 适用于: v0.1 及以上版本
-- ========================================

-- 数据库名 wukong_agents_db

-- 用户表
DROP TABLE IF EXISTS users CASCADE;
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL UNIQUE,
    username VARCHAR(64) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    email VARCHAR(128) NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_user_id UNIQUE (user_id),
    CONSTRAINT uk_username UNIQUE (username)
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_status ON users(status);
COMMENT ON TABLE users IS '用户表';

-- ========================================
-- 对话链路相关表
-- ========================================

-- 对话会话表 (chat_session)
DROP TABLE IF EXISTS chat_session CASCADE;
CREATE TABLE chat_session (
    id BIGSERIAL PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL UNIQUE,
    user_id VARCHAR(64) NOT NULL,
    title VARCHAR(256) NULL,
    scene VARCHAR(32) NOT NULL DEFAULT 'CHAT',
    status VARCHAR(32) NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expire_at TIMESTAMP WITH TIME ZONE NULL,
    CONSTRAINT uk_session_id UNIQUE (session_id)
);

CREATE INDEX idx_chat_session_user ON chat_session(user_id);
CREATE INDEX idx_chat_session_status ON chat_session(status);
COMMENT ON TABLE chat_session IS '多轮对话会话总表';

-- 对话消息表 (chat_message)
DROP TABLE IF EXISTS chat_message CASCADE;
CREATE TABLE chat_message (
    id BIGSERIAL PRIMARY KEY,
    msg_id VARCHAR(64) NOT NULL UNIQUE,
    session_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    role VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    content_type VARCHAR(32) DEFAULT 'TEXT',
    task_id VARCHAR(64) NULL,
    thought TEXT NULL,
    tool_call JSONB NULL,
    tool_result JSONB NULL,
    seq INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_msg_id UNIQUE (msg_id)
);

CREATE INDEX idx_chat_msg_session ON chat_message(session_id);
CREATE INDEX idx_chat_msg_user ON chat_message(user_id);
CREATE INDEX idx_chat_msg_task ON chat_message(task_id);
CREATE INDEX idx_chat_msg_seq ON chat_message(session_id, seq);
COMMENT ON TABLE chat_message IS '单轮/多轮消息明细表';

-- 对话记忆表 (chat_memory)
DROP TABLE IF EXISTS chat_memory CASCADE;
CREATE TABLE chat_memory (
    id BIGSERIAL PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL UNIQUE,
    user_id VARCHAR(64) NOT NULL,
    recent_messages JSONB NOT NULL DEFAULT '[]'::JSONB,
    summary TEXT NULL,
    user_profile JSONB NULL,
    preference JSONB NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_chat_mem_session UNIQUE (session_id)
);

CREATE INDEX idx_chat_mem_user ON chat_memory(user_id);
COMMENT ON TABLE chat_memory IS '多轮对话全局记忆，跨任务复用';

-- ========================================
-- 任务链路相关表
-- ========================================

-- 主任务表 (task_info)
DROP TABLE IF EXISTS task_info CASCADE;
CREATE TABLE task_info (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL UNIQUE,
    user_id VARCHAR(64) NOT NULL,
    session_id VARCHAR(64) NULL,
    skill_name VARCHAR(64) NOT NULL,
    params JSONB NOT NULL DEFAULT '{}'::JSONB,
    status VARCHAR(32) NOT NULL,
    priority INT NOT NULL DEFAULT 5,
    retry_count INT NOT NULL DEFAULT 0,
    max_retry INT NOT NULL DEFAULT 3,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE NULL,
    result JSONB NULL,
    error TEXT NULL,
    is_deleted BOOLEAN DEFAULT FALSE,
    CONSTRAINT uk_task_id UNIQUE (task_id)
);

CREATE INDEX idx_task_status ON task_info(status);
CREATE INDEX idx_task_user ON task_info(user_id);
CREATE INDEX idx_task_session ON task_info(session_id);
CREATE INDEX idx_task_priority ON task_info(priority);
COMMENT ON TABLE task_info IS '主任务表，核心调度载体';

-- 子任务DAG表 (task_sub)
DROP TABLE IF EXISTS task_sub CASCADE;
CREATE TABLE task_sub (
    id BIGSERIAL PRIMARY KEY,
    sub_task_id VARCHAR(64) NOT NULL UNIQUE,
    task_id VARCHAR(64) NOT NULL,
    depends_on JSONB NOT NULL DEFAULT '[]'::JSONB,
    action VARCHAR(128) NOT NULL,
    params JSONB NOT NULL DEFAULT '{}'::JSONB,
    status VARCHAR(32) NOT NULL,
    worker_id VARCHAR(64) NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    result JSONB NULL,
    error TEXT NULL,
    CONSTRAINT uk_sub_task_id UNIQUE (sub_task_id)
);

CREATE INDEX idx_sub_task_id ON task_sub(task_id);
CREATE INDEX idx_sub_status ON task_sub(status);
COMMENT ON TABLE task_sub IS '子任务表，维护DAG依赖关系';

-- ========================================
-- 记忆体系相关表
-- ========================================

-- 短期工作记忆 (memory_working)
DROP TABLE IF EXISTS memory_working CASCADE;
CREATE TABLE memory_working (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL UNIQUE,
    user_id VARCHAR(64) NOT NULL,
    full_history JSONB NOT NULL DEFAULT '[]'::JSONB,
    summary TEXT NULL,
    window_size INT NOT NULL DEFAULT 5,
    compress_flag BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expire_at TIMESTAMP WITH TIME ZONE NULL,
    CONSTRAINT mem_working_task UNIQUE (task_id)
);

CREATE INDEX idx_mem_work_user ON memory_working(user_id);
COMMENT ON TABLE memory_working IS '任务级短期记忆，任务结束归档';

-- 长期经验记忆 (memory_long_term)
DROP TABLE IF EXISTS memory_long_term CASCADE;
CREATE TABLE memory_long_term (
    id BIGSERIAL PRIMARY KEY,
    memory_id VARCHAR(64) NOT NULL UNIQUE,
    user_id VARCHAR(64) NOT NULL,
    skill_name VARCHAR(64) NOT NULL,
    topic VARCHAR(256) NOT NULL,
    content TEXT NOT NULL,
    embedding  vector(1536)  NULL,
    source_task_id VARCHAR(64) NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_long_term_id UNIQUE (memory_id)
);

CREATE INDEX idx_mem_long_user ON memory_long_term(user_id);
CREATE INDEX idx_mem_long_skill ON memory_long_term(skill_name);
COMMENT ON TABLE memory_long_term IS '行业/经验类长期记忆，支持RAG检索';

-- 共享记忆 (memory_shared)
DROP TABLE IF EXISTS memory_shared CASCADE;
CREATE TABLE memory_shared (
    id BIGSERIAL PRIMARY KEY,
    share_key VARCHAR(128) NOT NULL UNIQUE,
    data JSONB NOT NULL DEFAULT '{}'::JSONB,
    owner_task_id VARCHAR(64) NOT NULL,
    read_only BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_shared_key UNIQUE (share_key)
);

CREATE INDEX idx_mem_shared_owner ON memory_shared(owner_task_id);
COMMENT ON TABLE memory_shared IS '多智能体/子任务共享记忆空间';

-- ========================================
-- 辅助支撑表
-- ========================================

-- 任务执行日志 (task_exec_log)
DROP TABLE IF EXISTS task_exec_log CASCADE;
CREATE TABLE task_exec_log (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL,
    sub_task_id VARCHAR(64) NULL,
    log_type VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_log_task ON task_exec_log(task_id);
CREATE INDEX idx_log_sub_task ON task_exec_log(sub_task_id);
COMMENT ON TABLE task_exec_log IS '任务全链路执行日志，可追溯';

-- 流式消息 (stream_message)
DROP TABLE IF EXISTS stream_message CASCADE;
CREATE TABLE stream_message (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL,
    msg_type VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    seq INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_stream_task ON stream_message(task_id);
CREATE INDEX idx_stream_seq ON stream_message(task_id, seq);
COMMENT ON TABLE stream_message IS '流式实时消息，支持SSE/WebSocket';

-- 技能元信息 (skill_meta)
DROP TABLE IF EXISTS skill_meta CASCADE;
CREATE TABLE skill_meta (
    id BIGSERIAL PRIMARY KEY,
    skill_name VARCHAR(64) NOT NULL UNIQUE,
    description TEXT NULL,
    version VARCHAR(32) NOT NULL DEFAULT '1.0.0',
    enabled BOOLEAN DEFAULT TRUE,
    memory_type VARCHAR(32) DEFAULT 'working',
    memory_window INT DEFAULT 5,
    memory_compress BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_skill_name UNIQUE (skill_name)
);

COMMENT ON TABLE skill_meta IS '技能插件元信息，配置记忆策略';

-- ========================================
-- 初始化默认数据
-- ========================================

-- 插入默认管理员用户 (密码: admin123)
INSERT INTO users (user_id, username, password, email, status) VALUES
('user_admin', 'admin', '$2a$10$N.zmdr9k7uOCQb376NoUnuTJ8iAt6Z5EHsM8lE9lBOsl7iAt6Z5EH', 'admin@wukong.com', 'ACTIVE')
ON CONFLICT (user_id) DO NOTHING;

-- 插入默认技能
INSERT INTO skill_meta (skill_name, description, version, enabled, memory_type, memory_window, memory_compress) VALUES
('chat', '基础对话技能', '1.0.0', true, 'working', 5, true),
('web_search', '联网搜索技能', '1.0.0', true, 'working', 10, true),
('report_gen', '报告生成技能', '1.0.0', true, 'long_term', 20, true)
ON CONFLICT (skill_name) DO NOTHING;
