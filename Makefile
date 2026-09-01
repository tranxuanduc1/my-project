.PHONY: up down logs test tidy smoke

up:
	docker compose up --build

down:
	docker compose down

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
