# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26
ARG ALPINE_VERSION=3.21

FROM golang:${GO_VERSION}-alpine AS builder

RUN apk add --no-cache \
    build-base \
    ca-certificates \
    git \
    tzdata

WORKDIR /src

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 \
    GOOS="${TARGETOS:-linux}" \
    GOARCH="${TARGETARCH:-$(go env GOARCH)}" \
    go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/codechat-api \
    ./cmd/api

FROM alpine:${ALPINE_VERSION} AS runtime

ARG APP_NAME=whatsapp-go-api
ARG APP_VERSION=dev
ARG APP_DESCRIPTION="API HTTP em Go para gerenciar instancias do WhatsApp com Whatsmeow"
ARG APP_DEVELOPER=CodeChat
ARG APP_REPOSITORY=
ARG BUILD_DATE=
ARG VCS_REF=
ARG VCS_URL=

LABEL org.opencontainers.image.title="${APP_NAME}" \
      org.opencontainers.image.description="${APP_DESCRIPTION}" \
      org.opencontainers.image.version="${APP_VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.source="${VCS_URL}" \
      org.opencontainers.image.url="${APP_REPOSITORY}" \
      org.opencontainers.image.documentation="${APP_REPOSITORY}" \
      org.opencontainers.image.authors="${APP_DEVELOPER}" \
      com.codechat.developer="${APP_DEVELOPER}"

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    ffmpeg \
    wget \
    && command -v ffmpeg >/dev/null \
    && command -v ffprobe >/dev/null \
    && addgroup -S app \
    && adduser -S -G app -h /app app \
    && mkdir -p /app/tmp /app/data /app/internal/database/migrations \
    && chown -R app:app /app

WORKDIR /app

COPY --from=builder --chown=app:app /out/codechat-api /app/codechat-api
COPY --chown=app:app internal/database/migrations /app/internal/database/migrations

ENV DOCKER_ENV=true
ENV TZ=America/Sao_Paulo
ENV SERVER_PORT=8084
ENV FFMPEG_PATH=/usr/bin/ffmpeg
ENV FFPROBE_PATH=/usr/bin/ffprobe
ENV TMPDIR=/app/tmp

USER app

EXPOSE 8084

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=5 \
    CMD wget --spider -q "http://127.0.0.1:${SERVER_PORT:-8084}/health" || exit 1

ENTRYPOINT ["/app/codechat-api"]
