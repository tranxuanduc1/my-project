# Kế hoạch triển khai Observability

Kiến trúc mục tiêu:

```text
Go (`slog` + OpenTelemetry SDK)
  -> OpenTelemetry Collector
      -> Prometheus (metrics)
      -> Loki (logs)
      -> Tempo (traces)
  -> Grafana
```

Project hiện có ba Go service độc lập:

- `services/iam`
- `services/order`
- `services/payment`

Các service chạy cùng PostgreSQL, Redis, RabbitMQ, Meilisearch và MinIO bằng Docker Compose. Thực hiện tuần tự tám bước bên dưới. Mỗi bước được viết để có thể dán riêng vào một cuộc trò chuyện mới.

## Nguyên tắc chung cho cả tám bước

- Trước khi sửa, đọc source thực tế và kiểm tra `git status`; không ghi đè thay đổi không liên quan.
- Giữ mỗi service có thể build và chạy độc lập. Chưa tạo shared Go module chỉ để chia sẻ một package observability nhỏ.
- Cấu hình runtime phải đi qua environment variable; không hard-code endpoint hoặc credential production.
- Dùng structured logging với `log/slog`; không log password, JWT, API key, cookie, authorization header hoặc payload nhạy cảm.
- Không dùng `user_id`, `order_id`, `payment_id`, `event_id`, URL thật hoặc error message làm Prometheus label.
- Tên HTTP span và metric phải dùng route template, ví dụ `POST /orders/:id`, không dùng path chứa ID thật.
- Telemetry không được làm request hoặc worker thất bại chỉ vì Collector tạm thời không hoạt động.
- Thêm comment tiếng Anh ngắn gọn chỉ cho intent hoặc invariant không hiển nhiên.
- Chỉ làm đúng bước đang được yêu cầu; không triển khai trước các bước tiếp theo.
- Sau mỗi bước, chạy kiểm tra phù hợp và báo rõ phần nào đã được kiểm chứng bằng source, test, Compose hoặc runtime.

---

## Bước 1 — Dựng Collector, Prometheus, Loki, Tempo và Grafana

### Prompt để dán vào cuộc trò chuyện mới

> Hãy thực hiện Bước 1 trong `plan.md`: dựng observability stack local gồm OpenTelemetry Collector, Prometheus, Loki, Tempo và Grafana trong Docker Compose. Hãy đọc repo và kiểm tra worktree trước khi sửa. Chỉ làm Bước 1, chưa instrument Go service.

### Mục tiêu

Dựng được backend observability hoàn chỉnh trước khi sửa application. Sau bước này các container phải khởi động được, Grafana có sẵn ba datasource và Collector sẵn sàng nhận OTLP từ các service ở những bước sau.

### Phạm vi thực hiện

1. Tạo cấu trúc cấu hình dưới `deploy/observability/`, dự kiến gồm:

   ```text
   deploy/observability/
     otel-collector.yaml
     prometheus.yaml
     loki.yaml
     tempo.yaml
     grafana/
       provisioning/
         datasources/
   ```

2. Bổ sung vào `docker-compose.yml` các service:

   - `otel-collector`
   - `prometheus`
   - `loki`
   - `tempo`
   - `grafana`

3. Cấu hình Collector:

   - Nhận OTLP qua gRPC và HTTP trong Docker network.
   - Có `memory_limiter` và `batch` processor.
   - Metrics được expose để Prometheus scrape.
   - Logs được gửi tới Loki qua giao thức OTLP được phiên bản Loki đang dùng hỗ trợ.
   - Traces được gửi tới Tempo qua OTLP.
   - Bật health extension để Compose healthcheck được Collector.

4. Cấu hình Prometheus scrape ít nhất:

   - Chính Prometheus.
   - Metrics exporter của Collector.

5. Provision Grafana datasource tự động:

   - Prometheus
   - Loki
   - Tempo

6. Cấu hình liên kết datasource nếu phiên bản hỗ trợ:

   - Loki derived field `trace_id` mở trace trong Tempo.
   - Tempo có thể tìm log liên quan trong Loki.

7. Pin image version cụ thể thay vì dùng `latest`, trừ khi repo đã có quy ước khác. Chỉ expose ra host các port hữu ích cho local development; các endpoint nội bộ còn lại dùng Docker network.

