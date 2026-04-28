# Hướng Dẫn Chạy Và Test ERG Dành Cho Người Không Chuyên Code

Tài liệu này dành cho người muốn:

- chạy project lên để xem có hoạt động không
- kiểm tra nhanh hệ thống có "sống" hay không
- chạy test để biết code có lỗi không
- làm các bước cơ bản mà không cần hiểu sâu về lập trình

Nếu bạn chỉ cần một cách đơn giản nhất, hãy dùng phần `Cách 1: Chạy bằng Docker`.

---

## 1. ERG là gì, chạy cái gì?

Hiện tại repo này có nhiều tài liệu cũ nói về nhiều service nhỏ như `bot-service`, `crawler-service`, `trending-service`...

Nhưng ở trạng thái code hiện tại, điểm chạy chính bạn nên dùng là:

```bash
go run ./cmd/server
```

Hoặc build thành file chạy:

```bash
go build -o erg-server ./cmd/server
./erg-server
```

Nói đơn giản:

- `MongoDB` là nơi lưu dữ liệu
- `Redis` là nơi cache và queue hoạt động
- `cmd/server` là chương trình chính của app

Nếu thiếu MongoDB hoặc Redis, app thường sẽ không khởi động được.

---

## 2. Bạn cần chuẩn bị gì trước?

### Tối thiểu

Bạn cần có:

- `Go` đã cài
- `Docker Desktop` hoặc `Docker Engine`
- Internet để tải dependency lần đầu

### Kiểm tra nhanh

Mở Terminal trong thư mục `erg-go`, rồi chạy:

```bash
go version
docker --version
docker compose version
```

Nếu cả 3 lệnh đều hiện ra phiên bản, bạn có thể đi tiếp.

---

## 3. Cách dễ nhất: chạy bằng Docker

Đây là cách phù hợp nhất nếu bạn không muốn tự cài MongoDB và Redis bằng tay.

### Bước 1: đi vào thư mục project

```bash
cd /Users/vuong/ERG.Workspace/erg-go
```

### Bước 2: bật hạ tầng

Repo đang có file `docker-compose.yml`.

Chạy:

```bash
docker compose up -d mongodb redis
```

Lệnh này sẽ bật:

- MongoDB ở cổng `27017`
- Redis ở cổng `6379`

### Bước 3: kiểm tra container đã chạy chưa

```bash
docker compose ps
```

Bạn nên thấy ít nhất:

- `erg-mongodb`
- `erg-redis`

Nếu thấy trạng thái gần giống `running` hoặc `healthy` là ổn.

### Bước 4: chạy app chính

Vẫn trong thư mục `erg-go`, chạy:

```bash
go run ./cmd/server
```

Nếu mọi thứ ổn, app sẽ khởi động và mở cổng HTTP.

Thông thường app chạy ở:

```text
http://localhost:8080
```

### Bước 5: kiểm tra app có chạy thật không

