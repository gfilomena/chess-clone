-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username      VARCHAR(30) UNIQUE NOT NULL,
    email         VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255),
    google_id     VARCHAR(255) UNIQUE,
    avatar_url    VARCHAR(500),
    elo_rapid     INTEGER NOT NULL DEFAULT 1200,
    elo_blitz     INTEGER NOT NULL DEFAULT 1200,
    elo_bullet    INTEGER NOT NULL DEFAULT 1200,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    last_seen     TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Games
CREATE TYPE game_status AS ENUM ('waiting', 'active', 'paused', 'finished');
CREATE TYPE game_result AS ENUM ('white', 'black', 'draw', 'abandoned');
CREATE TYPE finish_reason AS ENUM ('checkmate', 'timeout', 'resigned', 'stalemate', 'fifty_moves', 'threefold', 'abandoned', 'draw_agreed');

CREATE TABLE games (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    white_id      UUID NOT NULL REFERENCES users(id),
    black_id      UUID NOT NULL REFERENCES users(id),
    status        game_status NOT NULL DEFAULT 'waiting',
    result        game_result,
    finish_reason finish_reason,
    time_control  INTEGER NOT NULL DEFAULT 600,
    increment     INTEGER NOT NULL DEFAULT 0,
    pgn           TEXT NOT NULL DEFAULT '',
    started_at    TIMESTAMP,
    finished_at   TIMESTAMP,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_games_white_id ON games(white_id);
CREATE INDEX idx_games_black_id ON games(black_id);
CREATE INDEX idx_games_status ON games(status);

-- ELO history
CREATE TABLE elo_history (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES users(id),
    game_id    UUID NOT NULL REFERENCES games(id),
    game_type  VARCHAR(10) NOT NULL DEFAULT 'rapid',
    elo_before INTEGER NOT NULL,
    elo_after  INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_elo_history_user_id ON elo_history(user_id);
