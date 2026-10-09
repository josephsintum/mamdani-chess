# syntax=docker/dockerfile:1

# 1. Build the SvelteKit app.
FROM node:22-alpine AS web
RUN corepack enable
WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# 2. Build the Go server with the frontend embedded.
FROM golang:1.27-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/build ./web/build
RUN CGO_ENABLED=0 go build -tags embedweb -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# 3. DB-IP's free City Lite database (CC BY 4.0; credited on /about): this
#    month's file, or last month's early in a month. Railway's build cache
#    keeps the layer, so the file refreshes when the cache does.
FROM alpine:3.21 AS geo
RUN apk add --no-cache curl
RUN set -e; for m in $(date +%Y-%m) $(date -d @$(( $(date +%s) - 20*86400 )) +%Y-%m); do \
      curl -fsSL -o /dbip.mmdb.gz "https://download.db-ip.com/free/dbip-city-lite-$m.mmdb.gz" && break; done; \
    gunzip /dbip.mmdb.gz && ls -l /dbip.mmdb

# 4. Run it. Root image: Railway volumes mount as root, so a nonroot
#    user could not write the SQLite file on /data.
FROM gcr.io/distroless/static-debian12
COPY --from=server /out/server /server
COPY --from=geo /dbip.mmdb /geo/dbip-city-lite.mmdb
ENV PORT=8080 DB_PATH=/data/mamdani.db LOG_FORMAT=json GEOIP_PATH=/geo/dbip-city-lite.mmdb
EXPOSE 8080
ENTRYPOINT ["/server"]
