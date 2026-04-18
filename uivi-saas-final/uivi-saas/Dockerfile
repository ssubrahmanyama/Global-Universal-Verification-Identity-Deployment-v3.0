FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /app

# Copy go.mod first for cache efficiency
COPY go.mod ./
# Download deps (generates go.sum automatically)
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/server ./server
COPY frontend ./frontend
EXPOSE 8080
ENTRYPOINT ["./server"]