8. Cập nhật `.env.example` hoặc README nếu cấu hình Compose cần biến môi trường mới. Không đưa secret thật vào Git.

### Tiêu chí nghiệm thu

- `docker compose config` hợp lệ.
- Năm service observability có cấu hình healthcheck hoặc cách kiểm tra readiness phù hợp.
- Grafana mở được và thấy ba datasource đã provision.
- Prometheus targets cho Prometheus và Collector ở trạng thái healthy.
- Collector sẵn sàng nhận OTLP dù chưa có application gửi telemetry.
- Không thay đổi Go source trong bước này.

### Kiểm tra dự kiến

```bash
docker compose config
docker compose up -d otel-collector prometheus loki tempo grafana
docker compose ps
```

Sau đó kiểm tra health endpoint, Prometheus targets và Grafana datasources bằng endpoint hoặc UI tương ứng.

---

## Bước 2 — Khởi tạo OTel SDK và `slog` cho service `order`

### Prompt để dán vào cuộc trò chuyện mới

> Hãy thực hiện Bước 2 trong `plan.md`: thêm nền tảng OpenTelemetry SDK và structured `slog` cho riêng service `order`. Bước 1 đã được hoàn thành. Hãy đọc trạng thái source hiện tại trước khi sửa. Chỉ làm bootstrap/provider và lifecycle; chưa instrument HTTP, database, Redis, RabbitMQ, Meilisearch hoặc MinIO.

### Mục tiêu

`order` có một composition root tạo logger, tracer provider và meter provider; gửi OTLP tới Collector; shutdown/flush đúng khi process dừng. Collector unavailable không được ngăn service khởi động hoặc phục vụ request.

### Phạm vi thực hiện

1. Tạo package:

   ```text
   services/order/internal/infrastructure/observability/
   ```

2. Package cung cấp một API nhỏ để:

   - Đọc observability config từ environment.
   - Tạo `*slog.Logger` JSON.
   - Tạo OpenTelemetry `TracerProvider`.
   - Tạo OpenTelemetry `MeterProvider`.
   - Cấu hình global propagator gồm W3C Trace Context và Baggage.
   - Trả về hàm shutdown idempotent có timeout để flush providers.

3. Resource attributes tối thiểu:

   - `service.name=order`
   - `service.version`
   - `deployment.environment.name`
   - `service.instance.id` nếu có nguồn giá trị ổn định phù hợp.

4. Environment variables dự kiến:

   ```text
   OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
   OTEL_SERVICE_VERSION=dev
   OTEL_DEPLOYMENT_ENVIRONMENT=local
   OTEL_TRACES_SAMPLER=parentbased_traceidratio
   OTEL_TRACES_SAMPLER_ARG=1.0
   LOG_LEVEL=info
   ```

   Tên biến nên ưu tiên convention chính thức của OTel. Bổ sung vào file example phù hợp, không ghi secret.

5. Tích hợp vào `services/order/internal/bootstrap/bootstrap.go` và/hoặc `cmd/api/main.go`:

   - Dùng signal-aware root context thay cho các `context.Background()` sống vô hạn.
   - Logger là dependency được truyền rõ ràng, hoặc được thiết lập làm `slog.Default()` tại composition root nếu giúp migration gọn hơn.
   - Log startup, shutdown và fatal error bằng structured attributes.
   - Shutdown HTTP server và telemetry providers có timeout.

6. Không thêm business metric hoặc manual span ở bước này. Chỉ cần chứng minh một startup/shutdown log và SDK initialization hoạt động.

### Quyết định logging

Ưu tiên một trong hai cách, sau khi xác nhận support của dependency version:

- `slog` bridge gửi logs qua OTel Log SDK tới Collector; hoặc
- `slog` JSON stdout với handler bổ sung trace/span IDs, nếu direct OTel logs chưa phù hợp.

Nếu chọn stdout, phải giải thích Collector sẽ thu stdout bằng cơ chế nào trước khi tuyên bố log đã đi vào Loki. Với local Docker Compose, direct OTLP logs thường tạo luồng rõ ràng hơn, nhưng không được hy sinh stdout log dùng cho vận hành container.

### Tiêu chí nghiệm thu

