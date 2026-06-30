FROM node:22-alpine AS web
WORKDIR /src/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/index.html web/tsconfig.json web/vite.config.ts web/uno.config.ts web/eslint.config.js ./
COPY web/public ./public
COPY web/src ./src
RUN pnpm build

FROM golang:1.22-alpine AS api
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY internal ./internal
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/xfile .

FROM debian:bookworm-slim
WORKDIR /app
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*
COPY --from=api /out/xfile /app/xfile
COPY --from=web /src/web/dist /app/web/dist
ENV XFILE_ADDR=:3008 \
  XFILE_DATA_DIR=/app/data \
  XFILE_FILES_DIR=/app/data/files \
  XFILE_DB=/app/data/xfile.db \
  XFILE_STATIC_DIR=/app/web/dist
EXPOSE 3008
VOLUME ["/app/data"]
CMD ["/app/xfile"]
