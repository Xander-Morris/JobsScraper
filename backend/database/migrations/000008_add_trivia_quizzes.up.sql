CREATE TABLE IF NOT EXISTS trivia_quizzes (
	id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	source TEXT NOT NULL CHECK (source IN ('resume', 'prompt')),
	-- The user's prompt, or a fixed label for resume quizzes
	topic TEXT NOT NULL,
	difficulty TEXT NOT NULL CHECK (difficulty IN ('easy', 'medium', 'hard')),
	questions JSONB NOT NULL,
	answers JSONB NOT NULL DEFAULT '[]',
	score INTEGER NOT NULL DEFAULT 0,
	completed_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_trivia_quizzes_profile ON trivia_quizzes(profile_id, created_at DESC);
