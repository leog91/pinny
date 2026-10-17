CREATE TABLE device_sessions (
  token_hash TEXT PRIMARY KEY,
  credential_hash TEXT NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX device_sessions_expiry ON device_sessions(expires_at);