- `order` vẫn migrate, build và chạy được.
- Startup và shutdown có structured log.
- Collector nhận được telemetry resource mang `service.name=order`.
- Process nhận SIGTERM có thể dừng HTTP server, worker context và flush telemetry.
- Không có instrumentation dependency cụ thể ngoài nền tảng provider/lifecycle.

### Kiểm tra dự kiến

```bash
cd services/order
go test ./...
go vet ./...
```

Ngoài ra chạy Compose và xác nhận Collector không có lỗi export liên tục. Thử dừng Collector và kiểm tra `order` vẫn phục vụ request.

---

## Bước 3 — Instrument HTTP inbound/outbound và liên kết log với trace

### Prompt để dán vào cuộc trò chuyện mới

> Hãy thực hiện Bước 3 trong `plan.md`: instrument HTTP cho service `order` và thêm trace correlation vào `slog`. Bước 1 và 2 đã hoàn thành. Chỉ làm HTTP inbound/outbound, access log, recovery và log-trace correlation; chưa instrument database/cache/broker.

### Mục tiêu

Mỗi HTTP request vào `order` có server span, structured access log có `trace_id`/`span_id`, và các HTTP request ra ngoài có client span đồng thời tự động propagate W3C trace context.

### Phạm vi thực hiện

1. Thay `gin.Default()` bằng `gin.New()` để kiểm soát logging:

   - OTel Gin middleware.
   - `slog` access-log middleware.
   - Recovery middleware ghi structured error và vẫn trả response đúng.

2. Access log tối thiểu có:

   - `service`
   - `trace_id`
   - `span_id`
   - HTTP method
   - route template
   - status code
   - duration
   - response size nếu dễ lấy và có ích

3. Không log health endpoint ở mức `INFO` nếu nó tạo quá nhiều noise; có thể bỏ qua hoặc hạ level.

4. Instrument outbound `http.Client` bằng OTel transport:

   - Client gọi Meilisearch.
   - Các HTTP client khác của `order` nếu source thực tế có.
   - Giữ nguyên timeout hiện tại.

5. Bảo đảm `context.Context` từ Gin/request được truyền xuống application và adapters. Không dùng `context.Background()` giữa request flow.

6. Span naming:

   - Inbound dùng route template.
   - Outbound dùng HTTP semantic conventions.
   - Không ghi request/response body mặc định.

7. Error handling:

   - Server span nhận error/status phù hợp.
   - Không đánh dấu mọi HTTP 4xx là internal server error một cách máy móc.
   - Panic được recovery, log có trace ID và span ghi nhận lỗi.

### Tiêu chí nghiệm thu

- Gọi một endpoint `order` tạo trace hiển thị trong Tempo.
- Access log tương ứng trong Loki có cùng `trace_id`.
- Từ log có thể mở trace, hoặc ít nhất query chính xác theo `trace_id` nếu liên kết UI chưa được cấu hình.
- Tên span không chứa UUID hoặc path có cardinality cao.
- Healthcheck không làm log/dashboard bị nhiễu đáng kể.

### Kiểm tra dự kiến

```bash
cd services/order
go test ./...
go vet ./...
```

Chạy stack, gọi `/health` và ít nhất một API business; kiểm tra log Loki và trace Tempo bằng cùng `trace_id`.

---

## Bước 4 — Instrument PostgreSQL, Redis, Meilisearch, MinIO và metrics của `order`

### Prompt để dán vào cuộc trò chuyện mới

> Hãy thực hiện Bước 4 trong `plan.md`: instrument dependencies và thêm metrics cho service `order`. Bước 1-3 đã hoàn thành. Tập trung PostgreSQL/GORM, Redis, Meilisearch, MinIO, Go runtime và HTTP metrics. Chưa thay đổi RabbitMQ/outbox propagation.

### Mục tiêu

Trace của `order` thể hiện các dependency calls quan trọng, và Prometheus có RED metrics cùng những metric vận hành cần thiết mà không tạo cardinality cao.

### Phạm vi thực hiện

1. PostgreSQL/GORM:

   - Dùng instrumentation tương thích với GORM/driver hiện tại.
   - Tạo spans theo database semantic conventions.
   - Thu connection-pool metrics từ underlying `sql.DB` nếu instrumentation không tự cung cấp.
   - Không export bind parameter, password hoặc raw dữ liệu nhạy cảm.

