# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Build context is the project root (context: .)
COPY backend/go.mod ./
COPY backend/cmd/ cmd/
COPY backend/internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/server

# Final runtime stage
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/server /app/server
COPY data/ /app/data/

ENV APP_ENV=production \
    BACKEND_PORT=8080

EXPOSE 8080

CMD ["/app/server"]
