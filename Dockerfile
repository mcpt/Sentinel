FROM golang:1.21-alpine AS builder
LABEL org.opencontainers.image.source="https://github.com/mcpt/sentinel"
LABEL org.opencontainers.image.authors="Jason Cameron <sentinel+mcpt@jasoncameron.dev>"

WORKDIR /app

COPY . .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o sentinel

FROM alpine:latest

RUN apk --no-cache add ca-certificates mariadb-client

WORKDIR /root/

COPY --from=builder /app/sentinel .

CMD ["./sentinel"]