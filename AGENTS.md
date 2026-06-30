# Repository Guidelines

## Project Structure & Module Organization

`xfile` is a Go web service with a Vite/Vue admin UI. The backend stays at the repository root and is split into layered packages:

- `main.go`: backend entry point.
- `internal/config`: environment configuration.
- `internal/database`: SQLite opening and migrations.
- `internal/domain`: JSON/domain models.
- `internal/store`: persistence and file-storage business logic.
- `internal/server`: HTTP routing, auth, handlers, and static frontend serving.
- `web/`: Vue 3 + Vite + Element Plus admin UI.
- `web/dist/`: production frontend assets read by the Go backend.
- `data/`: default runtime data, including `data/xfile.db` and uploads in `data/files/`.

Docker, Compose, and GitHub Actions files live at the repository root. The backend serves `web/dist` by default through `XFILE_STATIC_DIR`.

## Build, Test, And Development Commands

- `go run .`: starts the backend on `http://localhost:3008`.
- `go test ./...`: runs all Go tests.
- `cd web && pnpm install`: installs frontend dependencies from `pnpm-lock.yaml`.
- `cd web && pnpm dev`: starts the Vite dev server on `http://localhost:5173` with `/api` proxied to the Go server.
- `cd web && pnpm typecheck`: runs Vue/TypeScript checks.
- `cd web && pnpm build`: builds static assets into `web/dist`.
- `docker build -t xfile:local .`: builds the combined production image.
- `docker compose up --build`: runs the combined service locally.

On first use, visit `/login` and initialize the system super administrator account and password.

## Current Implemented Features

- First-use super administrator initialization stored in SQLite.
- Admin login/logout with signed HTTP-only session cookie.
- Protected management API requiring login.
- SQLite persistence using `github.com/glebarez/sqlite` with pure-Go builds.
- Local file browsing, upload, download, delete, create folder, rename, and move.
- Upload enable/disable and max upload size settings.
- Share links with optional password, expiration field, hashed password storage, and a dedicated `/s/{token}` landing page.
- Direct links with enable/disable support.
- Access logging for file operations, shares, and direct links.
- Dashboard summary for file count, folder count, storage usage, shares, recent files, and logs.
- Vue pages for first-use setup/login, file management, public share landing, shares, direct links, logs, settings, rules, uploads, access, and WebDAV placeholder.
- Dockerfile builds `web/dist` and embeds it in the Go runtime image.
- GitHub Actions runs Go tests, frontend typecheck/build, then Docker image build/publish.

## Known Missing Features / Backlog

- Real WebDAV protocol implementation, account management, permissions, and mount paths.
- Multiple storage sources such as S3, MinIO, external WebDAV, or cloud drive mounts.
- Online previews for images, video, audio, PDF, text, and office documents.
- Text editing and richer file metadata/descriptions.
- Batch operations: batch delete, move, share, and archive download.
- Nested folder share browsing.
- Global backend search and server-side sorting/filtering.
- Large-file chunked upload, folder upload, upload queue, progress, and resumable uploads.
- Access-control rules: IP allow/deny lists, private directory rules, Referer protection, and download rate limiting.
- User/role permission model beyond a single admin password.
- Audit enhancements: log pagination, filtering, cleanup, download counts, and share visit statistics.
- Security hardening: login rate limiting, CSRF protection, share password attempt limiting, and stronger reverse-proxy deployment docs.
- More tests: HTTP handler tests, auth tests, migration tests, and frontend interaction tests.

Recommended next priorities: nested folder share browsing, global search, log pagination/filtering, and richer access-control rules.

## Coding Style & Naming Conventions

Format Go code with `gofmt` and keep package names short and lowercase. Prefer small helper functions near related handlers, preserve explicit error handling, and keep JSON response structs tagged with lower camelCase field names.

Frontend code uses TypeScript, Vue single-file components, and Element Plus components. Use PascalCase for component names, camelCase for state/helpers, and keep global styles in `web/src/styles/index.scss` unless a local pattern emerges.

## Testing Guidelines

Add Go tests as `*_test.go` files beside the code under test. Prefer table-driven tests for path validation, authentication, share access, direct-link access, migrations, and access-control rules.

Run these before completing backend/frontend changes:

- `go test ./...`
- `cd web && pnpm typecheck`
- `cd web && pnpm build`

Docker local verification is useful when `docker` is available.

## Commit & Pull Request Guidelines

Use concise, imperative commit subjects such as `Add admin auth` or `Implement share password checks`. Pull requests should describe user-visible changes, list tests run, mention affected environment variables or storage paths, and include screenshots for UI changes.

## Security & Configuration Tips

Never expose or commit admin passwords, session secrets, database files, uploaded files, or runtime `data/` contents. Initialize the super administrator through `/login` on first use. Set `XFILE_SESSION_SECRET` for stable sessions across restarts. Keep `web/dist`, `web/node_modules`, local logs, and generated binaries untracked.
