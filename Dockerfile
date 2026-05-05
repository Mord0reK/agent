FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o vm-agent .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    systemd \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/vm-agent /usr/local/bin/vm-agent
ENTRYPOINT ["/usr/local/bin/vm-agent"]
