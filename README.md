# homelab-api

Backend adapter untuk menyediakan data monitoring homelab kepada client seperti iOS app. Repository ini hanya berisi service backend dan deployment Docker; tidak berisi kode iOS.

## Arsitektur

```text
iOS app --HTTPS--> homelab-api --REST/PocketBase--> Beszel
                         |
                         +-- Prometheus /metrics --> Uptime Kuma
```

Service melakukan polling upstream secara berkala, menyimpan snapshot normalisasi di memory, lalu menyediakan REST API dan SSE. Interval realtime maksimum tetap dibatasi oleh cadence data dari Beszel/Uptime Kuma.

Adapter ini sengaja menjadi kontrak untuk client. Client tidak perlu mengetahui struktur internal PocketBase Beszel atau format Prometheus Uptime Kuma.

## Endpoint

Semua endpoint `/api/*` menggunakan `Authorization: Bearer <API_TOKEN>` jika `API_TOKEN` dikonfigurasi.

- `GET /healthz` — proses hidup.
- `GET /readyz` — minimal satu upstream telah berhasil atau sebagian berhasil dibaca.
- `GET /api/v1/overview` — snapshot gabungan systems, containers, services, alerts, summary, dan status source.
- `GET /api/v1/systems`
- `GET /api/v1/containers`
- `GET /api/v1/services`
- `GET /api/v1/alerts`
- `GET /api/v1/events` — SSE; mengirim event `snapshot` setelah polling menghasilkan data.

Contoh:

```sh
curl -H "Authorization: Bearer $API_TOKEN" http://localhost:8080/api/v1/overview
curl -N -H "Authorization: Bearer $API_TOKEN" http://localhost:8080/api/v1/events
```

## Konfigurasi lokal

```sh
cp .env.example .env
chmod 600 .env
```

Isi `.env` dengan kredensial read-only Beszel dan API key metrics Uptime Kuma. File `.env` diabaikan Git. Gunakan `BESZEL_PASSWORD_FILE`, `UPTIME_KUMA_API_KEY_FILE`, atau `API_TOKEN_FILE` bila secret ingin disediakan melalui mounted file.

Untuk menjalankan binary tanpa Docker:

```sh
go test ./...
go run ./cmd/server
```

## Docker Compose

Compose mengasumsikan network eksternal `cf-tunnel` telah dibuat oleh deployment Cloudflare Tunnel. Service tidak mem-publish port ke host; hanya menggunakan `expose` dan dapat di-route ke `homelab-api:8080` dari Cloudflare Tunnel.

```sh
docker compose up -d --build
docker compose ps
docker inspect -f '{{.State.Health.Status}}' homelab-api
```

Beszel dan Uptime Kuma harus dapat dijangkau dari network Docker yang sama atau melalui alamat internal yang disediakan di `.env`. Jangan berikan Docker socket kepada adapter ini.

## GitHub

Repository lokal ini belum memiliki remote karena URL/owner GitHub belum ditentukan. CI yang disiapkan menjalankan `go test`, `go vet`, dan Docker build. Jangan commit `.env`, password, API key, atau token.

## Catatan kontrak data

Beszel dibangun di atas PocketBase dan struktur API/collection dapat berubah antar versi. Nama collection dibuat configurable melalui environment. Field numerik yang aman dinormalisasi ke `metrics`; field credential/secret tidak diteruskan ke response. Setelah Beszel dan Uptime Kuma berjalan di homelab, mapping field perlu divalidasi terhadap response aktual sebelum API dianggap production-ready.

`services[].response_time` mengikuti nilai mentah metric `monitor_response_time` dari Uptime Kuma; unitnya tidak dikonversi oleh adapter. URL monitor yang masuk response dibersihkan dari userinfo, query, dan fragment agar token health-check tidak ikut terekspos.
