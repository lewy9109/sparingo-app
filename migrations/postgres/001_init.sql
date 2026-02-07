CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  first_name TEXT,
  last_name TEXT,
  phone TEXT,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT,
  role TEXT,
  skill TEXT,
  avatar_url TEXT
);

CREATE TABLE IF NOT EXISTS leagues (
  id TEXT PRIMARY KEY,
  name TEXT,
  description TEXT,
  location TEXT,
  owner_id TEXT REFERENCES users(id),
  admin_roles JSONB,
  player_ids JSONB,
  sets_per_match INTEGER,
  start_date TIMESTAMPTZ,
  end_date TIMESTAMPTZ,
  status TEXT,
  created_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS matches (
  id TEXT PRIMARY KEY,
  league_id TEXT REFERENCES leagues(id),
  player_a_id TEXT REFERENCES users(id),
  player_b_id TEXT REFERENCES users(id),
  sets_json JSONB,
  status TEXT,
  reported_by TEXT,
  confirmed_by TEXT,
  created_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS friendly_matches (
  id TEXT PRIMARY KEY,
  player_a_id TEXT REFERENCES users(id),
  player_b_id TEXT REFERENCES users(id),
  sets_json JSONB,
  status TEXT,
  reported_by TEXT,
  confirmed_by TEXT,
  played_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_leagues_created_at ON leagues(created_at);
CREATE INDEX IF NOT EXISTS idx_matches_league_id ON matches(league_id);
CREATE INDEX IF NOT EXISTS idx_matches_created_at ON matches(created_at);
CREATE INDEX IF NOT EXISTS idx_friendly_matches_played_at ON friendly_matches(played_at);
