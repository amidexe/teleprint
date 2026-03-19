# syntax=docker/dockerfile:1

# ── Stage 1: сборка ──────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o teleprint ./cmd/teleprint/

# ── Stage 2: финальный образ ─────────────────────────────────────────────────
FROM alpine:3.21

# Ghostscript для конвертации PDF → URF
RUN apk add --no-cache ghostscript ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/teleprint .

# users.json хранится в volume — монтируется снаружи
VOLUME ["/app/data"]

# Переопределяем путь к users.json через рабочую директорию
ENV HOME=/app

ENTRYPOINT ["./teleprint"]
