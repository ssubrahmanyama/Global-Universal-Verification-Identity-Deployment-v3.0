#!/usr/bin/env bash
set -e

PG="psql -U uivi -h db -d postgres"
echo "Waiting for Postgres..."
until $PG -c '\q' 2>/dev/null; do sleep 2; done
echo "Postgres ready."

# ── Master DB ─────────────────────────────────────────────
$PG -d uivi_master << 'SQL'
CREATE TABLE IF NOT EXISTS tenants (
  id          SERIAL PRIMARY KEY,
  name        VARCHAR(256) NOT NULL,
  slug        VARCHAR(64)  NOT NULL UNIQUE,
  org_type    VARCHAR(32)  NOT NULL,
  country     VARCHAR(64)  NOT NULL DEFAULT 'India',
  plan        VARCHAR(32)  NOT NULL DEFAULT 'free',
  status      VARCHAR(32)  NOT NULL DEFAULT 'pending',
  created_at  TIMESTAMP    NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS recovery_requests (
  id          VARCHAR(64) PRIMARY KEY,
  uivid       VARCHAR(32),
  tenant_slug VARCHAR(64),
  email       VARCHAR(256),
  status      VARCHAR(32) DEFAULT 'pending',
  threshold   INT DEFAULT 2,
  created_at  TIMESTAMP DEFAULT NOW(),
  expires_at  TIMESTAMP
);
CREATE TABLE IF NOT EXISTS guardian_approvals (
  id          VARCHAR(64) PRIMARY KEY,
  request_id  VARCHAR(64),
  guardian_id VARCHAR(128),
  signature   TEXT,
  approved_at TIMESTAMP DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS certificate_index (
  uivid          VARCHAR(32) PRIMARY KEY,
  tenant_slug    VARCHAR(64)  NOT NULL,
  issuer_name    VARCHAR(256) NOT NULL,
  holder_name    VARCHAR(256) NOT NULL,
  holder_email   VARCHAR(256),
  holder_dob     VARCHAR(32),
  degree_type    VARCHAR(64)  NOT NULL,
  degree_name    VARCHAR(256) NOT NULL,
  specialization VARCHAR(256),
  roll_number    VARCHAR(128) NOT NULL,
  passing_year   INT          NOT NULL,
  grade          VARCHAR(64),
  status         VARCHAR(32)  NOT NULL DEFAULT 'active',
  revoke_reason  TEXT,
  created_at     TIMESTAMP    NOT NULL DEFAULT NOW(),
  revoked_at     TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_cert_index_tenant_slug ON certificate_index(tenant_slug);
CREATE INDEX IF NOT EXISTS idx_cert_index_status ON certificate_index(status);
INSERT INTO tenants (name,slug,org_type,country,plan,status) VALUES
  ('IIT Bombay',       'iit-bombay',   'institution', 'India', 'enterprise', 'active'),
  ('Mumbai University','mumbai-univ',  'institution', 'India', 'standard',   'active'),
  ('Google HR India',  'google-hr',    'hr',          'India', 'enterprise', 'active'),
  ('Accenture HR',     'accenture-hr', 'hr',          'India', 'enterprise', 'active'),
  ('NAAC Regulatory',  'naac-reg',     'regulatory',  'India', 'government', 'active'),
  ('Fraud Monitor L1', 'fraud-l1',     'government',  'India', 'enterprise', 'active')
ON CONFLICT(slug) DO NOTHING;
SQL

# ── Tenant DB creator ─────────────────────────────────────
create_tenant() {
  local slug=$1
  local db="uivi_$(echo "$slug" | tr '-' '_')"

  $PG -c "CREATE DATABASE ${db};" 2>/dev/null || echo "DB ${db} already exists"

  $PG -d "$db" << 'TSQL'
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
  id            VARCHAR(64)  PRIMARY KEY,
  email         VARCHAR(256) NOT NULL UNIQUE,
  full_name     VARCHAR(256) NOT NULL,
  role          VARCHAR(32)  NOT NULL,
  password_hash VARCHAR(256) NOT NULL,
  is_active     BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at    TIMESTAMP    NOT NULL DEFAULT NOW()
);
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
  status          VARCHAR(32)  NOT NULL DEFAULT 'active',
  revoke_reason   TEXT,
  created_at      TIMESTAMP    NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS verifications (
  id                      VARCHAR(64)  PRIMARY KEY,
  uivid                   VARCHAR(32)  NOT NULL,
  candidate_name_provided VARCHAR(256) NOT NULL,
  verifier_user_id        VARCHAR(64),
  purpose                 VARCHAR(128),
  result_valid            BOOLEAN      NOT NULL,
  verified_at             TIMESTAMP    NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS audit_events (
  id         VARCHAR(64) PRIMARY KEY,
  user_id    VARCHAR(64),
  action     VARCHAR(64) NOT NULL,
  details    TEXT        NOT NULL DEFAULT '{}',
  created_at TIMESTAMP   NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS consent_preferences (
  user_id    VARCHAR(64) PRIMARY KEY,
  mode       VARCHAR(32) NOT NULL DEFAULT 'PASSIVE',
  updated_at TIMESTAMP   NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS fraud_checks (
  id         VARCHAR(64) PRIMARY KEY,
  uivid      VARCHAR(32),
  risk_score INT         NOT NULL DEFAULT 0,
  risk_level VARCHAR(32) NOT NULL DEFAULT 'LOW',
  signals    TEXT        NOT NULL DEFAULT '[]',
  flagged    BOOLEAN     NOT NULL DEFAULT FALSE,
  status     VARCHAR(32) NOT NULL DEFAULT 'open',
  notes      TEXT,
  created_at TIMESTAMP   NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS fraud_graph (
  id          SERIAL      PRIMARY KEY,
  source_type VARCHAR(32),
  source_id   VARCHAR(128),
  target_type VARCHAR(32),
  target_id   VARCHAR(128),
  linked_at   TIMESTAMP   NOT NULL DEFAULT NOW(),
  UNIQUE(source_type, source_id, target_type, target_id)
);
TSQL

  echo "✓ Created tenant DB: ${db}"
}

# ── Create all tenant DBs ──────────────────────────────────
for slug in iit-bombay mumbai-univ google-hr accenture-hr naac-reg fraud-l1; do
  create_tenant "$slug"
done

# ── Seed users using pgcrypto (no hardcoded hash!) ─────────
# Password for all demo accounts: Demo@1234

$PG -d uivi_iit_bombay << 'SQL'
INSERT INTO users (id, email, full_name, role, password_hash) VALUES
  ('u1', 'issuer@iit-bombay.uivi.app',  'Prof. Sharma (IIT Bombay)', 'institution',
   crypt('Demo@1234', gen_salt('bf', 10))),
  ('u2', 'student@iit-bombay.uivi.app', 'Rahul Mehta',               'user',
   crypt('Demo@1234', gen_salt('bf', 10)))
ON CONFLICT(email) DO NOTHING;
SQL

$PG -d uivi_google_hr << 'SQL'
INSERT INTO users (id, email, full_name, role, password_hash) VALUES
  ('u3', 'hr@google-hr.uivi.app', 'Priya Singh (Google HR)', 'hr',
   crypt('Demo@1234', gen_salt('bf', 10)))
ON CONFLICT(email) DO NOTHING;
SQL

$PG -d uivi_accenture_hr << 'SQL'
INSERT INTO users (id, email, full_name, role, password_hash) VALUES
  ('u6', 'hr@accenture-hr.uivi.app', 'Anita Roy (Accenture HR)', 'hr',
   crypt('Demo@1234', gen_salt('bf', 10)))
ON CONFLICT(email) DO NOTHING;
SQL

$PG -d uivi_naac_reg << 'SQL'
INSERT INTO users (id, email, full_name, role, password_hash) VALUES
  ('u4', 'regulator@naac-reg.uivi.app', 'Dr. Kumar (NAAC)', 'regulatory',
   crypt('Demo@1234', gen_salt('bf', 10)))
ON CONFLICT(email) DO NOTHING;
SQL

$PG -d uivi_fraud_l1 << 'SQL'
INSERT INTO users (id, email, full_name, role, password_hash) VALUES
  ('u5', 'analyst@fraud-l1.uivi.app', 'Arun Das (Fraud L1)', 'fraud_monitor',
   crypt('Demo@1234', gen_salt('bf', 10)))
ON CONFLICT(email) DO NOTHING;
SQL

# ── Seed demo certificate ──────────────────────────────────
$PG -d uivi_iit_bombay << 'SQL'
INSERT INTO certificates (
  id, uivid, credential_hash, holder_name, holder_email,
  degree_type, degree_name, roll_number, passing_year, grade,
  issuer_id, issuer_name
) VALUES (
  'cert-demo-001',
  'UIVI-2024-DEMO01',
  'a3f9b2c1d4e5f6a7b8c9d0e1f2a3b4c5',
  'Rahul Mehta',
  'student@iit-bombay.uivi.app',
  'B.Tech',
  'Computer Science & Engineering',
  'IIT-2024-CS-001',
  2024,
  'First Class with Distinction',
  'u1',
  'IIT Bombay'
) ON CONFLICT(uivid) DO NOTHING;
SQL

echo ""
echo "============================================"
echo "✅ UIVI database initialisation complete"
echo "============================================"
echo ""
echo "Demo accounts — password: Demo@1234"
echo "  Institution : issuer@iit-bombay.uivi.app   | tenant: iit-bombay"
echo "  HR (Google) : hr@google-hr.uivi.app         | tenant: google-hr"
echo "  Student     : student@iit-bombay.uivi.app  | tenant: iit-bombay"
echo "  Regulatory  : regulator@naac-reg.uivi.app  | tenant: naac-reg"
echo "  Fraud L1    : analyst@fraud-l1.uivi.app    | tenant: fraud-l1"
echo ""
echo "Demo cert: UIVI-2024-DEMO01  holder: Rahul Mehta"
echo "============================================"