2. Redis:

   - Instrument command tracing và duration/error metrics bằng integration tương thích với `go-redis`.
   - Thêm cache hit/miss metric tại product-cache adapter hoặc boundary phù hợp.

3. Meilisearch:

   - Dùng OTel-instrumented HTTP transport.
   - Giữ timeout.
   - Span/metric không chứa search query hoặc API key mặc định.

4. MinIO:

   - Instrument HTTP transport nếu MinIO client cho phép cấu hình transport an toàn.
   - Không log presigned URL đầy đủ hoặc credential.

5. Metrics nền tảng:

   - Go runtime/process metrics qua OTel runtime instrumentation phù hợp.
   - HTTP request count, duration và in-flight requests nếu middleware chưa cung cấp đủ.
   - Dependency error/duration metrics chỉ bổ sung khi auto-instrumentation chưa có dữ liệu cần thiết.

6. Custom metric names phải theo OTel semantic conventions khi có convention tương ứng. Unit phải rõ ràng, ví dụ seconds hoặc bytes.

7. Attribute/label được phép nên giới hạn ở tập nhỏ như:

   - `service.name`
   - HTTP route template/method/status
   - dependency/system name
   - operation
   - result dạng hữu hạn như `success|error`

### Tiêu chí nghiệm thu

- Một request business có child spans cho database và các dependency thực sự được gọi.
- Prometheus thấy HTTP request rate/error/duration và Go runtime metrics.
- Cache hit/miss quan sát được.
- Không có UUID, search term, SQL parameter hoặc error string làm metric label.
- App vẫn chạy nếu Collector tắt.

### Kiểm tra dự kiến

```bash
cd services/order
go test ./...
go vet ./...
```

Chạy một create/read/search flow và kiểm tra trace structure, Prometheus label set và log không chứa secret.

---

## Bước 5 — Propagate trace qua transactional outbox và RabbitMQ

### Prompt để dán vào cuộc trò chuyện mới

> Hãy thực hiện Bước 5 trong `plan.md`: thêm tracing, metrics và context propagation cho transactional outbox/RabbitMQ giữa `order` và `payment`. Bước 1-4 đã hoàn thành cho `order`. Đây là bước reliability-sensitive: hãy đọc migration, transaction tạo outbox, publisher, consumer và ACK/NACK order trước khi sửa. Không thay đổi business semantics hoặc retry policy ngoài phần telemetry cần thiết.

### Mục tiêu

Trace context không bị mất tại ranh giới outbox. Có thể theo dõi flow từ HTTP request tạo order qua outbox publisher, RabbitMQ, payment consumer, payment outbox, RabbitMQ và order consumer.

### Vấn đề cần giải quyết

Publisher hiện chạy nền bằng context riêng. Nếu chỉ inject context tại lúc publish thì nó không còn context của request đã tạo outbox record. Vì vậy trace propagation metadata phải được lưu nguyên tử cùng outbox record trong transaction business.

### Phạm vi thực hiện

1. Khảo sát chính xác schema và mọi nơi tạo outbox record ở cả `order` và `payment`.

2. Thêm migration forward/backward cho propagation metadata, có thể là:

   - Cột JSON headers/carrier; hoặc
   - Các cột `traceparent`, `tracestate`, `baggage`.

   Ưu tiên carrier nhỏ, version-neutral và không chứa business payload nhạy cảm.

3. Khi tạo outbox record:

   - Inject W3C propagation headers từ request/consumer context hiện tại.
   - Lưu cùng transaction với business mutation và outbox event.

4. Outbox publisher:

   - Extract context từ outbox record.
   - Tạo producer span theo messaging semantic conventions.
   - Inject propagation headers vào `amqp.Publishing.Headers`.
   - Ghi nhận publish success/failure.
   - Không đánh dấu published nếu RabbitMQ publish thất bại.
   - Không làm thay đổi ACK/confirm semantics hiện tại ngoài khi một bug rõ ràng được phát hiện và được báo riêng.

5. Consumer:

   - Extract context từ AMQP headers.
   - Tạo consumer/process span.
   - Truyền context đó vào application handler và database calls.
   - Ghi nhận JSON decode failure, handler failure, ACK, NACK và requeue.

6. Chọn quan hệ trace rõ ràng:

   - MVP có thể tiếp tục parent context qua message để UI dễ theo dõi.

