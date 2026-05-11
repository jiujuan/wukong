-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL UNIQUE,
    username VARCHAR(64) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    email VARCHAR(128),
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_user_id UNIQUE (user_id),
    CONSTRAINT uk_username UNIQUE (username)
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
COMMENT ON TABLE users IS 'users';

CREATE TABLE IF NOT EXISTS chat_session (
    id BIGSERIAL PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL UNIQUE,
    user_id VARCHAR(64) NOT NULL,
    title VARCHAR(256),
    scene VARCHAR(32) NOT NULL DEFAULT 'CHAT',
    status VARCHAR(32) NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expire_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT uk_session_id UNIQUE (session_id)
);

CREATE INDEX IF NOT EXISTS idx_chat_session_user ON chat_session(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_session_status ON chat_session(status);
COMMENT ON TABLE chat_session IS 'chat sessions';

CREATE TABLE IF NOT EXISTS chat_message (
    id BIGSERIAL PRIMARY KEY,
    msg_id VARCHAR(64) NOT NULL UNIQUE,
    session_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    role VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    content_type VARCHAR(32) DEFAULT 'TEXT',
    task_id VARCHAR(64),
    thought TEXT,
    tool_call JSONB,
    tool_result JSONB,
    seq INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_msg_id UNIQUE (msg_id)
);

CREATE INDEX IF NOT EXISTS idx_chat_msg_session ON chat_message(session_id);
CREATE INDEX IF NOT EXISTS idx_chat_msg_user ON chat_message(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_msg_task ON chat_message(task_id);
CREATE INDEX IF NOT EXISTS idx_chat_msg_seq ON chat_message(session_id, seq);
COMMENT ON TABLE chat_message IS 'chat messages';

CREATE TABLE IF NOT EXISTS chat_memory (
    id BIGSERIAL PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL UNIQUE,
    user_id VARCHAR(64) NOT NULL,
    recent_messages JSONB NOT NULL DEFAULT '[]'::JSONB,
    summary TEXT,
    user_profile JSONB,
    preference JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_chat_mem_session UNIQUE (session_id)
);

CREATE INDEX IF NOT EXISTS idx_chat_mem_user ON chat_memory(user_id);
COMMENT ON TABLE chat_memory IS 'chat memory';

CREATE TABLE IF NOT EXISTS task_info (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL UNIQUE,
    user_id VARCHAR(64) NOT NULL,
    session_id VARCHAR(64),
    skill_name VARCHAR(64) NOT NULL,
    params JSONB NOT NULL DEFAULT '{}'::JSONB,
    status VARCHAR(32) NOT NULL,
    priority INT NOT NULL DEFAULT 5,
    retry_count INT NOT NULL DEFAULT 0,
    max_retry INT NOT NULL DEFAULT 3,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    result JSONB,
    error TEXT,
    is_deleted BOOLEAN DEFAULT FALSE,
    CONSTRAINT uk_task_id UNIQUE (task_id)
);

CREATE INDEX IF NOT EXISTS idx_task_status ON task_info(status);
CREATE INDEX IF NOT EXISTS idx_task_user ON task_info(user_id);
CREATE INDEX IF NOT EXISTS idx_task_session ON task_info(session_id);
CREATE INDEX IF NOT EXISTS idx_task_priority ON task_info(priority);
COMMENT ON TABLE task_info IS 'main tasks';

CREATE TABLE IF NOT EXISTS task_sub (
    id BIGSERIAL PRIMARY KEY,
    sub_task_id VARCHAR(64) NOT NULL UNIQUE,
    task_id VARCHAR(64) NOT NULL,
    depends_on JSONB NOT NULL DEFAULT '[]'::JSONB,
    action VARCHAR(128) NOT NULL,
    params JSONB NOT NULL DEFAULT '{}'::JSONB,
    status VARCHAR(32) NOT NULL,
    worker_id VARCHAR(64),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    result JSONB,
    error TEXT,
    CONSTRAINT uk_sub_task_id UNIQUE (sub_task_id)
);

CREATE INDEX IF NOT EXISTS idx_sub_task_id ON task_sub(task_id);
CREATE INDEX IF NOT EXISTS idx_sub_status ON task_sub(status);
COMMENT ON TABLE task_sub IS 'subtask DAG nodes';

CREATE TABLE IF NOT EXISTS memory_working (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL UNIQUE,
    user_id VARCHAR(64) NOT NULL,
    full_history JSONB NOT NULL DEFAULT '[]'::JSONB,
    summary TEXT,
    window_size INT NOT NULL DEFAULT 5,
    compress_flag BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expire_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT mem_working_task UNIQUE (task_id)
);

CREATE INDEX IF NOT EXISTS idx_mem_work_user ON memory_working(user_id);
COMMENT ON TABLE memory_working IS 'working memory';

CREATE TABLE IF NOT EXISTS memory_long_term (
    id BIGSERIAL PRIMARY KEY,
    memory_id VARCHAR(64) NOT NULL UNIQUE,
    user_id VARCHAR(64) NOT NULL,
    skill_name VARCHAR(64) NOT NULL,
    topic VARCHAR(256) NOT NULL,
    content TEXT NOT NULL,
    embedding vector(1536),
    source_task_id VARCHAR(64),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_long_term_id UNIQUE (memory_id)
);

CREATE INDEX IF NOT EXISTS idx_mem_long_user ON memory_long_term(user_id);
CREATE INDEX IF NOT EXISTS idx_mem_long_skill ON memory_long_term(skill_name);
COMMENT ON TABLE memory_long_term IS 'long term memory';

CREATE TABLE IF NOT EXISTS memory_shared (
    id BIGSERIAL PRIMARY KEY,
    share_key VARCHAR(128) NOT NULL UNIQUE,
    data JSONB NOT NULL DEFAULT '{}'::JSONB,
    owner_task_id VARCHAR(64) NOT NULL,
    read_only BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_shared_key UNIQUE (share_key)
);

CREATE INDEX IF NOT EXISTS idx_mem_shared_owner ON memory_shared(owner_task_id);
COMMENT ON TABLE memory_shared IS 'shared memory';

CREATE TABLE IF NOT EXISTS task_exec_log (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL,
    sub_task_id VARCHAR(64),
    log_type VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_log_task ON task_exec_log(task_id);
CREATE INDEX IF NOT EXISTS idx_log_sub_task ON task_exec_log(sub_task_id);
COMMENT ON TABLE task_exec_log IS 'task execution logs';

CREATE TABLE IF NOT EXISTS stream_message (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL,
    msg_type VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    seq INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_stream_task ON stream_message(task_id);
CREATE INDEX IF NOT EXISTS idx_stream_seq ON stream_message(task_id, seq);
COMMENT ON TABLE stream_message IS 'stream messages';

CREATE TABLE IF NOT EXISTS skill_meta (
    id BIGSERIAL PRIMARY KEY,
    skill_name VARCHAR(64) NOT NULL UNIQUE,
    description TEXT,
    version VARCHAR(32) NOT NULL DEFAULT '1.0.0',
    enabled BOOLEAN DEFAULT TRUE,
    memory_type VARCHAR(32) DEFAULT 'working',
    memory_window INT DEFAULT 5,
    memory_compress BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_skill_name UNIQUE (skill_name)
);

COMMENT ON TABLE skill_meta IS 'skill metadata';

INSERT INTO users (user_id, username, password, email, status) VALUES
('user_admin', 'admin', '$2a$10$N.zmdr9k7uOCQb376NoUnuTJ8iAt6Z5EHsM8lE9lBOsl7iAt6Z5EH', 'admin@wukong.com', 'ACTIVE')
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO skill_meta (skill_name, description, version, enabled, memory_type, memory_window, memory_compress) VALUES
('chat', 'basic chat skill', '1.0.0', true, 'working', 5, true),
('web_search', 'web search skill', '1.0.0', true, 'working', 10, true),
('report_gen', 'report generation skill', '1.0.0', true, 'long_term', 20, true)
ON CONFLICT (skill_name) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS skill_meta CASCADE;
DROP TABLE IF EXISTS stream_message CASCADE;
DROP TABLE IF EXISTS task_exec_log CASCADE;
DROP TABLE IF EXISTS memory_shared CASCADE;
DROP TABLE IF EXISTS memory_long_term CASCADE;
DROP TABLE IF EXISTS memory_working CASCADE;
DROP TABLE IF EXISTS task_sub CASCADE;
DROP TABLE IF EXISTS task_info CASCADE;
DROP TABLE IF EXISTS chat_memory CASCADE;
DROP TABLE IF EXISTS chat_message CASCADE;
DROP TABLE IF EXISTS chat_session CASCADE;
DROP TABLE IF EXISTS users CASCADE;
-- +goose StatementEnd
