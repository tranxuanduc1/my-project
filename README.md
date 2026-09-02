# MyProject commerce MVP

Monorepo gồm ba Go service (`iam`, `order`, `payment`) dùng Gin, GORM,
golang-migrate, PostgreSQL database riêng cho từng service, Redis, RabbitMQ,
Meilisearch và MinIO.
Payment là mock để có thể chạy trọn luồng local mà không cần tài khoản bên thứ ba.

## Kiến trúc source code

Mỗi service là một Go module độc lập và dùng cùng một cấu trúc phân lớp:

```text
services/<service>/
├── cmd/api/                    # composition root, khởi động process
├── internal/
│   ├── domain/                 # entity và quy tắc cốt lõi, không phụ thuộc HTTP
│   ├── application/            # điều phối use case
│   ├── infrastructure/config/  # adapter đọc cấu hình môi trường
│   └── transport/httpauth/     # JWT/RBAC middleware của HTTP transport
└── migrations/                 # schema PostgreSQL
```

Chiều phụ thuộc chính là `cmd -> application -> domain`. Code phụ thuộc Gin,
GORM, RabbitMQ, Redis, MinIO và Meilisearch được giữ ngoài `domain`; nhờ đó entity
không biết process được chạy bằng HTTP hay lưu bằng công nghệ nào. `cmd/api` chỉ
xử lý mode `migrate`, gọi composition root và log lỗi khởi động.

## Chạy local

```bash
cp .env.example .env
docker compose up --build
```

Mỗi service có `Dockerfile.dev`, `.air.toml`, `.env`, `.env.example`, `go.mod`
và `go.sum` riêng. Compose đọc biến runtime qua `env_file`, mount trực tiếp đúng source service và Air tự
build/restart service tương ứng. Các endpoint:

| Thành phần | URL | Thông tin local |
|---|---|---|
| IAM | http://localhost:8081 | admin@example.com / admin123456 |
| Order | http://localhost:8082 | JWT từ IAM |
| Payment | http://localhost:8083 | JWT từ IAM |
| Adminer | http://localhost:8080 | postgres / postgres / iam, orders hoặc payments |
| Meilisearch | http://localhost:7700 | master key trong `.env` |
| MinIO console | http://localhost:9001 | minioadmin / minioadmin |
| RabbitMQ console | http://localhost:15672 | app / app |

Health check lần lượt ở `/health`. Chạy business flow mẫu sau khi stack healthy:

```bash
make smoke
```

## Business flow

1. IAM đăng ký/đăng nhập và cấp JWT chứa roles.
2. Admin tạo sản phẩm; tìm kiếm dùng Meilisearch và fallback PostgreSQL.
3. Customer tạo order với `Idempotency-Key`. Order khóa tồn kho, tính tiền phía
   server và ghi `order.created` vào transactional outbox.
4. Payment nhận event qua RabbitMQ và tạo payment `pending`.
5. Customer gọi `/api/v1/payments/:id/succeed` hoặc `/fail`. Payment đối soát
   order qua internal HTTP rồi phát event kết quả.
6. Order nhận event idempotently, xác nhận đơn hoặc hoàn tồn kho khi thất bại.

## API chính

- IAM: `POST /api/v1/auth/register`, `POST /api/v1/auth/login`,
  `GET /api/v1/users/me`; admin quản lý `/api/v1/users` và `/api/v1/roles`.
- Order: product CRUD, `POST /api/v1/products/:id/image/presign`,
  `POST/GET /api/v1/orders`, `GET /api/v1/orders/:id`, cancel order.
- Payment: list/get payment và mock `POST /api/v1/payments/:id/succeed|fail`.

Tiền được truyền dưới dạng số nguyên cents (`price_cents`, `amount_cents`) để
không phát sinh sai số dấu phẩy động. GORM không chạy AutoMigrate; toàn bộ DDL
nằm trong `services/*/migrations`.

## Lệnh hữu ích

```bash
make logs
make test
make tidy
docker compose down
docker compose down -v  # xóa toàn bộ dữ liệu local
```

## Chạy từng service độc lập

Không có `go.work` và không service nào import package của service khác. Sau khi
các dependency bên ngoài cần thiết đã sẵn sàng, có thể build/test/chạy từ chính
thư mục service:

```bash
cd services/iam
set -a
. ./.env.example
set +a
go test ./...
go run ./cmd/api
```

Thực hiện tương tự với `services/order` và `services/payment`. JWT secret,
RabbitMQ URL và internal API key là contract cấu hình giữa các process, không
phải shared source hay shared Go module. Payment không phụ thuộc startup của
Order; nếu Order tạm dừng thì bước đối soát HTTP trả lỗi tạm thời và có thể thử
lại sau. File `.env` chứa hostname trong Docker network; `.env.example` dùng
`localhost` để chạy service trực tiếp trên host.
