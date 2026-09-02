.PHONY: up down logs test tidy smoke start

up:
	docker compose up --build

down:
	docker compose down

start:
	docker compose start

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
