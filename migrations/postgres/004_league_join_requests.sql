CREATE TABLE IF NOT EXISTS league_join_requests (
  id TEXT PRIMARY KEY,
  league_id TEXT REFERENCES leagues(id),
  user_id TEXT REFERENCES users(id),
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  decided_by TEXT REFERENCES users(id),
  decided_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_join_requests_league_status ON league_join_requests(league_id, status);
CREATE INDEX IF NOT EXISTS idx_join_requests_user_status ON league_join_requests(user_id, status);
