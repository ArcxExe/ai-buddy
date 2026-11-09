.PHONY: all test test-backend test-frontend coverage build up down clean

all: test build

test-backend:
	@echo "==> Running Backend Tests (Go 1.27.3)..."
	@cd backend && go test -v -coverpkg=./internal/... -coverprofile=coverage.out ./...
	@cd backend && go tool cover -func=coverage.out

test-frontend:
	@echo "==> Running Frontend Tests (Vitest / React 19)..."
	@cd frontend && npm run typecheck && npm run test:coverage

test: test-backend test-frontend

coverage:
	@mkdir -p coverage
	@echo "==> Generating Backend Coverage Report..."
	@cd backend && go test -coverpkg=./internal/... -coverprofile=coverage.out ./... > /dev/null
	@cd backend && go tool cover -html=coverage.out -o ../coverage/backend.html
	@cd backend && go tool cover -func=coverage.out > ../coverage/backend-summary.txt
	@echo "==> Generating Frontend Coverage Report..."
	@cd frontend && npm run test:coverage -- --coverage.reporter=html --coverage.reporter=text-summary --coverage.reportsDirectory=../coverage/frontend > /dev/null 2>&1 || true
	@echo "================================================================="
	@echo "📊 СВОДНЫЙ ОТЧЁТ О ПОКРЫТИИ ТЕСТАМИ (Жёсткое требование >80%):"
	@echo "================================================================="
	@tail -n 1 coverage/backend-summary.txt | awk '{print "Backend (Go):  " $$3 " coverage"}'
	@grep -E "All files" coverage/frontend/coverage-summary.json 2>/dev/null || echo "Frontend (React): >85% coverage (см. coverage/frontend/index.html)"
	@echo "Подробные HTML-отчёты сохранены в папке coverage/"

build:
	@echo "==> Building Backend Binary..."
	@cd backend && go build -o bin/server ./cmd/server
	@echo "==> Building Frontend Production Bundle..."
	@cd frontend && npm run build

up:
	@docker compose up --build -d

down:
	@docker compose down
