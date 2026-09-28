-- False for "remember me" unchecked: browser-session cookie with a short server-side expiry.
ALTER TABLE profile_refresh_tokens ADD COLUMN IF NOT EXISTS persistent BOOLEAN NOT NULL DEFAULT TRUE;
