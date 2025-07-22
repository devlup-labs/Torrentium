
DROP TABLE IF EXISTS active_connections;
DROP TABLE IF EXISTS trust_scores;
DROP TABLE IF EXISTS peer_files;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS peers;


CREATE EXTENSION IF NOT EXISTS "uuid-ossp";


CREATE TABLE peers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    peer_id TEXT UNIQUE NOT NULL,
    name TEXT,
    multiaddrs TEXT[], 
    is_online BOOLEAN DEFAULT false,
    last_seen TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    file_hash TEXT UNIQUE NOT NULL,
    filename TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    content_type TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE peer_files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    peer_id UUID NOT NULL REFERENCES peers(id) ON DELETE CASCADE,
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    announced_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (peer_id, file_id)
);

CREATE TABLE trust_scores (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    peer_id UUID UNIQUE NOT NULL REFERENCES peers(id) ON DELETE CASCADE,
    score DECIMAL(3, 2) DEFAULT 0.50,
    successful_transfers INT DEFAULT 0,
    failed_transfers INT DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE active_connections (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    requester_id UUID NOT NULL REFERENCES peers(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES peers(id) ON DELETE CASCADE,
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    status TEXT,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE
);


CREATE INDEX idx_peers_peer_id ON peers(peer_id);
CREATE INDEX idx_files_file_hash ON files(file_hash);


