# XFile

XFile is a self-hosted file management service. The backend lives at the repository root and the frontend lives in `web/`.

## Project Structure

- `main.go`: Go backend entry point.
- `internal/`: backend config, SQLite database, store, domain models, and HTTP API modules.
- `web/`: Vite + Vue 3 + Element Plus admin UI.
- `web/dist/`: frontend production build read by the Go backend.
- `data/`: runtime SQLite database and uploaded files.

## Feature Reference

ZFile demo core features:

- Multiple storage sources and file browsing.
- Site settings, logo, root path name, backend URL, and upload concurrency.
- Share list, direct links, short links, download logs, and rankings.
- Upload rules, display rules, user rules, and access control.
- WebDAV, user management, SSO, security settings, and system logs.

Zdir Pro core features:

- Public/private file indexing.
- File upload, download, copy, move, rename, and delete.
- Online preview for images, audio, video, and documents.
- Text editing, file descriptions, and global search.
- Share links with expiration time.
- WebDAV server and S3/WebDAV external storage mounts.
- Online archive extraction, offline download, music list, HTML tools, and API.

## Development

Start the backend:

```bash
go run .
```

Start the frontend:

```bash
cd web
pnpm install
pnpm dev
```

Vite proxies `/api` to `http://localhost:3008`.

## Build And Test

```bash
cd web && pnpm build
go test ./...
```

The Go service reads frontend assets from `web/dist` by default. Override it with `XFILE_STATIC_DIR` if needed.

## Configuration

- First visit `/login` to initialize the system super administrator account and password.
- `XFILE_SESSION_SECRET`: optional signing key for login cookies. If omitted, a process-local secret is generated on startup.
- `XFILE_DATA_DIR`: runtime data directory, default `data`.
- `XFILE_FILES_DIR`: uploaded file directory, default `data/files`.
- `XFILE_DB`: SQLite database path, default `data/xfile.db`.
- `XFILE_STATIC_DIR`: frontend build directory served by the backend, default `web/dist`.

## Docker

```bash
docker build -t xfile:local .
docker run --rm -p 3008:3008 -v ./data:/app/data xfile:local
```

Or:

```bash
docker compose up --build
```

GitHub Actions runs Go tests, frontend typecheck/build, then builds the Docker image on `main` / `master`, tags, pull requests, and manual runs. Non-PR builds are pushed to GHCR.
