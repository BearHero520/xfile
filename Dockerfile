FROM node:22-alpine AS web-builder
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS api-builder
WORKDIR /src
COPY go.mod ./
COPY cmd/ ./cmd/
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/xfile ./cmd/xfile

FROM alpine:3.20
WORKDIR /app
RUN addgroup -S xfile && adduser -S xfile -G xfile
COPY --from=api-builder /out/xfile /app/xfile
COPY --from=web-builder /src/web/dist /app/web/dist
RUN mkdir -p /app/data/files && chown -R xfile:xfile /app
USER xfile
ENV XFILE_PORT=3008
ENV XFILE_DATA_DIR=/app/data
ENV XFILE_WEB_DIR=/app/web/dist
EXPOSE 3008
VOLUME ["/app/data"]
ENTRYPOINT ["/app/xfile"]
