package auth

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type mockPool struct {
	db  *sql.DB
	err error
}

func (p *mockPool) DB(tenantSlug string) (*sql.DB, error) {
	return p.db, p.err
}

func TestRegisterUser_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	masterDB, masterMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer masterDB.Close()

	tenantDB, tenantMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer tenantDB.Close()

	masterMock.ExpectQuery(`SELECT id,name,org_type,status FROM tenants WHERE slug=\$1`).
		WithArgs("iit-bombay").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "org_type", "status"}).AddRow(1, "IIT Bombay", "institution", "active"))

	tenantMock.ExpectExec(`INSERT INTO users`).
		WithArgs(sqlmock.AnyArg(), "newuser@test.uivi.app", "Test User", "user", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := New(masterDB, &mockPool{db: tenantDB}, "secret")

	w := httptest.NewRecorder()
	reqBody := `{"tenant_slug":"iit-bombay","email":"newuser@test.uivi.app","password":"Demo@1234","full_name":"Test User"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register/user", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.RegisterUser(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["role"] != "user" {
		t.Fatalf("expected role user, got %v", resp["role"])
	}
	if resp["tenant"] != "iit-bombay" {
		t.Fatalf("expected tenant iit-bombay, got %v", resp["tenant"])
	}

	if err := masterMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := tenantMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterUser_InvalidTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	masterDB, masterMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer masterDB.Close()

	masterMock.ExpectQuery(`SELECT id,name,org_type,status FROM tenants WHERE slug=\$1`).
		WithArgs("unknown-tenant").
		WillReturnError(sql.ErrNoRows)

	h := New(masterDB, &mockPool{db: nil, err: nil}, "secret")

	w := httptest.NewRecorder()
	reqBody := `{"tenant_slug":"unknown-tenant","email":"newuser@test.uivi.app","password":"Demo@1234","full_name":"Test User"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register/user", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.RegisterUser(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", w.Code, w.Body.String())
	}

	if err := masterMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLogin_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	masterDB, masterMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer masterDB.Close()

	tenantDB, tenantMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer tenantDB.Close()

	masterMock.ExpectQuery(`SELECT id,name,org_type,status FROM tenants WHERE slug=\$1`).
		WithArgs("iit-bombay").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "org_type", "status"}).AddRow(1, "IIT Bombay", "institution", "active"))

	hashed, err := bcrypt.GenerateFromPassword([]byte("Demo@1234"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}

	tenantMock.ExpectQuery(`SELECT id,role,full_name,password_hash FROM users WHERE email=\$1 AND is_active=true`).
		WithArgs("issuer@iit-bombay.uivi.app").
		WillReturnRows(sqlmock.NewRows([]string{"id", "role", "full_name", "password_hash"}).AddRow("u1", "institution", "Prof. Sharma (IIT Bombay)", string(hashed)))

	tenantMock.ExpectExec(`INSERT INTO audit_events\(id,user_id,action,details,created_at\) VALUES\(\$1,\$2,'LOGIN','\{\}',NOW\(\)\)`).
		WithArgs(sqlmock.AnyArg(), "u1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := New(masterDB, &mockPool{db: tenantDB}, "secret")

	w := httptest.NewRecorder()
	reqBody := `{"tenant_slug":"iit-bombay","email":"issuer@iit-bombay.uivi.app","password":"Demo@1234"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.Login(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["user_id"] != "u1" {
		t.Fatalf("expected user_id u1, got %v", resp["user_id"])
	}
	if _, ok := resp["token"]; !ok {
		t.Fatal("expected token in response")
	}

	if err := masterMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := tenantMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
