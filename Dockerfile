# Go-RED Dockerfile: builds the WebUI and the Go binary in separate stages
# and ships both in a small Alpine image.

# --- WebUI -------------------------------------------------------------
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# --- Go binary ---------------------------------------------------------
FROM golang:1.25-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/go-red ./cmd/go-red

# --- Runtime -----------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget \
    && addgroup -S gored && adduser -S gored -G gored
WORKDIR /app
COPY --from=build /out/go-red ./go-red
COPY --from=web /src/web/dist ./web/dist
RUN mkdir -p /app/data && chown -R gored:gored /app
USER gored

EXPOSE 8080
VOLUME ["/app/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

ENTRYPOINT ["./go-red"]
CMD ["-port", "8080", "-data-dir", "/app/data", "-web-dir", "/app/web/dist"]
