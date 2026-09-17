FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/haproxy-webui ./cmd/server

FROM alpine:3
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 appuser \
    && mkdir -p /data && chown appuser /data
COPY --from=build /out/haproxy-webui /usr/local/bin/haproxy-webui
USER appuser
ENV HAPROXY_WEBUI_DB=/data/haproxy-webui.db
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["haproxy-webui"]
