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

# 3. Run it. Root image: Railway volumes mount as root, so a nonroot
#    user could not write the SQLite file on /data.
FROM gcr.io/distroless/static-debian12
COPY --from=server /out/server /server
ENV PORT=8080 DB_PATH=/data/mamdani.db LOG_FORMAT=json
EXPOSE 8080
ENTRYPOINT ["/server"]