7. Custom metrics tối thiểu:

   - `outbox_pending` hoặc observable gauge tương đương.
   - `outbox_oldest_event_age`.
   - Publish attempts theo result.
   - Consumer processed theo event type và result.
   - Consumer processing duration.
   - Reconnect count.
   - ACK/NACK/requeue count nếu không bị trùng metric.

8. Label chỉ dùng giá trị hữu hạn như service, event type đã kiểm soát, operation và result. Event ID/order ID chỉ nằm trong span/log attributes.

9. Lifecycle:

   - Worker phải dùng root cancellable context từ bootstrap.
   - Reconnect loop và sleep phải thoát khi context cancel.
   - Shutdown không để goroutine tiếp tục vô hạn.

### Tiêu chí nghiệm thu

- Migration up/down hợp lệ cho cả database bị ảnh hưởng.
- Trace context được lưu cùng outbox event, xuất hiện trong AMQP headers và được consumer extract.
- Một order flow cho thấy rõ producer/consumer spans ở `order` và `payment`.
- Logs ở hai service query được bằng trace ID/event ID.
- ACK/NACK, outbox marking và idempotency semantics không bị phá vỡ.
- Outbox backlog/age và consumer result có metrics trong Prometheus.

### Kiểm tra dự kiến

```bash
cd services/order && go test ./...
cd services/payment && go test ./...
```

Ngoài unit/integration tests, chạy end-to-end order flow và mô phỏng ít nhất một publish/consumer failure có kiểm soát để xác nhận metric/log/span phản ánh đúng mà không mất event.

---

## Bước 6 — Áp dụng pattern hoàn chỉnh sang `payment` và `iam`

### Prompt để dán vào cuộc trò chuyện mới

> Hãy thực hiện Bước 6 trong `plan.md`: áp dụng observability pattern đã hoàn thành ở `order` sang `payment`, sau đó `iam`. Hãy dùng implementation hiện tại của `order` làm pattern nhưng vẫn đọc source và dependency thực tế của từng service. Không tạo shared module mới trừ khi có bằng chứng duplication đang gây vấn đề thực sự.

### Mục tiêu

Cả ba service có logging, metrics, tracing, lifecycle và resource attributes nhất quán; đồng thời vẫn giữ instrumentation phù hợp với dependency riêng của từng service.

### Phạm vi thực hiện

1. `payment`:

   - Observability provider và structured `slog`.
   - Graceful shutdown/root context.
   - Gin inbound instrumentation và access logging.
   - Outbound HTTP client `payment -> order` dùng OTel transport và propagate trace context.
   - GORM/PostgreSQL instrumentation.
   - RabbitMQ consumer/outbox instrumentation theo Bước 5.
   - Resource `service.name=payment`.

2. `iam`:

   - Observability provider và structured `slog`.
   - Graceful shutdown/root context.
   - Gin inbound instrumentation và access logging.
   - GORM/PostgreSQL instrumentation.
   - Resource `service.name=iam`.
   - Không thêm RabbitMQ/Redis instrumentation vì `iam` hiện không sở hữu các dependency đó.

3. Cấu hình:

   - Thêm environment variables tương ứng vào example/Compose env của từng service.
   - Mỗi service phải có service name riêng; version/environment có convention chung.
   - Sampling có thể cấu hình độc lập nhưng default local nhất quán.

4. Consistency:

   - Cùng field names cho log: `trace_id`, `span_id`, `service`, `error`, `duration`.
   - Cùng metric naming/unit conventions.
   - Cùng behavior khi Collector unavailable.
   - Không copy instrumentation không liên quan chỉ để ba package giống hệt nhau.

5. Business metrics có thể bổ sung với số lượng nhỏ:

   - IAM authentication attempts theo result/reason hữu hạn.
   - Order created/status transition theo status hữu hạn.
   - Payment created/status transition theo status hữu hạn.

   Không dùng email, user ID, order ID hoặc payment ID làm label.

### Tiêu chí nghiệm thu

- Cả ba module `go test ./...` và `go vet ./...` thành công.
- Grafana/Tempo phân biệt đúng `iam`, `order`, `payment` bằng `service.name`.
- Request từ client và `payment -> order` giữ trace context.
- RabbitMQ flow giữ context như Bước 5.
- Tắt Collector không làm ba API ngừng phục vụ.
- SIGTERM làm HTTP server, worker và telemetry provider shutdown có kiểm soát.

