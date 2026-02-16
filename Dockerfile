FROM golang:1.23-alpine AS builder

WORKDIR /build

COPY go.mod .
COPY main.go .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /restarter main.go

RUN apk add --no-cache upx && upx --best --lzma /restarter

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /restarter /restarter

ENTRYPOINT ["/restarter"]
