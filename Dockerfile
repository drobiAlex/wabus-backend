FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /wabus ./cmd/wabus

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /wabus /wabus

ENV TZ=Europe/Warsaw

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=60s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/wabus"]