### Kiểm tra dự kiến

```bash
cd services/iam && go test ./... && go vet ./...
cd services/order && go test ./... && go vet ./...
cd services/payment && go test ./... && go vet ./...
docker compose config
```

---

## Bước 7 — Provision dashboard, alert và viết tài liệu vận hành

### Prompt để dán vào cuộc trò chuyện mới

> Hãy thực hiện Bước 7 trong `plan.md`: tạo Grafana dashboards, Prometheus alert rules và tài liệu vận hành dựa trên telemetry thực tế đang được ba service export. Bước 1-6 đã hoàn thành. Trước khi viết query, hãy kiểm tra tên metric/label thật trong Prometheus; không giả định tên metric từ kế hoạch.

### Mục tiêu

Biến telemetry thô thành màn hình và tín hiệu vận hành dùng được. Dashboard/query phải dựa trên metric thực tế, có datasource provision bằng file và có hướng dẫn điều tra sự cố.

### Phạm vi thực hiện

1. Provision dashboard bằng file dưới `deploy/observability/grafana/` thay vì tạo thủ công trong UI.

2. Dashboard tổng quan RED:

   - Request rate theo service.
   - Error rate.
   - P50/P95/P99 latency.
   - In-flight requests nếu có.
   - Breakdown theo route template với cardinality được kiểm soát.

3. Dashboard dependency:

   - PostgreSQL duration/error/connection pool.
   - Redis duration/error/cache hit ratio.
   - Meilisearch/MinIO/outbound HTTP latency và error.

4. Dashboard async processing:

   - Outbox pending count.
   - Oldest event age.
   - Publish success/failure.
   - Consumer throughput/duration/error.
   - NACK/requeue/reconnect.
   - Order/payment status metrics nếu đã có.

5. Dashboard phải có variables hữu hạn như environment và service. Không tạo variable cho user/order/event ID.

6. Liên kết observability:

   - Log panel có thể lọc theo service/level/trace ID.
   - Loki `trace_id` dẫn tới Tempo.
   - Tempo có link sang logs của cùng service/time range.
   - Metrics-to-trace exemplar nếu pipeline hiện tại hỗ trợ và có dữ liệu thật.

7. Alert rules ban đầu:

   - HTTP 5xx ratio cao trong một cửa sổ đủ dài.
   - P95 latency vượt ngưỡng.
   - Outbox pending tăng hoặc oldest event age quá cao.
   - Consumer error/requeue tăng.
   - PostgreSQL connection pool gần cạn nếu metric có sẵn.
   - Collector/export failures nếu có self-observability metric phù hợp.

8. Tránh alert theo traffic tuyệt đối khi local/dev không có tải đều. Rule phải có `for` duration để giảm flapping.

9. Cập nhật README/runbook:

   - Cách khởi động stack.
   - URL local của Grafana/Prometheus và các endpoint cần thiết.
   - Cách tìm trace từ `trace_id`.
   - Cách chuyển từ trace sang logs.
   - Cách điều tra outbox backlog/consumer failure.
   - Environment variables và sampling.
   - Quy tắc không ghi dữ liệu nhạy cảm/cardinality cao.

### Tiêu chí nghiệm thu

- Dashboard được Grafana load tự động sau khi recreate container/volume phù hợp.
- Các panel chính có dữ liệu từ cả ba service khi có traffic.
- PromQL/LogQL không tham chiếu metric hoặc label không tồn tại.
- Alert rule được Prometheus load thành công.
- Có runbook ngắn cho HTTP failure và async/outbox failure.

### Kiểm tra dự kiến

```bash
docker compose config
docker compose up -d prometheus loki tempo grafana otel-collector
docker compose ps
```

Kiểm tra Prometheus rules/targets, Grafana provisioning logs và từng dashboard bằng traffic thật.

---

## Bước 8 — Smoke test end-to-end và xác nhận failure modes

### Prompt để dán vào cuộc trò chuyện mới

> Hãy thực hiện Bước 8 trong `plan.md`: hoàn thiện script/test và chạy xác minh observability end-to-end cho toàn hệ thống. Bước 1-7 đã hoàn thành. Không chỉ kiểm tra container healthy; hãy tạo traffic business thực, truy vết qua HTTP/database/outbox/RabbitMQ và kiểm tra một số failure mode có thể phục hồi an toàn.

