# UIVI SaaS — Product Requirements Document (PRD)

**Version**: 1.0  
**Last Updated**: June 2024  
**Status**: Active Development  

---

## 📋 Executive Summary

UIVI (Global Universal Verification & Identity) is a **multi-tenant SaaS platform** for issuing, verifying, and managing digital verifiable credentials (academic certificates, degrees, licenses). The platform enables organizations to issue tamper-proof credentials and allows third parties (HR companies, regulatory bodies) to verify them across organizational boundaries—all while maintaining strict data isolation, user consent, and fraud detection.

**Key Value Proposition**:
- **For Institutions (Issuers)**: Issue blockchain-verifiable credentials without managing infrastructure
- **For HR Companies (Verifiers)**: Verify credentials instantly without contacting institutions directly
- **For Users (Credential Holders)**: Control access to their credentials with granular consent management
- **For Regulators**: Real-time compliance audits and fraud detection across the platform
- **For Fraud Teams**: Detect and investigate credential fraud using velocity and relationship analysis

---

## 🎯 Product Goals

### Primary Goals (MVP)
1. **Multi-Tenant Isolation**: Each organization operates in a separate database; no data leakage
2. **Cross-Tenant Verification**: HR can verify certificates issued by any institution without direct DB access
3. **Role-Based Access Control**: Five distinct user roles with differentiated permissions
4. **Audit Trail**: Every access to credentials is logged with full context (who, when, what, why)
5. **Fraud Detection**: Identify suspicious credential issuance patterns in real-time
6. **Account Recovery**: Users can recover accounts via guardian-based verification

### Secondary Goals (Roadmap)
1. **Multi-Country Compliance**: Support DPDP Act (India), GDPR (EU), and country-specific regulations
2. **Blockchain Integration**: Anchor credential hashes on public ledger for permanent audit trail
3. **API for Third Parties**: Allow external systems (university portals, job platforms) to integrate
4. **Mobile App**: Mobile-friendly credential viewing and sharing
5. **Credential Wallets**: Users can aggregate credentials from multiple institutions
6. **Batch Operations**: Bulk issuance and verification for enterprises

---

## 👥 User Personas & Roles

### 1. **Institution Administrator** (Issuer)
- **Goal**: Issue verifiable credentials to graduates
- **Permissions**: Issue certificates, revoke, search issued certs, view audit trail
- **Typical Workflow**: 
  1. Login to institution portal
  2. Issue certificate for student (name, credential type, dates)
  3. Receive UIVID (e.g., UIVI-2024-A001)
  4. Notify student via email with UIVID
  5. View fraud alerts for anomalies

### 2. **HR Recruiter / Verifier** (Verifier)
- **Goal**: Verify candidate credentials during hiring
- **Permissions**: Verify certificates by UIVID, view verification history, download verification reports
- **Typical Workflow**:
  1. Login to HR company portal
  2. Search for candidate by name / email
  3. Enter UIVID provided by candidate
  4. System verifies credential against issuing institution
  5. Receive VALID / NAME_MISMATCH / REVOKED verdict
  6. Record verification in candidate file

### 3. **User / Credential Holder**
- **Goal**: View and manage their credentials; control who can access them
- **Permissions**: View issued credentials, grant/revoke verification consent, view audit trail, download credentials, delete account
- **Typical Workflow**:
  1. Login to user portal
  2. View list of credentials issued to them
  3. See who has verified their credentials (access audit trail)
  4. Grant consent to new verifiers
  5. Revoke access if needed

### 4. **Regulatory Officer**
- **Goal**: Monitor platform compliance; ensure data protection regulations are met
- **Permissions**: View platform-wide statistics, run compliance reports, view audit logs, investigate complaints
- **Typical Workflow**:
  1. Login to regulatory dashboard
  2. View monthly compliance metrics (total certs, verifications, user complaints)
  3. Run GDPR data export for specific user
  4. Review audit trail for data deletion requests
  5. Generate compliance certificate for auditors

