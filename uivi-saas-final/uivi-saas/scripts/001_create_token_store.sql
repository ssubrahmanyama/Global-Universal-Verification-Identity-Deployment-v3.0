-- SECURITY FIX: Create token_store table for jti-based token revocation
-- This allows tracking active tokens and checking for revocation in middleware
-- For high-throughput systems, prefer Redis with TTL instead of DB

CREATE TABLE IF NOT EXISTS token_store (
  jti        TEXT PRIMARY KEY,
  user_id    TEXT NOT NULL,
  tenant_slug TEXT NOT NULL,
  expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_token_store_expires_at ON token_store (expires_at);
CREATE INDEX IF NOT EXISTS idx_token_store_user_id ON token_store (user_id);
