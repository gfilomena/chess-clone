-- ============================================================
-- 003_bot_user.sql
-- Utente speciale che rappresenta il bot Stockfish nelle partite.
-- UUID fisso (nil UUID) per filtrarlo facilmente nelle query stats.
-- ============================================================

INSERT INTO users (id, username, email, elo_rapid, elo_blitz, elo_bullet)
VALUES ('00000000-0000-0000-0000-000000000000', '(bot)', 'bot@chess.internal', 0, 0, 0)
ON CONFLICT (id) DO NOTHING;