### Mục tiêu

Chứng minh hệ thống quan sát được một business flow hoàn chỉnh và telemetry vẫn có ý nghĩa khi dependency hoặc Collector gặp sự cố.

### Happy-path scenario

1. Khởi động toàn bộ Compose stack.
2. Đăng nhập hoặc lấy token qua `iam` theo flow thực tế.
3. Tạo dữ liệu product/order cần thiết.
4. Tạo order.
5. Chờ outbox publisher gửi `order.created`.
6. Xác nhận `payment` consumer tạo payment.
7. Kích hoạt flow cập nhật payment phù hợp với API hiện có.
8. Chờ payment event quay lại `order` consumer.
9. Xác nhận trạng thái cuối trong API/database.

### Telemetry cần chứng minh

- `iam`, `order`, `payment` xuất hiện đúng dưới ba `service.name`.
- HTTP server spans có route template.
- `payment -> order` outbound request có client/server trace propagation.
- Database/dependency spans xuất hiện đúng nơi dependency được gọi.
- Outbox producer và RabbitMQ consumer spans liên kết được với business flow.
- Structured logs có `trace_id`/`span_id` và query được trong Loki.
- Prometheus có request, error, latency, runtime, outbox và consumer metrics.
- Grafana dashboard hiển thị traffic vừa tạo.
- Từ log có thể mở trace và từ trace tìm được log liên quan.

### Failure scenarios an toàn

Thực hiện từng scenario độc lập và khôi phục dependency sau khi kiểm tra:

1. **Collector unavailable**
   - Dừng Collector tạm thời.
   - API vẫn phục vụ request; không panic hoặc block lâu bất thường.
   - Khởi động lại Collector và kiểm tra telemetry mới tiếp tục được export.

2. **RabbitMQ unavailable**
   - Tạo business record/outbox event trong điều kiện phù hợp.
   - Event vẫn nằm pending, không bị đánh dấu published sai.
   - Sau khi RabbitMQ trở lại, publisher xử lý lại và backlog giảm.
   - Metrics/logs phản ánh reconnect/publish failure/backlog age.

3. **Consumer handler failure hoặc invalid message**
   - Chỉ dùng cách test không phá dữ liệu thật.
   - Xác nhận ACK/NACK/requeue/DLQ behavior đúng với implementation hiện tại.
   - Trace/log/metric phải phản ánh outcome thật, không báo success giả.

4. **Graceful shutdown**
   - Gửi SIGTERM qua Compose stop.
   - HTTP server ngừng nhận request mới.
   - Workers thoát theo context.
   - Telemetry được flush trong timeout.

### Tự động hóa

- Mở rộng `scripts/smoke.sh` hoặc tạo script observability smoke riêng nếu giúp chạy lặp lại.
- Script phải fail fast, in rõ bước bị lỗi và không phụ thuộc click thủ công cho phần API/data validation.
- Không hard-code secret thật; dùng environment variables/local defaults đã có.
- Nếu Grafana/Tempo/Loki API được dùng để xác minh, query theo time range và trace ID cụ thể để tránh false positive từ dữ liệu cũ.

### Tiêu chí nghiệm thu cuối cùng

- Một lệnh hoặc một chuỗi lệnh được tài liệu hóa có thể dựng stack và chạy smoke test.
- Có bằng chứng runtime cho happy path xuyên ba service.
- Có bằng chứng rằng Collector outage không kéo application xuống.
- Có bằng chứng outbox không mất event khi RabbitMQ tạm unavailable.
- Dashboard, alerts, trace-log linking và Prometheus metrics dùng dữ liệu thật.
- Báo cáo cuối phân biệt rõ:
  - kiểm tra source/test;
  - kiểm tra Compose/config;
  - kiểm tra runtime end-to-end;
  - phần nào chưa thể xác minh và lý do.

### Lệnh kiểm tra tổng quát

```bash
docker compose config
docker compose up -d --build
docker compose ps

cd services/iam && go test ./...
cd services/order && go test ./...
cd services/payment && go test ./...

./scripts/smoke.sh
```

Nếu đã tạo script observability riêng, chạy thêm script đó và ghi lại các trace ID/event ID dùng làm bằng chứng.
