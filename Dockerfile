FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o vm-agent .

FROM scratch
COPY --from=builder /app/vm-agent /vm-agent
ENTRYPOINT ["/vm-agent"]
