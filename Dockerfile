FROM golang:alpine AS builder

WORKDIR /app

COPY . .

ENV CGO_ENABLED=0

RUN go build -o /worker cmd/main.go

FROM alpine
RUN apk add --no-cache perl-utils zstd
COPY --from=builder /worker /bin/worker

ENTRYPOINT /bin/worker
