# Go 后端 Dockerfile — 多阶段构建
# 构建方式（唯一约定）:
#   docker build -f deploy/Dockerfile -t animal-poke-backend:local ./backend
# 上下文必须是 backend/（含 go.mod），勿用仓库根目录。

# syntax=docker/dockerfile:1.6
FROM --platform=$BUILDPLATFORM golang:1.25.12-alpine AS builder

WORKDIR /src
ARG TARGETOS
ARG TARGETARCH
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/animal-poke-backend ./cmd && \
    go build -trimpath -ldflags="-s -w" -o /out/animal-poke-migrate ./cmd/migrate

FROM alpine:3.20
ARG VERSION=dev
ARG VCS_REF=unknown
ARG BUILD_DATE=unknown
LABEL org.opencontainers.image.title="animal-poke-backend" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="https://github.com/Bluepoisons/animal_poke"
RUN apk --no-cache add ca-certificates tzdata wget \
    && adduser -D -H -u 10001 appuser
WORKDIR /app
COPY --from=builder /out/animal-poke-backend /out/animal-poke-migrate .

USER 10001:10001
ENV SERVER_ADDR=:8080 \
    APP_ENV=production \
    GIN_MODE=release
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/livez || exit 1

CMD ["./animal-poke-backend"]
