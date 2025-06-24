FROM golang:1.24.4-alpine3.22 AS builder

WORKDIR /build

COPY go.mod go.mod
COPY go.sum go.sum
COPY cmd/ cmd/
COPY internal/ internal/

ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/root/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags="-extldflags=-static -s -w" -o bot cmd/svoibot/main.go


FROM scratch

WORKDIR /app

COPY --from=builder /build/bot bot
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/app/bot"]