### 5. **Fraud Analyst (L1 / L2)**
- **Goal**: Investigate and resolve fraud alerts; prevent credential fraud
- **Permissions**: View fraud alerts, investigate certificates, flag/resolve alerts, view fraud graph
- **Typical Workflow**:
  1. Login to fraud dashboard
  2. See list of open high-severity alerts
  3. Click on alert to investigate
  4. View fraud graph (related certificates, relationships)
  5. Mark as confirmed fraud or false positive
  6. If confirmed, recommend revocation to institution

---

## 🔐 Core Features & User Stories

### Feature 1: Multi-Tenant Authentication & JWT

**User Story**: "As an institution admin, I want to login with my email/password and receive a secure token that identifies me and my organization."

**Acceptance Criteria**:
- [ ] Login endpoint accepts email + password
- [ ] Password is validated against bcrypt hash (cost 12)
- [ ] Account lockout after 5 failed attempts (15-min cooldown)
- [ ] JWT generated with: role, tenant_slug, exp (15 min), iat, jti, sub
- [ ] Token stored in memory (frontend) or httpOnly cookie (refresh token)
- [ ] JTI tracked in `token_store` for revocation
- [ ] Logout invalidates JTI immediately
- [ ] Refresh endpoint allows new token without re-login

**Technical Details**:
- Endpoint: `POST /api/v1/auth/login`
- Request: `{ email: string, password: string }`
- Response: `{ access_token: string, refresh_token?: string, expires_in: number }`
- Error Codes: 400 (invalid email), 401 (wrong password), 429 (rate limit), 500 (server error)

---

### Feature 2: Certificate Issuance (Institution Portal)

**User Story**: "As an institution, I want to issue a verifiable certificate to a graduate with a unique UIVID."

