-- ============================================================
-- UIVI SaaS — Database Schema
-- File 1: master DB (uivi_master) — shared across all tenants
-- File 2: tenant DB template (uivi_<slug>) — one per org
-- ============================================================

-- ════════════════════════════════════════════
-- MASTER DATABASE: uivi_master
-- ════════════════════════════════════════════

\c uivi_master;

CREATE TABLE IF NOT EXISTS tenants (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(256) NOT NULL,
    slug        VARCHAR(64)  NOT NULL UNIQUE,   -- used as DB name suffix
    org_type    VARCHAR(32)  NOT NULL,           -- institution|hr|regulatory|government
    country     VARCHAR(64)  NOT NULL DEFAULT 'India',
    plan        VARCHAR(32)  NOT NULL DEFAULT 'free',
    status      VARCHAR(32)  NOT NULL DEFAULT 'pending', -- pending|active|suspended
    created_at  TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS recovery_requests (
    id          VARCHAR(64)  PRIMARY KEY,
    uivid       VARCHAR(32)  NOT NULL,
    tenant_slug VARCHAR(64)  NOT NULL,
    email       VARCHAR(256) NOT NULL,
    status      VARCHAR(32)  NOT NULL DEFAULT 'pending',
    threshold   INT          NOT NULL DEFAULT 2,
    created_at  TIMESTAMP    NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMP    NOT NULL
);

CREATE TABLE IF NOT EXISTS guardian_approvals (
    id          VARCHAR(64)  PRIMARY KEY,
    request_id  VARCHAR(64)  NOT NULL REFERENCES recovery_requests(id),
    guardian_id VARCHAR(128) NOT NULL,
    signature   TEXT,
    approved_at TIMESTAMP    NOT NULL DEFAULT NOW()
);

-- ════════════════════════════════════════════
-- TENANT DATABASE TEMPLATE
-- Run this for EACH tenant: uivi_<slug>
-- Called automatically by init script
-- ════════════════════════════════════════════

-- Switch to tenant DB before running below:
-- \c uivi_<slug>

CREATE TABLE IF NOT EXISTS users (
    id            VARCHAR(64)  PRIMARY KEY,
    email         VARCHAR(256) NOT NULL UNIQUE,
    full_name     VARCHAR(256) NOT NULL,
    role          VARCHAR(32)  NOT NULL, -- institution|hr|user|regulatory|fraud_monitor|admin
    password_hash VARCHAR(256) NOT NULL,
    is_active     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMP    NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

CREATE TABLE IF NOT EXISTS certificates (
    id              VARCHAR(64)  PRIMARY KEY,
    uivid           VARCHAR(32)  NOT NULL UNIQUE,
    credential_hash VARCHAR(64)  NOT NULL UNIQUE,
    holder_name     VARCHAR(256) NOT NULL,
    holder_email    VARCHAR(256),
    holder_dob      VARCHAR(32),
    degree_type     VARCHAR(64)  NOT NULL,
    degree_name     VARCHAR(256) NOT NULL,
    specialization  VARCHAR(256),
    roll_number     VARCHAR(128) NOT NULL,
    passing_year    INT          NOT NULL,
    grade           VARCHAR(64),
    issuer_id       VARCHAR(64)  NOT NULL,
    issuer_name     VARCHAR(256) NOT NULL,
    status          VARCHAR(32)  NOT NULL DEFAULT 'active', -- active|revoked
    revoke_reason   TEXT,
    created_at      TIMESTAMP    NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_certs_uivid ON certificates(uivid);
CREATE INDEX IF NOT EXISTS idx_certs_holder ON certificates(holder_name);
CREATE INDEX IF NOT EXISTS idx_certs_status ON certificates(status);

CREATE TABLE IF NOT EXISTS verifications (
    id                       VARCHAR(64)  PRIMARY KEY,
    uivid                    VARCHAR(32)  NOT NULL,
    candidate_name_provided  VARCHAR(256) NOT NULL,
    verifier_user_id         VARCHAR(64),
    purpose                  VARCHAR(128),
    result_valid             BOOLEAN      NOT NULL,
    verified_at              TIMESTAMP    NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_verif_uivid ON verifications(uivid);
CREATE INDEX IF NOT EXISTS idx_verif_at ON verifications(verified_at DESC);

CREATE TABLE IF NOT EXISTS audit_events (
    id         VARCHAR(64)  PRIMARY KEY,
    user_id    VARCHAR(64),
    action     VARCHAR(64)  NOT NULL,
    details    TEXT         NOT NULL DEFAULT '{}',
    created_at TIMESTAMP    NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_events(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_events(action);
CREATE INDEX IF NOT EXISTS idx_audit_at ON audit_events(created_at DESC);

CREATE TABLE IF NOT EXISTS consent_preferences (
    user_id    VARCHAR(64)  PRIMARY KEY,
    mode       VARCHAR(32)  NOT NULL DEFAULT 'PASSIVE',
    updated_at TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fraud_checks (
    id          VARCHAR(64)  PRIMARY KEY,
    uivid       VARCHAR(32),
    risk_score  INT          NOT NULL DEFAULT 0,
    risk_level  VARCHAR(32)  NOT NULL DEFAULT 'LOW',
    signals     TEXT         NOT NULL DEFAULT '[]',
    flagged     BOOLEAN      NOT NULL DEFAULT FALSE,
    status      VARCHAR(32)  NOT NULL DEFAULT 'open',
    notes       TEXT,
    created_at  TIMESTAMP    NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_fraud_flagged ON fraud_checks(flagged);

CREATE TABLE IF NOT EXISTS fraud_graph (
    id          SERIAL PRIMARY KEY,
    source_type VARCHAR(32)  NOT NULL,
    source_id   VARCHAR(128) NOT NULL,
    target_type VARCHAR(32)  NOT NULL,
    target_id   VARCHAR(128) NOT NULL,
    linked_at   TIMESTAMP    NOT NULL DEFAULT NOW(),
    UNIQUE(source_type, source_id, target_type, target_id)
);
