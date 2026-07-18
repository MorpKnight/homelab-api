# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/homelab-api ./cmd/server

FROM alpine:3.22

RUN addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app \
    && apk add --no-cache ca-certificates

COPY --from=build /out/homelab-api /usr/local/bin/homelab-api

USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/homelab-api"]
