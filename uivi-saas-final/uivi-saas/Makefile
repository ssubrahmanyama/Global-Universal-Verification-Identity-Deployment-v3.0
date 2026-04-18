.PHONY: up down logs test clean

up:
	docker compose up --build -d
	@echo ""
	@echo "✅ UIVI SaaS running → http://localhost:8080"
	@echo ""
	@echo "Demo Accounts (password: Demo@1234)"
	@echo "  Institution : issuer@iit-bombay.uivi.app   | tenant: iit-bombay"
	@echo "  HR          : hr@google-hr.uivi.app         | tenant: google-hr"
	@echo "  Student     : student@iit-bombay.uivi.app  | tenant: iit-bombay"
	@echo "  Regulatory  : regulator@naac-reg.uivi.app  | tenant: naac-reg"
	@echo "  Fraud L1    : analyst@fraud-l1.uivi.app    | tenant: fraud-l1"

down:
	docker compose down -v

logs:
	docker compose logs -f app

test:
	@curl -sf http://localhost:8080/health && echo "✓ API healthy" || echo "✗ API not reachable"

clean:
	docker compose down -v --rmi local 2>/dev/null || true
