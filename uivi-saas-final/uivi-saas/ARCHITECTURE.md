# UIVI SaaS — System Architecture & Design

## 🏗️ High-Level System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           EXTERNAL CLIENTS / USERS                           │
├─────────────────────────────────────────────────────────────────────────────┤
│  🏛️ Institution Portal │ 🏢 HR Portal │ 👤 User Portal │ ⚖️ Regulatory │ 🛡️ Fraud │
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │
                    ┌──────────────────▼──────────────────┐
                    │    TLS Termination Layer            │
                    │  (nginx / Cloud Load Balancer)      │
                    │  - HTTPS (443 ← 80 redirect)        │
                    │  - HSTS Headers                      │
                    │  - Rate Limiting                     │
                    └──────────────────┬──────────────────┘
                                       │
                    ┌──────────────────▼──────────────────┐
                    │    UIVI SaaS Backend (Go)           │
                    │    Port: 8080 (Internal)            │
                    ├──────────────────────────────────────┤
                    │ CORS Middleware                      │
                    │ JWT Auth Middleware                  │
                    │ Rate Limiting Middleware             │
                    │ Error Handling & Logging             │
                    └──────────────────┬──────────────────┘
                                       │
            ┌──────────────────────────┼──────────────────────────┐
            │                          │                          │
            ▼                          ▼                          ▼
    ┌──────────────┐         ┌──────────────┐         ┌──────────────┐
    │  Auth Module │         │  Cert Module │         │ Verify Module│
    ├──────────────┤         ├──────────────┤         ├──────────────┤
    │ • Login      │         │ • Issuance   │         │ • Cross-     │
    │ • JWT Issue  │         │ • Storage    │         │   tenant     │
    │ • Logout     │         │ • Revocation │         │   verify     │
    │ • Refresh    │         │              │         │ • Validation │
    └──────────────┘         └──────────────┘         └──────────────┘
            │                          │                          │
            │         ┌────────────────┼────────────────┐         │
            │         │                │                │         │
            ▼         ▼                ▼                ▼         ▼
    ┌─────────────────────────────────────────────────────────────────┐
    │                    UIVI SaaS API Handlers                        │
    ├─────────────────────────────────────────────────────────────────┤
    │ • /api/v1/auth/*         (Login, Logout, Refresh, Profile)      │
    │ • /api/v1/cert/*         (Issue, Revoke, Search)                │
    │ • /api/v1/verify/*       (Verify, Status)                       │
    │ • /api/v1/audit/*        (Audit Trail, Consent)                 │
    │ • /api/v1/fraud/*        (Alerts, Velocity, Graph)              │
    │ • /api/v1/regulatory/*   (Stats, Reports, Compliance)           │
    │ • /api/v1/recovery/*     (Guardian, Account Recovery)           │
    │ • /api/v1/tenant/*       (Registry, Onboarding)                 │
    └────────────────┬─────────────────────────────────────────────────┘
                     │
         ┌───────────┼────────────┐
         │           │            │
         ▼           ▼            ▼
    ┌─────────────────────────────────────────────────────────────────┐
    │                    Data Access Layer (ORM)                      │
    │  • SQL Query Builder  • Connection Pooling  • Transactions      │
    └────────────────┬────────────────────────────────────────────────┘
         │           │            │
         ▼           ▼            ▼
    ┌──────────┐ ┌──────────┐ ┌──────────┐
    │ Master DB│ │Tenant DB1│ │Tenant DB2│ ... ┌──────────┐
    │          │ │          │ │          │     │Tenant DBN│
    │uivi_     │ │uivi_     │ │uivi_     │     │uivi_     │
    │master    │ │iit-bombay│ │google-hr │     │naac-reg  │
    │          │ │          │ │          │     │          │
    │• Tenants │ │•Users    │ │•Users    │ ... │•Users    │
    │• Auth    │ │•Certs    │ │•Verif.   │     │•Stats    │
    │• Recovery│ │•Audit    │ │•Audit    │     │•Audit    │
    └──────────┘ └──────────┘ └──────────┘     └──────────┘
```

---

## 🔄 Multi-Tenant Database Architecture

### Master Database (`uivi_master`)
Shared across all tenants; stores tenant registry and cross-tenant references.

```sql
uivi_master/
├── tenants                    -- Tenant registry
│   ├── id (UUID)
│   ├── slug (TEXT UNIQUE)     -- e.g., "iit-bombay"
│   ├── name (TEXT)            -- e.g., "IIT Bombay"
│   ├── org_type (ENUM)        -- INSTITUTION | HR | REGULATORY | FRAUD_MONITOR
│   ├── country (VARCHAR)      -- India, US, EU, UAE, ...
│   ├── status (ENUM)          -- PENDING | ACTIVE | SUSPENDED | DELETED
│   ├── db_name (TEXT)         -- e.g., "uivi_iit_bombay"
│   ├── encryption_key (TEXT)  -- [ENCRYPTED] Master key for this tenant
│   └── created_at, updated_at
│
├── tenant_users               -- Cross-tenant user lookup
│   ├── id (UUID)
│   ├── email (TEXT UNIQUE)
│   ├── tenant_slug (TEXT)     -- FK → tenants.slug
│   ├── user_id_in_tenant (TEXT)
│   └── created_at
│
├── recovery_requests          -- Account recovery (guardian-based)
│   ├── id (UUID)
│   ├── user_email (TEXT)
│   ├── tenant_slug (TEXT)
│   ├── guardian_email (TEXT)
│   ├── status (ENUM)          -- PENDING | APPROVED | REJECTED
│   ├── token (TEXT UNIQUE)    -- One-time recovery token
│   └── created_at, expires_at
│
├── token_store                -- JWT revocation tracking
│   ├── jti (TEXT PRIMARY KEY) -- JWT ID (UUID)
│   ├── user_id (TEXT)
│   ├── tenant_slug (TEXT)
│   ├── expires_at (TIMESTAMP WITH TIME ZONE)
│   └── created_at
│
└── audit_events               -- Platform-wide audit
    ├── id (UUID)
    ├── event_type (TEXT)      -- LOGIN | LOGOUT | CERT_ISSUED | VERIFY_ATTEMPT
    ├── user_id (TEXT)
    ├── tenant_slug (TEXT)
    ├── ip_address (TEXT)
    ├── user_agent (TEXT)
    ├── metadata (JSONB)
    └── created_at
```

### Tenant Database (`uivi_<slug>`)
Isolated database per tenant; contains user, certificate, and audit data for that tenant.

```sql
uivi_<slug>/
├── users
│   ├── id (UUID)
│   ├── email (TEXT UNIQUE)
│   ├── password_hash (TEXT)   -- [HASHED with bcrypt]
│   ├── full_name (TEXT)
│   ├── role (ENUM)            -- USER | INSTITUTION | HR | REGULATORY | FRAUD_L1 | FRAUD_L2
│   ├── mfa_secret (TEXT)      -- [ENCRYPTED] TOTP secret
│   ├── mfa_enabled (BOOLEAN)
│   ├── status (ENUM)          -- ACTIVE | LOCKED | SUSPENDED | DELETED
│   ├── last_login_at (TIMESTAMP)
│   ├── failed_login_count (INT) -- For lockout
│   ├── locked_until (TIMESTAMP) -- Account lockout expiry
│   └── created_at, updated_at
│
├── certificates
│   ├── id (UUID)
│   ├── uivid (TEXT UNIQUE)    -- e.g., "UIVI-2024-DEMO01"
│   ├── issuer_id (UUID)       -- FK → users (institution user)
│   ├── holder_name (TEXT)
│   ├── holder_email (TEXT)
│   ├── credential_type (TEXT) -- CERTIFICATE | DEGREE | LICENSE
│   ├── issue_date (DATE)
│   ├── expiry_date (DATE)
│   ├── credential_data (JSONB)-- Extended data (scores, subjects, etc.)
│   ├── qr_code (TEXT)         -- QR data or image URL
│   ├── status (ENUM)          -- ACTIVE | REVOKED | EXPIRED | SUSPENDED
│   ├── issuer_signature (TEXT)-- [ENCRYPTED] Digital signature
│   └── created_at, revoked_at
│
├── verifications
│   ├── id (UUID)
│   ├── cert_id (UUID)         -- FK → certificates
│   ├── verifier_id (UUID)     -- FK → users (HR company user)
│   ├── verifier_tenant_slug (TEXT) -- Verifier's tenant slug
│   ├── uivid (TEXT)           -- Reference to certificate
│   ├── holder_name_provided (TEXT)
│   ├── verdict (ENUM)         -- VALID | NAME_MISMATCH | REVOKED | NOT_FOUND
│   ├── ip_address (TEXT)
│   ├── user_agent (TEXT)
│   └── created_at
│
├── audit_trail
│   ├── id (UUID)
│   ├── user_id (UUID)         -- User whose record is accessed
│   ├── accessor_tenant_slug (TEXT) -- Who accessed (if cross-tenant)
│   ├── accessor_id (UUID)     -- Who accessed
│   ├── access_type (ENUM)     -- VERIFY | VIEW_AUDIT | DOWNLOAD
│   ├── resource_type (TEXT)   -- CERTIFICATE | PROFILE
│   ├── resource_id (TEXT)     -- UIVID or user_id
│   ├── ip_address (TEXT)
│   ├── user_agent (TEXT)
│   ├── result (TEXT)          -- SUCCESS | DENIED
│   └── created_at
│
├── consent_records
│   ├── id (UUID)
│   ├── user_id (UUID)
│   ├── verifier_tenant_slug (TEXT)
│   ├── consent_type (ENUM)    -- VERIFY | VIEW_PROFILE | DOWNLOAD
│   ├── granted (BOOLEAN)
│   ├── expires_at (TIMESTAMP)
│   └── created_at
│
├── fraud_alerts
│   ├── id (UUID)
│   ├── cert_id (UUID)         -- FK → certificates
│   ├── alert_type (ENUM)      -- VELOCITY_ANOMALY | RELATIONSHIP_ANOMALY | MANUAL_FLAG
│   ├── severity (ENUM)        -- LOW | MEDIUM | HIGH | CRITICAL
│   ├── description (TEXT)
│   ├── metadata (JSONB)       -- velocity_count, graph_distance, etc.
│   ├── status (ENUM)          -- OPEN | INVESTIGATING | RESOLVED | FALSE_POSITIVE
│   ├── assigned_to (UUID)     -- FK → users (fraud analyst)
│   └── created_at, resolved_at
│
├── fraud_graph                -- Entity relationship graph (simplified view)
│   ├── id (UUID)
│   ├── cert_id_1 (UUID)
│   ├── cert_id_2 (UUID)
│   ├── relationship (TEXT)    -- SAME_EMAIL | SAME_PHONE | SAME_ADDRESS | etc.
│   ├── confidence_score (FLOAT) -- 0.0 - 1.0
│   └── created_at
│
├── recovery_guardians
│   ├── id (UUID)
│   ├── user_id (UUID)
│   ├── guardian_email (TEXT)
│   ├── guardian_name (TEXT)
│   ├── recovery_question_1 (TEXT)
│   ├── recovery_answer_1_hash (TEXT)
│   ├── recovery_question_2 (TEXT)
│   ├── recovery_answer_2_hash (TEXT)
│   └── created_at, updated_at
│
└── audit_events
    ├── id (UUID)
    ├── event_type (TEXT)      -- CERT_ISSUED | CERT_REVOKED | VERIFY_ATTEMPT | LOGIN
    ├── user_id (UUID)
    ├── ip_address (TEXT)
    ├── user_agent (TEXT)
    ├── metadata (JSONB)
    └── created_at
```

---

## 🔐 JWT Token Structure & Flow

### Token Payload (Minimal Claims)
```json
{
  "sub": "user-uuid",
  "role": "INSTITUTION",
  "tenant_slug": "iit-bombay",
  "exp": 1719705600,
  "iat": 1719619200,
  "iss": "uivi-saas",
  "jti": "token-uuid"
}
```

### Token Lifecycle
```
1. User Login (POST /api/v1/auth/login)
   ├─ Validate email + password
   ├─ Check account status (not locked, not suspended)
   ├─ Fetch user role & tenant_slug from tenant DB
   ├─ Generate JWT with registered claims
   ├─ Store JTI in master DB token_store
   ├─ Return: { access_token, refresh_token (httpOnly cookie) }
   └─ Clear failed_login_count

2. Request with Token (Any protected endpoint)
   ├─ Extract JWT from Authorization header
   ├─ Verify signature (HS256 or RS256)
   ├─ Validate exp, iat, iss
   ├─ Check JTI in token_store (not revoked)
   ├─ Extract role, tenant_slug
   ├─ Route to tenant DB
   └─ Continue to handler

3. Token Refresh (POST /api/v1/auth/refresh)
   ├─ Extract refresh_token from httpOnly cookie
   ├─ Verify refresh token signature & expiry
   ├─ Check if user is still active
   ├─ Generate new access_token + new refresh_token
   ├─ Delete old JTI from token_store
   ├─ Store new JTI
   └─ Return new tokens

4. Logout (POST /api/v1/auth/logout)
   ├─ Extract JTI from current token
   ├─ Delete JTI from token_store
   ├─ Clear httpOnly cookie
   └─ Return 200 OK (token immediately invalid)

5. Permission Change (Admin demotes user role)
   ├─ Update user.role in tenant DB
   ├─ Optionally: Delete all JTIs for that user (immediate effect)
   ├─ Or: Let tokens expire naturally (delayed effect)
   └─ New login generates token with new role
```

---

## 🌍 Multi-Tenant Data Flow

### Scenario: HR Company Verifies Certificate Issued by Institution

```
Step 1: Institution Tenant (iit-bombay)
┌─────────────────────────────────────┐
│ Institution Portal                  │
│ Issuer (IIT Bombay admin) logs in   │
│ POST /api/v1/cert/issue             │
│ {                                   │
│   holder_name: "Rahul Mehta",       │
│   credential_type: "DEGREE",        │
│   issue_date: "2024-06-01"          │
│ }                                   │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│ Tenant DB: uivi_iit_bombay          │
│ 1. Generate UIVID: "UIVI-2024-xxxx" │
│ 2. Insert into certificates table   │
│ 3. Insert into audit_events         │
│ 4. Notify fraud detection (async)   │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│ Master DB: uivi_master              │
│ Audit: CERT_ISSUED event logged     │
└─────────────────────────────────────┘


Step 2: HR Tenant (google-hr) - Cross-Tenant Verification
┌─────────────────────────────────────┐
│ HR Portal                           │
│ Verifier (Google HR user) logs in   │
│ POST /api/v1/verify/verify          │
│ {                                   │
│   uivid: "UIVI-2024-xxxx",          │
│   holder_name: "Rahul Mehta",       │
│   issuer_institution: "iit-bombay"  │
│ }                                   │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│ Verify Module (in-memory):          │
│ 1. Extract issuer tenant from UIVID │
│ 2. Route to issuer tenant DB        │
│ 3. Query certificate by UIVID       │
│ 4. Validate holder_name match       │
│ 5. Check revocation status          │
│ 6. Get consent from user (if needed)│
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│ Issuer Tenant DB: uivi_iit_bombay   │
│ SELECT * FROM certificates          │
│ WHERE uivid = "UIVI-2024-xxxx"      │
│ ✓ Found: status=ACTIVE              │
│ ✓ holder_name matches               │
│ ✓ not expired                       │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│ User Consent Check (uivi_iit_bombay)│
│ SELECT FROM consent_records WHERE   │
│ verifier_tenant_slug='google-hr'    │
│ AND consent_type='VERIFY'           │
│ ✓ User previously granted consent   │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│ Verifier Tenant DB: uivi_google_hr  │
│ 1. Record verification in           │
│    verifications table              │
│ 2. Audit trail in audit_events      │
│ 3. No detailed cert data written    │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│ Issuer Tenant DB: uivi_iit_bombay   │
│ 1. Record access in audit_trail     │
│ 2. Update verification count        │
│ 3. Log for fraud detection          │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│ Response to HR Portal               │
│ {                                   │
│   verdict: "VALID",                 │
│   uivid: "UIVI-2024-xxxx",          │
│   holder_name_match: true,          │
│   verified_at: "2024-06-14T10:30Z"  │
│ }                                   │
└─────────────────────────────────────┘
```

---

## 🛡️ Security Layers

### Layer 1: TLS/Transport
- TLS 1.3 termination at nginx/load balancer
- HSTS header enforced (max-age=31536000)
- Redirect HTTP → HTTPS

### Layer 2: CORS & Origin Validation
- Whitelist allowed origins (configurable via `ALLOWED_ORIGINS`)
- Reject cross-origin requests from unknown origins
- Credentials (cookies) sent only to whitelisted origins

### Layer 3: Authentication
- Email + password login
- bcrypt password hashing (cost: 12)
- Account lockout after 5 failed attempts (15-min lockout)
- Rate limiting: 5 login attempts per minute per user

### Layer 4: JWT & Authorization
- Strong JWT_SECRET (32+ chars, generated)
- HS256 (symmetric) or RS256 (asymmetric) signing
- Token claims: sub, role, tenant_slug, jti, exp, iat
- Token revocation via JTI lookup in token_store
- Role-based access control (RBAC) in middleware

### Layer 5: Tenant Isolation
- Each tenant has isolated DB
- JWT contains tenant_slug; routes to correct DB
- Cross-tenant queries only via explicit verify flow
- No direct SQL joins across tenant DBs

### Layer 6: Data Encryption
- Passwords: bcrypt
- Sensitive fields: AES-256-GCM (tenant encryption key)
- In-flight: TLS 1.3
- At-rest: Database encryption (RDS encryption, etc.)

### Layer 7: Audit & Monitoring
- All access logged (audit_trail, audit_events)
- Fraud detection: velocity analysis, graph anomalies
- Error tracking: Sentry / Datadog
- Log aggregation: CloudWatch, Splunk, ELK

---

## 🔄 API Endpoint Routing

```
┌────────────────────────────────────────────────────────────────┐
│                    Request Router (Gin)                        │
├────────────────────────────────────────────────────────────────┤
│ CORS Middleware → JWT Middleware → Role Middleware → Handler   │
└────────────┬──────────────────────────────────────────────────┘
             │
    ┌────────┴──────────┬─────────────────┬──────────────┐
    │                   │                 │              │
    ▼                   ▼                 ▼              ▼
/api/v1/auth/*    /api/v1/cert/*    /api/v1/verify/*  /api/v1/audit/*
├─Login            ├─Issue           ├─Verify          ├─Trail
├─Logout           ├─Revoke          ├─Status          ├─Consent
├─Refresh          ├─Search          └─Batch           └─Profile
└─Profile          └─Validate
    │                   │                 │              │
    ▼                   ▼                 ▼              ▼
Master DB +        Tenant DB         Multi-tenant      Tenant DB
Tenant DB          (Issuer)          Query             (Accessor)
```

---

## 📊 Fraud Detection Pipeline

```
┌─────────────────────────────────────────┐
│ Certificate Issued (Event Trigger)      │
│ POST /api/v1/cert/issue                 │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ Async Fraud Analysis Job                │
├─────────────────────────────────────────┤
│ 1. Velocity Check                       │
│    ├─ Query: certs from same issuer     │
│    ├─ Time window: last 24h             │
│    ├─ Count: X certificates issued      │
│    ├─ Threshold: > 100 certs/day        │
│    └─ Alert: "VELOCITY_ANOMALY" [HIGH] │
│                                         │
│ 2. Relationship Analysis                │
│    ├─ Query: certificates by same       │
│    │   holder name / email / phone      │
│    ├─ Build: graph of connections      │
│    ├─ Check: distance between entities  │
│    ├─ Distance: > 2 hops = anomaly      │
│    └─ Alert: "RELATIONSHIP_ANOMALY"    │
│                                         │
│ 3. Pattern Matching                     │
│    ├─ Query: historical patterns        │
│    ├─ Compare: against known fraud      │
│    ├─ Score: confidence 0.0-1.0         │
│    └─ Alert: if score > 0.7             │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ Insert into fraud_alerts                │
│ ├─ status: OPEN                         │
│ ├─ severity: MEDIUM / HIGH              │
│ └─ assigned_to: NULL (unassigned)       │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ Fraud Analyst Dashboard                 │
│ ├─ View: Open alerts                    │
│ ├─ Filter: by severity, type            │
│ ├─ Action: INVESTIGATE / RESOLVE        │
│ └─ Outcome: FALSE_POSITIVE / CONFIRMED  │
└─────────────────────────────────────────┘
```

---

## 🚀 Deployment Topology

### Development (Single Machine)
```
┌─────────────────────────────────────────┐
│ Docker Compose (docker-compose.yml)     │
├─────────────────────────────────────────┤
│ ├─ Service: app                         │
│ │  └─ Image: uivi-saas:latest           │
│ │     Port: 8080 → localhost:8080       │
│ │                                       │
│ ├─ Service: db                          │
│ │  └─ Image: postgres:15                │
│ │     Port: 5432 (internal only)        │
│ │     Volumes: ./data/postgres          │
│ │                                       │
│ └─ Service: redis (optional)            │
│    └─ Image: redis:7-alpine             │
│       Port: 6379 (internal only)        │
│                                         │
│ Networks: uivi-network (bridge)         │
│ Volumes: postgres_data, redis_data      │
└─────────────────────────────────────────┘
```

### Production (Cloud)
```
┌──────────────────────────────────────────────────────────┐
│                    INTERNET                              │
└───────────────────────┬──────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────────┐
│        AWS / GCP / Azure / On-Premise                     │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  ┌─────────────────────────────────────────────────┐   │
│  │ AWS Route 53 / Cloud DNS                        │   │
│  │ ├─ uivi.example.com CNAME → ALB                 │   │
│  │ └─ DNS validation for SSL certificate           │   │
│  └─────────────────────────────────────────────────┘   │
│                                                          │
│  ┌─────────────────────────────────────────────────┐   │
│  │ AWS ALB (Application Load Balancer)             │   │
│  │ ├─ Listener: 443 (HTTPS)                        │   │
│  │ │  ├─ TLS Termination (ACM Certificate)         │   │
│  │ │  ├─ Target Group → ECS / K8s pods (8080)      │   │
│  │ │  └─ Security Group: 443 from 0.0.0.0/0       │   │
│  │ │                                               │   │
│  │ └─ Listener: 80 (HTTP)                          │   │
│  │    └─ Redirect to 443                           │   │
│  └──────────────────────────────────────────────────┘   │
│                                                          │
│  ┌─────────────────────────────────────────────────┐   │
│  │ Container Orchestration (ECS / K8s)             │   │
│  │ ├─ UIVI SaaS Replicas: 3x (for HA)             │   │
│  │ │  ├─ Port: 8080                               │   │
│  │ │  ├─ CPU: 1 vCPU, RAM: 2 GB per pod           │   │
│  │ │  ├─ Health check: /api/v1/health            │   │
│  │ │  └─ Auto-scale: 3-10 pods based on CPU       │   │
│  │ │                                               │   │
│  │ ├─ Memory Cache (Redis) Cluster                │   │
│  │ │  ├─ 3-node cluster (multi-AZ)                │   │
│  │ │  ├─ Purpose: Token revocation, session cache │   │
│  │ │  ├─ Persistence: AOF enabled                 │   │
│  │ │  └─ Security Group: 6379 from app only       │   │
│  │ │                                               │   │
│  │ └─ Job Queue (optional Kafka / SQS)            │   │
│  │    ├─ Fraud detection jobs                     │   │
│  │    ├─ Email notifications                      │   │
│  │    └─ Batch reporting                          │   │
│  └──────────────────────────────────────────────────┘   │
│                                                          │
│  ┌─────────────────────────────────────────────────┐   │
│  │ Database Tier (AWS RDS / GCP Cloud SQL)         │   │
│  │ ├─ Master DB: PostgreSQL 15                     │   │
│  │ │  ├─ Instance: db.r5.large (HA)               │   │
│  │ │  ├─ Multi-AZ failover enabled                │   │
│  │ │  ├─ Backup: automated, 30-day retention      │   │
│  │ │  ├─ Encryption: RDS encryption (AES-256)     │   │
│  │ │  ├─ TLS: sslmode=require                      │   │
│  │ │  ├─ Security Group: 5432 from app only       │   │
│  │ │  └─ Database: uivi_master                    │   │
│  │ │                                               │   │
│  │ └─ Tenant DBs: PostgreSQL 15 (auto-create)     │   │
│  │    ├─ Instance: db.t3.medium (pay-per-tenant)  │   │
│  │    ├─ Backup: 30-day retention                 │   │
│  │    ├─ Encryption: RDS encryption (AES-256)     │   │
│  │    ├─ TLS: sslmode=require                      │   │
│  │    └─ Database: uivi_<slug>                    │   │
│  └──────────────────────────────────────────────────┘   │
│                                                          │
│  ┌─────────────────────────────────────────────────┐   │
│  │ Observability Stack                             │   │
│  │ ├─ Logs: CloudWatch Logs / ELK Stack           │   │
│  │ ├─ Metrics: CloudWatch / Prometheus             │   │
│  │ ├─ Traces: X-Ray / Jaeger                       │   │
│  │ ├─ Error Tracking: Sentry                       │   │
│  │ └─ Alerts: SNS / PagerDuty                      │   │
│  └──────────────────────────────────────────────────┘   │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

---

## 🔑 Key Design Decisions

| Decision | Rationale | Trade-off |
|----------|-----------|-----------|
| **Isolated Tenant DBs** | Data isolation, compliance, scalability per tenant | Operational complexity (manage N DBs), no cross-DB queries |
| **JWT with Minimal Claims** | Reduce PII exposure, smaller token size | Need to fetch user profile on-demand from DB |
| **HS256 (Symmetric JWT)** | Simpler deployment, no key distribution | Single secret compromise = total system compromise |
| **Token Revocation via JTI** | Immediate logout support, fine-grained revocation | Redis/DB lookup cost per request |
| **CORS Whitelist** | Prevent CSRF / token theft from malicious sites | Requires ops to maintain allowed origins list |
| **Multi-Tenant Verification** | Cross-org credential verification without DB access | Complex routing logic, requires explicit consent |
| **Fraud Detection (Post-Issue)** | Async, non-blocking cert issuance | Fraud alerts may lag (seconds/minutes) |
| **Account Lockout** | Brute-force protection | User convenience (must wait 15 min) |
| **httpOnly Cookies for Refresh** | XSS immunity for long-lived tokens | Requires CSRF token for state-changing requests |

---

## 📈 Scalability Considerations

### Horizontal Scaling
- **App Servers**: Stateless Go processes; add replicas behind load balancer
- **Token Store**: Move from DB to Redis cluster (key-value, TTL expiry automatic)
- **Database**: Read replicas for read-heavy queries; sharding by tenant_slug for write scaling

### Vertical Scaling
- **App Server**: Increase CPU/RAM per pod; tune connection pool size
- **Database**: Larger instance type (e.g., db.r5.large → db.r5.4xlarge)
- **Redis**: Larger nodes, clustering with multi-master

### Caching Strategy
- **User Sessions**: Redis (TTL matches token expiry)
- **Tenant Registry**: In-memory cache (refresh on update)
- **Fraud Rules**: In-memory cache (refresh hourly)
- **Certificates (Public Data)**: Optional Redis caching (for high-volume verification)

---

## 🔗 References & Standards

- [OWASP Architecture Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Architectural_Risk_Analysis_Cheat_Sheet.html)
- [Twelve-Factor App Methodology](https://12factor.net/)
- [NIST Secure Software Development Framework (SSDF)](https://pages.nist.gov/ssdf/practices/)
- [W3C Verifiable Credentials Data Model](https://www.w3.org/TR/vc-data-model/)
- [JWT Best Practices (RFC 8725)](https://tools.ietf.org/html/rfc8725)

---

**Last Updated**: June 2024