Mở terminal mới và chạy:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/metrics
```

Nếu `healthz` trả về JSON hoặc text báo `ok`, nghĩa là app đã lên.

---

## 4. Cách chạy local hoàn toàn bằng terminal

Cách này dùng khi bạn muốn:

- nhìn log trực tiếp
- sửa code rồi chạy lại ngay
- test nhanh trong lúc phát triển

### Bước 1: bật MongoDB và Redis

Nếu bạn chưa cài thủ công, vẫn có thể dùng Docker chỉ để bật 2 dịch vụ nền:

```bash
docker compose up -d mongodb redis
```

### Bước 2: chạy app

```bash
cd /Users/vuong/ERG.Workspace/erg-go
go run ./cmd/server
```

### Bước 3: dừng app

Trong cửa sổ terminal đang chạy app, nhấn:

```text
Ctrl + C
```

### Bước 4: tắt MongoDB và Redis nếu không dùng nữa

```bash
docker compose down
```

---

## 5. Cách build thành file chạy

Nếu bạn không muốn dùng `go run` mỗi lần, có thể build thành file.

### Build

```bash
cd /Users/vuong/ERG.Workspace/erg-go
go build -o erg-server ./cmd/server
```

### Chạy

```bash
./erg-server
```

### Ý nghĩa

- `go run` = chạy luôn từ source code
- `go build` = tạo ra file chương trình thật

Nếu bạn chỉ kiểm tra nhanh, `go run` là đủ.

---

## 6. Cách test project

Trong ngôn ngữ dễ hiểu:

- "chạy app" là để xem chương trình có mở lên được không
- "chạy test" là để xem code có đang tự kiểm tra và báo lỗi logic hay không

### Cách test an toàn nhất

```bash
cd /Users/vuong/ERG.Workspace/erg-go
go test ./...
```

Nếu repo lớn và bạn muốn xem kỹ hơn:

```bash
go test ./... -v
```

### Nếu muốn test vài phần quan trọng mà không phải chờ quá lâu

```bash
go test ./pkg/config ./pkg/queue ./pkg/scraper ./pkg/event ./cmd/server
```

### Nếu muốn test bot/crawler/trending

```bash
go test ./internal/modules/bot/...
go test ./internal/modules/crawler
go test ./internal/modules/trending
```

### Hiểu kết quả test

Nếu bạn thấy:

```text
ok
```

nghĩa là phần đó pass.

Nếu bạn thấy:

```text
FAIL
```

nghĩa là có lỗi cần sửa.

---

## 7. Cách kiểm tra nhanh hệ thống sau khi chạy

Sau khi app đã chạy bằng `go run ./cmd/server`, bạn có thể kiểm tra theo checklist này.

### Kiểm tra 1: app sống

```bash
curl http://localhost:8080/healthz
```

### Kiểm tra 2: có metrics

```bash
curl http://localhost:8080/metrics
```

### Kiểm tra 3: route bot health

```bash
curl http://localhost:8080/api/bot/healthz
```

### Kiểm tra 4: route crawler/trending nếu cần

```bash
curl http://localhost:8080/api/trending/ready
curl http://localhost:8080/api/trending/sources
```

Nếu không chắc endpoint nào hợp lệ, chỉ cần bắt đầu với `healthz` và `metrics`.

---

## 8. Cách xem log khi app lỗi

### Nếu chạy bằng `go run`

Log sẽ hiện ngay trong terminal đang chạy app.

### Nếu chạy bằng Docker cho MongoDB/Redis

Xem log MongoDB:

```bash
docker compose logs -f mongodb
```

Xem log Redis:

```bash
docker compose logs -f redis
```

---

## 9. Lỗi thường gặp và cách hiểu

### Lỗi 1: `connect to MongoDB`

Ý nghĩa:

- app không kết nối được MongoDB

Cách xử lý:

```bash
docker compose up -d mongodb
docker compose ps
```

Sau đó chạy lại app.

### Lỗi 2: `connect to Redis`

Ý nghĩa:

- app không kết nối được Redis

Cách xử lý:

```bash
docker compose up -d redis
docker compose ps
```

### Lỗi 3: `load config`

Ý nghĩa:

- cấu hình đang thiếu hoặc sai

Cách xử lý:

- kiểm tra file `.env`
- kiểm tra nếu bạn có `config.yaml` tự tạo thì nó có đúng không
- nếu môi trường là `production`, không được để CORS là `*`

### Lỗi 4: `address already in use`

Ý nghĩa:

- cổng `8080` đang bị chương trình khác dùng

Cách xử lý:

- tắt chương trình đang chiếm cổng
- hoặc đổi cấu hình port

### Lỗi 5: test fail nhưng app vẫn chạy được

Ý nghĩa:

- phần nào đó trong code có bug hoặc có test chưa cập nhật
- không có nghĩa là app chắc chắn "chết", nhưng không nên xem là ổn hoàn toàn

---

## 10. Nên dùng lệnh nào, tránh lệnh nào?

### Nên dùng

```bash
docker compose up -d mongodb redis
go run ./cmd/server
go build -o erg-server ./cmd/server
go test ./...
```

### Cẩn thận với `Makefile`

File `Makefile` trong repo vẫn còn nhiều target của kiến trúc cũ nhiều service.

Ví dụ:

- `make docker-up` vẫn hữu ích để bật stack Docker
- nhưng các target như build/running từng service cũ có thể không còn khớp với trạng thái code hiện tại

Nếu bạn không chắc, hãy ưu tiên:

```bash
go run ./cmd/server
```

thay vì chạy target cũ trong `Makefile`.

---

## 11. Quy trình ngắn gọn nhất cho người mới

Nếu bạn chỉ muốn biết "repo này có chạy được không", hãy làm đúng 5 bước sau:

### Bước 1

```bash
cd /Users/vuong/ERG.Workspace/erg-go
```

### Bước 2

```bash
docker compose up -d mongodb redis
```

### Bước 3

```bash
go run ./cmd/server
```

### Bước 4

Mở terminal mới:

```bash
curl http://localhost:8080/healthz
```

### Bước 5

Nếu muốn test code:

```bash
go test ./...
```

Nếu cả `healthz` và `go test` đều ổn, bạn có thể xem như project đang chạy tốt ở mức cơ bản.

---

## 12. Khi nào cần nhờ người kỹ thuật?

Bạn nên nhờ người hỗ trợ nếu gặp một trong các tình huống sau:

- Docker không mở được
- `go version` không có
- app báo lỗi kết nối MongoDB/Redis liên tục dù container đang chạy
- test fail quá nhiều và bạn không hiểu log
- cần cấu hình bot thật với Discord, Telegram, SMTP, Gemini API

Những phần đó không phải "chạy project cơ bản", mà là "cấu hình môi trường thật".

---

## 13. Gợi ý thực tế

Nếu mục tiêu của bạn là kiểm tra nhanh:

1. bật MongoDB + Redis bằng Docker
2. chạy `go run ./cmd/server`
3. gọi `curl http://localhost:8080/healthz`
4. chạy `go test ./...`

Đó là bộ kiểm tra gọn nhất và đáng tin nhất cho repo này ở thời điểm hiện tại.
