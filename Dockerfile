# syntax=docker/dockerfile:1.7

FROM golang:1.24-alpine AS dependencias
WORKDIR /src

COPY spa-api/go.mod spa-api/go.sum ./
RUN go mod download

FROM dependencias AS pruebas
COPY spa-api/ ./
RUN go test ./... && go vet ./...

FROM pruebas AS compilacion
ARG VERSION=dev
ARG COMMIT=desconocido
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X spa-api/internal/version.Version=${VERSION} -X spa-api/internal/version.Commit=${COMMIT}" \
    -o /salida/spa-api ./cmd/api

FROM alpine:3.22 AS final
RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S spa \
    && adduser -S -G spa spa \
    && mkdir -p /app/ClientApp /data/uploads \
    && chown -R spa:spa /app /data

WORKDIR /app
COPY --from=compilacion --chown=spa:spa /salida/spa-api /app/spa-api
COPY --chown=spa:spa spa-api/ClientApp/ /app/ClientApp/
RUN test -f /app/ClientApp/index.html

USER spa

ENV APP_ENV=production \
    PORT=8080 \
    CLIENT_APP_DIR=/app/ClientApp \
    UPLOAD_DIR=/data/uploads

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/api/health >/dev/null || exit 1

ENTRYPOINT ["/app/spa-api"]
CMD ["serve"]
