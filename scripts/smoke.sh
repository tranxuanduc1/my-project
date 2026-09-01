#!/usr/bin/env bash
set -euo pipefail

IAM_URL=${IAM_URL:-http://localhost:8081}
ORDER_URL=${ORDER_URL:-http://localhost:8082}
PAYMENT_URL=${PAYMENT_URL:-http://localhost:8083}
ADMIN_EMAIL=${IAM_ADMIN_EMAIL:-admin@example.com}
ADMIN_PASSWORD=${IAM_ADMIN_PASSWORD:-admin123456}
CUSTOMER_EMAIL="smoke-$(date +%s)@example.com"
CUSTOMER_PASSWORD=customer123
RUN_ID=$(date +%s)

admin_token=$(curl -fsS -X POST "$IAM_URL/api/v1/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}" | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')
curl -fsS -X POST "$IAM_URL/api/v1/auth/register" -H 'Content-Type: application/json' -d "{\"email\":\"$CUSTOMER_EMAIL\",\"password\":\"$CUSTOMER_PASSWORD\"}" >/dev/null
customer_token=$(curl -fsS -X POST "$IAM_URL/api/v1/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"$CUSTOMER_EMAIL\",\"password\":\"$CUSTOMER_PASSWORD\"}" | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')
product=$(curl -fsS -X POST "$ORDER_URL/api/v1/products" -H "Authorization: Bearer $admin_token" -H 'Content-Type: application/json' -d "{\"sku\":\"SMOKE-$RUN_ID\",\"name\":\"Smoke product\",\"price_cents\":1250,\"currency\":\"USD\",\"stock\":10}")
product_id=$(printf '%s' "$product" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
order=$(curl -fsS -X POST "$ORDER_URL/api/v1/orders" -H "Authorization: Bearer $customer_token" -H "Idempotency-Key: smoke-$(date +%s)" -H 'Content-Type: application/json' -d "{\"items\":[{\"product_id\":\"$product_id\",\"quantity\":2}]}")
order_id=$(printf '%s' "$order" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')

for _ in $(seq 1 20); do
  payment=$(curl -fsS "$PAYMENT_URL/api/v1/payments" -H "Authorization: Bearer $customer_token")
  payment_id=$(printf '%s' "$payment" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
  [ -n "$payment_id" ] && break
  sleep 1
done

curl -fsS -X POST "$PAYMENT_URL/api/v1/payments/$payment_id/succeed" -H "Authorization: Bearer $customer_token" >/dev/null
sleep 2
result=$(curl -fsS "$ORDER_URL/api/v1/orders/$order_id" -H "Authorization: Bearer $customer_token")
printf '%s\n' "$result"
printf '%s' "$result" | grep -q '"status":"confirmed"'
echo "Smoke flow passed"
