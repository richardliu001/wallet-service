# builder
FROM golang:1.19-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
WORKDIR /app/cmd/poller
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o wallet-poller main.go

# runtime
FROM alpine:3.17
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/cmd/poller/wallet-poller .
COPY --from=builder /app/internal/config/config.yaml ./internal/config/config.yaml
ENTRYPOINT ["./wallet-poller"]