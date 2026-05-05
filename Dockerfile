FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o vm-agent .

FROM debian:bookworm-slim AS deps
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    systemd \
    && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /out/lib && \
    cp -r /lib/x86_64-linux-gnu /out/lib/ && \
    mkdir -p /out/usr/bin && \
    cp /usr/bin/journalctl /out/usr/bin/ && \
    cp /bin/systemctl /out/bin/ 2>/dev/null || true && \
    mkdir -p /out/etc && \
    cp /etc/ld.so.cache /out/etc/ 2>/dev/null || true && \
    mkdir -p /out/var/lib/systemd && \
    cp -r /var/lib/systemd/catalog /out/var/lib/systemd/ 2>/dev/null || true

FROM scratch
COPY --from=builder /app/vm-agent /vm-agent
COPY --from=deps /out/lib/x86_64-linux-gnu /lib/x86_64-linux-gnu
COPY --from=deps /out/usr/bin/journalctl /usr/bin/journalctl
COPY --from=deps /out/etc/ld.so.cache /etc/ld.so.cache
COPY --from=deps /out/var/lib/systemd /var/lib/systemd
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/vm-agent"]
