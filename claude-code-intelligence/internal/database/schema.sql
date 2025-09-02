-- Claude Code Intelligence Database Schema
-- Version: 1.0.0
-- Purpose: Store compressed sessions, embeddings, and metadata for intelligent session management

-- Enable foreign key constraints
PRAGMA foreign_keys = ON;

-- Sessions table: Core session storage
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    project_id TEXT,
    name TEXT NOT NULL,
    original_path TEXT NOT NULL,
    compressed_path TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    original_size INTEGER DEFAULT 0,
    compressed_size INTEGER DEFAULT 0,
    compression_ratio REAL DEFAULT 0,
    compression_model TEXT,
    status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'processing', 'compressed', 'failed', 'archived')),
    error_message TEXT,
    metadata TEXT DEFAULT '{}',
    summary TEXT,
    processing_time_ms INTEGER
);

-- Embeddings table: Vector embeddings for semantic search
CREATE TABLE IF NOT EXISTS embeddings (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    embedding BLOB NOT NULL,
    content_preview TEXT,
    chunk_size INTEGER DEFAULT 0,
    model_used TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    UNIQUE(session_id, chunk_index)
);

-- Topics table: Extracted topics from sessions
CREATE TABLE IF NOT EXISTS topics (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    topic TEXT NOT NULL,
    relevance_score REAL DEFAULT 0.5 CHECK(relevance_score >= 0 AND relevance_score <= 1),
    frequency INTEGER DEFAULT 1,
    first_mentioned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    context TEXT,
    extracted_by_model TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

-- Decisions table: Important decisions tracked across sessions
CREATE TABLE IF NOT EXISTS decisions (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    decision_text TEXT NOT NULL,
    reasoning TEXT,
    outcome TEXT,
    importance_score REAL DEFAULT 0.5 CHECK(importance_score >= 0 AND importance_score <= 1),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    tags TEXT DEFAULT '[]',
    extracted_by_model TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

-- Projects table: Group sessions by project
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    path TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_active TIMESTAMP,
    session_count INTEGER DEFAULT 0,
    total_size INTEGER DEFAULT 0,
    metadata TEXT DEFAULT '{}'
);

-- Search queries table: Track search history for analytics
CREATE TABLE IF NOT EXISTS search_queries (
    id TEXT PRIMARY KEY,
    query TEXT NOT NULL,
    query_embedding BLOB,
    results_count INTEGER DEFAULT 0,
    execution_time_ms INTEGER DEFAULT 0,
    clicked_results TEXT DEFAULT '[]',
    model_used TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Model performance table: Track model performance for optimization
CREATE TABLE IF NOT EXISTS model_performance (
    id TEXT PRIMARY KEY,
    model_name TEXT NOT NULL,
    operation_type TEXT NOT NULL,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    avg_processing_time_ms REAL DEFAULT 0,
    avg_quality_score REAL DEFAULT 0,
    total_tokens_used INTEGER DEFAULT 0,
    last_used TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(model_name, operation_type)
);

-- Session relationships table: Cross-session relationships
CREATE TABLE IF NOT EXISTS session_relationships (
    id TEXT PRIMARY KEY,
    source_session_id TEXT NOT NULL,
    target_session_id TEXT NOT NULL,
    relationship_type TEXT NOT NULL CHECK(relationship_type IN ('continuation', 'reference', 'similar')),
    similarity_score REAL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (source_session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (target_session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    UNIQUE(source_session_id, target_session_id, relationship_type)
);

-- Migrations table: Track schema migrations
CREATE TABLE IF NOT EXISTS migrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project_id);
CREATE INDEX IF NOT EXISTS idx_sessions_created ON sessions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_compression_model ON sessions(compression_model);

CREATE INDEX IF NOT EXISTS idx_embeddings_session ON embeddings(session_id);
CREATE INDEX IF NOT EXISTS idx_embeddings_model ON embeddings(model_used);
CREATE INDEX IF NOT EXISTS idx_embeddings_hash ON embeddings(content_hash);

CREATE INDEX IF NOT EXISTS idx_topics_session ON topics(session_id);
CREATE INDEX IF NOT EXISTS idx_topics_relevance ON topics(relevance_score DESC);
CREATE INDEX IF NOT EXISTS idx_topics_topic ON topics(topic);

CREATE INDEX IF NOT EXISTS idx_decisions_session ON decisions(session_id);
CREATE INDEX IF NOT EXISTS idx_decisions_importance ON decisions(importance_score DESC);

CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name);
CREATE INDEX IF NOT EXISTS idx_projects_last_active ON projects(last_active DESC);

CREATE INDEX IF NOT EXISTS idx_search_queries_created ON search_queries(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_model_performance_model_op ON model_performance(model_name, operation_type);

-- Triggers for automatic timestamp updates
CREATE TRIGGER IF NOT EXISTS update_sessions_timestamp 
AFTER UPDATE ON sessions
BEGIN
    UPDATE sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_projects_last_active
AFTER INSERT ON sessions
BEGIN
    UPDATE projects SET last_active = CURRENT_TIMESTAMP WHERE id = NEW.project_id;
END;

CREATE TRIGGER IF NOT EXISTS update_project_session_count
AFTER INSERT ON sessions
BEGIN
    UPDATE projects 
    SET session_count = session_count + 1 
    WHERE id = NEW.project_id;
END;

-- Views for common queries
CREATE VIEW IF NOT EXISTS v_session_stats AS
SELECT 
    s.id,
    s.name,
    s.created_at,
    s.compression_ratio,
    s.compression_model,
    s.status,
    COUNT(DISTINCT t.id) as topic_count,
    COUNT(DISTINCT d.id) as decision_count,
    COUNT(DISTINCT e.id) as embedding_count
FROM sessions s
LEFT JOIN topics t ON s.id = t.session_id
LEFT JOIN decisions d ON s.id = d.session_id
LEFT JOIN embeddings e ON s.id = e.session_id
GROUP BY s.id;

CREATE VIEW IF NOT EXISTS v_model_usage AS
SELECT 
    model_name,
    operation_type,
    success_count + failure_count as total_operations,
    CASE 
        WHEN success_count + failure_count > 0 
        THEN CAST(success_count AS REAL) / (success_count + failure_count) * 100
        ELSE 0 
    END as success_rate,
    avg_processing_time_ms,
    avg_quality_score,
    last_used
FROM model_performance
ORDER BY last_used DESC;

-- Insert initial migration record
INSERT OR IGNORE INTO migrations (name) VALUES ('initial_schema_v1.0.0');