CREATE TABLE IF NOT EXISTS profile_password_reset_tokens (
	token_hash TEXT PRIMARY KEY,
	profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	expires_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_profile_password_reset_tokens_profile_id ON profile_password_reset_tokens(profile_id);
