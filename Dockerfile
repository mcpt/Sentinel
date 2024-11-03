FROM golang:1.23-alpine AS builder
LABEL org.opencontainers.image.source="https://github.com/mcpt/sentinel"
LABEL org.opencontainers.image.authors="Jason Cameron <sentinel+mcpt@jasoncameron.dev>"

WORKDIR /app

COPY . .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o sentinel ./sentinel

FROM alpine:latest

RUN apk --no-cache add ca-certificates mariadb-client tar zstd zlib gzip

WORKDIR /root/

COPY --from=builder /app/sentinel .

ENTRYPOINT ["./sentinel"]