**Acceptance Criteria**:
- [ ] Issuance form has fields: holder_name, holder_email, credential_type, issue_date, expiry_date, additional metadata
- [ ] Certificate stored with unique UIVID (format: UIVI-YYYY-XXXXX)
- [ ] UIVID sent to holder via email with QR code
- [ ] Digital signature created and stored (issuer's private key)
- [ ] Fraud detection job triggered asynchronously
- [ ] Audit event logged: CERT_ISSUED
- [ ] Certificate status set to ACTIVE (searchable by HR)

**Technical Details**:
- Endpoint: `POST /api/v1/cert/issue`
- Request:
  ```json
  {
    "holder_name": "Rahul Mehta",
    "holder_email": "rahul@example.com",
    "credential_type": "DEGREE",
    "issue_date": "2024-06-01",
    "expiry_date": "2034-06-01",
    "metadata": { "gpa": "3.8", "department": "Computer Science" }
  }
  ```
- Response: `{ uivid: string, qr_code_url: string, issued_at: timestamp }`
- Fraud Checks: velocity (issuance rate), relationship (duplicate holders)

---

### Feature 3: Cross-Tenant Verification (HR Portal)

**User Story**: "As an HR recruiter, I want to verify a candidate's degree by entering the UIVID and their name, without accessing the institution's database."

**Acceptance Criteria**:
- [ ] Verification form has fields: UIVID, holder_name, issuer_institution (auto-inferred from UIVID)
- [ ] System queries issuing institution's DB (not full DB access, only the certificate record)
- [ ] Name matched against holder_name in certificate (exact or fuzzy)
- [ ] Certificate status checked: ACTIVE, not REVOKED, not EXPIRED
- [ ] User consent validated: does holder allow HR company to verify?
- [ ] Result returned: VALID / NAME_MISMATCH / REVOKED / EXPIRED / NOT_FOUND
- [ ] Verification recorded in both issuer and verifier tenant DBs
- [ ] Audit trail updated: ACCESS_LOG entry in issuer DB (for user to see who verified)

**Technical Details**:
- Endpoint: `POST /api/v1/verify/verify`
- Request:
  ```json
  {
    "uivid": "UIVI-2024-A001",
    "holder_name": "Rahul Mehta",
    "issuer_institution": "iit-bombay"
  }
  ```
- Response:
  ```json
  {
    "verdict": "VALID",
    "uivid": "UIVI-2024-A001",
    "holder_name_match": true,
    "verified_at": "2024-06-14T10:30:00Z",
    "credential_type": "DEGREE",
    "issue_date": "2024-06-01",
    "expiry_date": "2034-06-01"
  }
  ```
- Error Codes: 400 (invalid UIVID format), 404 (certificate not found), 403 (consent denied), 410 (credential revoked)

---

### Feature 4: Audit Trail & User Consent

**User Story**: "As a user, I want to see who has accessed my credentials and grant/revoke consent to specific verifiers."

**Acceptance Criteria**:
- [ ] User can view audit trail: timestamp, verifier, verification result, IP address
- [ ] Audit trail shows all verification attempts (successful and failed)
- [ ] Consent management UI allows: Grant Consent (permanent until revoked), Revoke Consent (immediate effect)
- [ ] Revoke consent blocks future verifications from that verifier (even if previous consent was given)
- [ ] Consent expiry date can be set (e.g., 1 year validity)
- [ ] New verification from previously-unauthorized verifier returns 403 (consent denied)

**Technical Details**:
- Endpoint: `GET /api/v1/audit/trail?resource_id=<UIVID>`
- Response: Array of audit events with timestamp, accessor, result, IP
- Endpoint: `POST /api/v1/audit/consent`
- Request: `{ verifier_tenant_slug: string, consent_type: "VERIFY" | "VIEW_PROFILE", granted: boolean, expires_at?: timestamp }`

---

### Feature 5: Fraud Detection & Alerts

**User Story**: "As a fraud analyst, I want to be alerted when suspicious credential issuance patterns are detected and investigate related credentials."

**Acceptance Criteria**:
- [ ] Velocity Check: Alert if issuer issues > 100 certs/day (configurable threshold)
- [ ] Relationship Check: Alert if multiple certs issued to same person (different names, same email/phone)
- [ ] Velocity alerts are MEDIUM severity; relationship alerts are HIGH severity
- [ ] Fraud analyst dashboard displays open alerts, filtered by severity
- [ ] Analyst can mark alert as: INVESTIGATING, RESOLVED, FALSE_POSITIVE
- [ ] Graph visualization shows relationships between related certificates (connected entities)
- [ ] Resolved alerts archived; false positives don't re-trigger

**Technical Details**:
- Endpoint: `GET /api/v1/fraud/alerts?severity=HIGH&status=OPEN`
- Response: Array of fraud alerts with cert_id, alert_type, description, metadata
- Endpoint: `GET /api/v1/fraud/graph?cert_id=<id>`
- Response: Fraud graph with related certs, relationships, confidence scores

---

### Feature 6: Account Recovery (Guardian-Based)

**User Story**: "As a user, I forgot my password. I want to recover my account by answering questions set by my guardian."

**Acceptance Criteria**:
- [ ] User sets 2-3 guardians (email) during account setup
- [ ] For each guardian, user sets 2 security questions + answers (hashed)
- [ ] Password recovery flow:
  1. User enters email → system sends email with one-time recovery link
  2. Email link directs to recovery page asking security questions
  3. User answers questions correctly → new password reset link sent
  4. User sets new password
- [ ] Recovery requests tracked in master DB (audit trail)
- [ ] Recovery tokens expire after 1 hour or on first use

**Technical Details**:
- Endpoint: `POST /api/v1/recovery/initiate`
- Request: `{ email: string }`
- Response: `{ message: "Recovery email sent", recovery_token_expiry: 3600 }`
- Endpoint: `POST /api/v1/recovery/verify-guardian`
- Request: `{ recovery_token: string, answers: { question_1: string, question_2: string } }`
- Response: `{ success: boolean, reset_link: string }`

---

### Feature 7: Regulatory Compliance & Reporting

**User Story**: "As a regulatory officer, I want to generate a compliance report showing GDPR data exports and audit trails."

**Acceptance Criteria**:
- [ ] Compliance dashboard shows:
  - Total users (by tenant)
  - Total certificates issued (by month)
  - Total verifications (by month)
  - User complaints / data deletion requests
- [ ] GDPR export: Download user's personal data (email, name, credentials, audit trail)
- [ ] Data deletion: User can request account deletion; system anonymizes data in audit logs
- [ ] Compliance report: PDF with metrics, audit summary, data handling procedures
- [ ] Reports are timestamped and signed (digital signature)

**Technical Details**:
- Endpoint: `GET /api/v1/regulatory/stats?start_date=<date>&end_date=<date>`
- Endpoint: `POST /api/v1/regulatory/gdpr-export?user_id=<id>`
- Response: ZIP file with user data in JSON/CSV format
- Endpoint: `POST /api/v1/regulatory/report-generate?format=pdf`

---

### Feature 8: Revocation & Credential Lifecycle

**User Story**: "As an institution, I want to revoke a credential if it was issued in error or if the graduate's status changed."

**Acceptance Criteria**:
- [ ] Revocation endpoint allows institution admin to revoke certificate by UIVID
- [ ] Revoked certificate status set to REVOKED
- [ ] Future verification attempts return: `{ verdict: "REVOKED", revoked_at: timestamp }`
- [ ] Revocation reason stored (optional)
- [ ] Revocation audit event logged
- [ ] User notified via email of revocation
- [ ] Revocation cannot be undone (deleted forever); only re-issue possible

**Technical Details**:
- Endpoint: `POST /api/v1/cert/revoke`
- Request: `{ uivid: string, reason?: string }`
- Response: `{ revoked: boolean, revoked_at: timestamp }`

---

## 📊 Non-Functional Requirements

### Performance
- **Login**: < 200ms
- **Verification**: < 500ms (cross-tenant query + consent check + audit log)
- **Certificate Issuance**: < 1s (sync); fraud detection < 30s (async)
- **Throughput**: 1,000 verifications/sec (at peak)
- **Database Query**: All queries execute in < 100ms (indexed columns)

### Scalability
- **Concurrent Users**: 10,000+
- **Certificates Per Tenant**: 10 million+
- **Data Storage**: 1 TB+ (multi-tenant)
- **Horizontal Scaling**: Stateless app servers behind load balancer
- **Database Sharding**: Optional; each tenant gets dedicated DB

### Availability
- **SLA**: 99.9% uptime (3 9s; 43.2 min downtime/month)
- **Recovery Time Objective (RTO)**: < 1 hour
- **Recovery Point Objective (RPO)**: < 15 minutes
- **Backup**: Automated daily; 30-day retention

### Security
- **Authentication**: Email + password (bcrypt) + optional MFA (TOTP)
- **Authorization**: Role-based access control (RBAC)
- **Encryption**: TLS 1.3 in-transit; AES-256-GCM at-rest
- **Data Isolation**: Each tenant in separate DB; no cross-DB queries
- **Audit Logging**: All access logged with timestamp, user, IP, action
- **Compliance**: DPDP Act 2023, GDPR, SOC 2 Type II

### Compliance
- **DPDP Act 2023** (India): User consent for data processing, data deletion on request, audit trails
- **GDPR** (EU): Data portability, right to be forgotten, consent withdrawal, DPA compliance
- **W3C Verifiable Credentials**: Support standard credential format for interoperability

---

## 🗺️ User Journeys

### Journey 1: Institution Issues Certificate

```
Institution Admin
    ↓
Login (email + password)
    ↓
Dashboard (home)
    ↓
"Issue Certificate" button
    ↓
Form (holder_name, email, credential_type, dates, metadata)
    ↓
Submit
    ↓
[Backend: Generate UIVID, create cert record, trigger fraud check async]
    ↓
Success page (UIVID: UIVI-2024-A001, QR code)
    ↓
System sends email to holder with UIVID + QR
    ↓
Institution views "My Issued Certificates" (searchable)
    ↓
[Async] Fraud system analyzes for velocity/relationship issues
    ↓
If fraud detected, analyst notified
```

### Journey 2: HR Verifies Certificate

```
HR Recruiter
    ↓
Login (email + password)
    ↓
Dashboard (home)
    ↓
"Verify Certificate" button
    ↓
Form (UIVID, holder_name, issuer_institution [optional])
    ↓
Submit
    ↓
[Backend: Query issuer tenant DB, validate consent, cross-check]
    ↓
Result: VALID / NAME_MISMATCH / REVOKED / EXPIRED / NOT_FOUND
    ↓
Display result with credential details (if VALID)
    ↓
Option: Download Verification Report (PDF)
    ↓
Verification recorded in HR company's verification history
```

### Journey 3: User Manages Credentials & Consent

```
User (Credential Holder)
    ↓
Login (email + password)
    ↓
Dashboard: "My Credentials"
    ↓
List of certificates issued to them
    ↓
Click on certificate → View details
    ↓
Tab: "Audit Trail"
    ↓
See list of who verified their credential (verifier, timestamp, result, IP)
    ↓
Click on verifier → "Grant / Revoke Consent"
    ↓
Modal: Consent history, consent expiry date, revoke button
    ↓
Submit
    ↓
Consent updated in system
    ↓
Future verification from revoked verifier rejected (403)
```

---

## 📱 UI/UX Mockups (Text Description)

### Institution Portal - Certificate Issuance Page

```
┌───────────────────────────────────────────────────────────┐
│ UIVI — Institution Portal                 [User] [Logout] │
├───────────────────────────────────────────────────────────┤
│ Left Sidebar:                                             │
│ ├─ Dashboard                                              │
│ ├─ Issue Certificate [ACTIVE]                             │
│ ├─ My Certificates                                        │
│ ├─ Fraud Alerts (3)                                       │
│ └─ Settings                                               │
├───────────────────────────────────────────────────────────┤
│ Main Content:                                             │
│                                                            │
│ Issue New Certificate                                     │
│ ┌─────────────────────────────────────┐                   │
│ │ Holder Full Name:  [_______________]│                   │
│ │ Holder Email:      [_______________]│                   │
│ │ Credential Type:   [Dropdown ▼]     │                   │
│ │ Issue Date:        [Date picker]    │                   │
│ │ Expiry Date:       [Date picker]    │                   │
│ │ Additional Info:   [Large text area]│                   │
│ │                                     │                   │
│ │ [Cancel]  [Issue Certificate]       │                   │
│ └─────────────────────────────────────┘                   │
└───────────────────────────────────────────────────────────┘
```

### HR Portal - Verification Results Page

```
┌───────────────────────────────────────────────────────────┐
│ UIVI — HR Verification Portal         [User] [Logout]     │
├───────────────────────────────────────────────────────────┤
│ Left Sidebar:                                             │
│ ├─ Dashboard                                              │
│ ├─ Verify Certificate [ACTIVE]                            │
│ ├─ Verification History                                   │
│ └─ Settings                                               │
├───────────────────────────────────────────────────────────┤
│                                                            │
│ Verification Result                                       │
│                                                            │
│ ✓ VALID                                                   │
│                                                            │
│ UIVID:          UIVI-2024-A001                            │
│ Holder Name:    Rahul Mehta ✓ (matches)                   │
│ Credential:     Bachelor of Technology                    │
│ Issue Date:     June 1, 2024                              │
│ Expiry Date:    June 1, 2034                              │
│ Issuer:         IIT Bombay                                │
│ Verified At:    June 14, 2024 10:30 AM                    │
│                                                            │
│ [Download Verification Report] [Back]                     │
│                                                            │
└───────────────────────────────────────────────────────────┘
```

---

## 📈 Success Metrics & KPIs

| Metric | Target | Measurement |
|--------|--------|-------------|
| **Platform Availability** | 99.9% | Uptime monitoring (CloudWatch) |
| **Average Login Time** | < 200ms | APM (Application Performance Monitoring) |
| **Average Verification Time** | < 500ms | API latency logs |
| **Credential Issuance Rate** | 10K/day | Database audit events |
| **Cross-Tenant Verification Rate** | 1K/day | Verification table row count |
| **Fraud Detection Rate** | 95%+ | Manual audit of fraud alerts |
| **User Retention** | 80%+ (6-month) | User login frequency |
| **Tenant Satisfaction (NPS)** | 50+ | Quarterly survey |
| **Data Deletion Requests (GDPR)** | < 30 days processing | Regulatory audit logs |
| **Security Incidents** | 0 (target) | Incident response logs |

---

## 🚀 Roadmap & Phases

### Phase 1: MVP (June 2024) ✅
- [x] Multi-tenant architecture with isolated DBs
- [x] 5 user roles with RBAC
- [x] Cert issuance & cross-tenant verification
- [x] Audit trail & consent management
- [x] Fraud detection (velocity + relationships)
- [x] Account recovery (guardian-based)
- [x] Security hardening (JWT, TLS, CORS)

### Phase 2: Compliance & Scale (Sept 2024)
- [ ] GDPR data export & deletion
- [ ] Multi-country support (India, EU, UAE, Singapore)
- [ ] Rate limiting & brute-force protection
- [ ] Redis-based token revocation (faster than DB)
- [ ] Refresh token flow (httpOnly cookies)
- [ ] Email notifications (issuance, verification, alerts)
- [ ] API documentation (OpenAPI/Swagger)

### Phase 3: Blockchain & Interop (Dec 2024)
- [ ] Blockchain integration (Ethereum / Hyperledger)
- [ ] Anchor certificate hashes on public ledger
- [ ] JWKS endpoint for public key distribution
- [ ] Migration from HS256 → RS256 (asymmetric JWT)
- [ ] OpenBadges / W3C VC format support
- [ ] Third-party API (for external integrations)

### Phase 4: Mobile & Wallet (March 2025)
- [ ] Mobile app (iOS / Android)
- [ ] Credential wallet (aggregate certs from multiple institutions)
- [ ] QR code scanning for instant verification
- [ ] Push notifications (new credential, verification attempt)
- [ ] Credential sharing via URL/NFC

### Phase 5: Enterprise Features (June 2025+)
- [ ] Batch issuance API (CSV upload)
- [ ] Customizable credential templates
- [ ] Brand customization (tenant logos, colors)
- [ ] Advanced fraud analytics (ML-based anomaly detection)
- [ ] SLA & premium support tiers

---

## 🎯 Success Criteria

### Launch Readiness
- [ ] All Phase 1 features implemented & tested
- [ ] Security audit completed (OWASP Top 10)
- [ ] Load testing: 1,000 concurrent users, 100 verifications/sec
- [ ] Documentation: API docs, user guides, admin manual
- [ ] Demo data seeded: 3 institutions, 5 HR companies, 10K sample certs
- [ ] Deployment automated (CI/CD pipeline)
- [ ] Monitoring & alerting configured (uptime, errors, fraud)

### Post-Launch (30-day)
- [ ] 10+ institutions onboarded
- [ ] 50+ HR companies registered
- [ ] 1M+ certificates issued
- [ ] 99.9% uptime SLA met
- [ ] Zero security incidents
- [ ] NPS score ≥ 40

---

## 📞 Support & Feedback

**Questions or feedback on this PRD?**
- Email: product@uivi.io
- Slack: #product-roadmap
- GitHub Issues: Tag `[PRD]`

---

**Approval Sign-Off**:
- [ ] Product Manager: ________________ Date: ________
- [ ] Tech Lead: ________________ Date: ________
- [ ] Security Officer: ________________ Date: ________

---

**Last Updated**: June 2024
