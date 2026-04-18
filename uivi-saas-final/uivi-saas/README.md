# UIVI SaaS — Universal Identity & Verification Infrastructure

## ⚡ One-Command Deploy

```bash
unzip uivi-saas-final.zip && cd uivi-saas
docker compose up --build -d
```

Open **http://localhost:8080** — wait ~60 seconds for DB init to complete.

---

## 🔐 Demo Accounts (password: `Demo@1234`)

| Role | Email | Tenant Slug |
|------|-------|-------------|
| 🏛 Institution | issuer@iit-bombay.uivi.app | iit-bombay |
| 🏢 HR Company | hr@google-hr.uivi.app | google-hr |
| 👤 Student/User | student@iit-bombay.uivi.app | iit-bombay |
| ⚖️ Regulatory | regulator@naac-reg.uivi.app | naac-reg |
| 🛡 Fraud Monitor L1 | analyst@fraud-l1.uivi.app | fraud-l1 |

---

## 🧪 Test Flow

1. **Login as Institution** → Issue Certificate → Note the UIVID  
2. **Login as HR** → Verify with that UIVID + correct name → `VALID ✓`  
3. **Login as HR** → Verify with wrong name → `NAME MISMATCH ✗`  
4. **Login as User** → View audit trail of accesses to your UIVID  
5. **Login as Regulatory** → Platform dashboard, compliance report  
6. **Login as Fraud L1** → Fraud alerts, graph visualization  

Demo certificate already seeded: `UIVI-2024-DEMO01` (holder: `Rahul Mehta`)

---

## 🏗 Architecture

- **Multi-tenant**: Each org gets an isolated PostgreSQL DB (`uivi_<slug>`)
- **Master DB** (`uivi_master`): Tenant registry, recovery requests
- **Cross-tenant verify**: HR company verifies UIVID issued by any institution
- **5 portals**: Institution, HR, User, Regulatory, Fraud Monitor
- **JWT**: Contains `tenant_slug + role + user_id` — routes to correct DB

## 📁 Structure

```
uivi-saas/
├── cmd/main.go              # Entry point, all routes
├── internal/
│   ├── auth/                # Multi-tenant login, JWT issuance
│   ├── cert/                # Certificate issuance (institutions)
│   ├── verify/              # Cross-tenant verification (HR)
│   ├── audit/               # Audit trail, consent, profile
│   ├── fraud/               # Fraud dashboard, alerts, graph
│   ├── regulatory/          # Platform stats, compliance
│   ├── recovery/            # Guardian-based account recovery
│   ├── tenant/              # Tenant DB pool, registration
│   └── middleware/          # JWT validation, role enforcement
├── frontend/index.html      # Single-page app, all 5 portals
├── scripts/init-db.sh       # Creates all DBs + seeds demo data
├── docker-compose.yml       # One-command deploy
├── Dockerfile
└── docs/ALL_DOCS.md         # SOP, Patent, Citation, Pitch
```

## 🔧 Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MASTER_DB_URL` | postgres://uivi:uivi@db:5432/uivi_master | Master DB connection |
| `TENANT_DB_HOST` | db | Postgres host for tenant DBs |
| `TENANT_DB_USER` | uivi | Postgres user |
| `TENANT_DB_PASS` | uivi | Postgres password |
| `JWT_SECRET` | uivi-saas-jwt-secret | Change in production! |
| `PORT` | 8080 | HTTP port |

## 🌍 Multi-Country SaaS Customization

Any country can onboard by:
1. `POST /api/v1/tenant/register` with their regulatory body as a tenant
2. Admin approves via `POST /api/v1/admin/tenants/:id/approve`
3. Register users under that tenant
4. Their data stays in `uivi_<slug>` — completely isolated

Supports: India (DPDP 2023), EU (GDPR), UAE, Singapore — configure per country.
