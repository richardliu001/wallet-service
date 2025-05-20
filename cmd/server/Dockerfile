# builder stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
WORKDIR /app/cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o wallet-server main.go

# runtime stage
FROM alpine:3.17
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/cmd/server/wallet-server .
COPY --from=builder /app/internal/config/config.yaml ./internal/config/config.yaml
EXPOSE 8080
ENTRYPOINT ["./wallet-server"]