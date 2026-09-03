.PHONY: up down logs test tidy smoke start init-db

up:
	docker compose up -d --build

down:
	docker compose down

start:
	docker compose start

init-db:
	docker compose exec -T postgres psql -U postgres -d postgres -f /docker-entrypoint-initdb.d/001-databases.sql

logs:
	docker compose logs -f iam order payment

tidy:
	cd services/iam && go mod tidy
	cd services/order && go mod tidy
	cd services/payment && go mod tidy

test:
	cd services/iam && go test ./...
	cd services/order && go test ./...
	cd services/payment && go test ./...

smoke:
	bash scripts/smoke.sh
