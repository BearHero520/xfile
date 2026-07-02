# XFile Feature Backlog

This file tracks remaining file-service features while keeping XFile's own UI style.

## Phase 1 - Storage Sources

- [x] Manage multiple storage source instances.
- [x] Enable, disable, publish, privatize, and sort storage sources.
- [x] Support local storage source browsing by each source root.
- [x] Implement S3 / MinIO adapter.
- [x] Implement WebDAV adapter.
- [x] Implement Aliyun OSS adapter.
- [x] Implement Tencent COS adapter.

## Phase 2 - File Rules And Permissions

- [x] Add per-storage hidden path rules.
- [x] Add per-storage blocked path rules.
- [x] Add directory password rules.
- [x] Add operation permissions for preview, download, upload, rename, move, copy, delete, share, and direct links.
- [x] Apply rules through a single file-list processing pipeline.

## Phase 3 - Users And Roles

- [x] Assign users to storage sources.
- [x] Assign users to root paths within a storage source.
- [x] Add per-user operation permissions.
- [x] Add user enable / disable status.
- [x] Add clearer admin versus normal user behavior.

## Phase 4 - Batch Operations

- [x] Batch move.
- [x] Batch copy.
- [x] Batch share.
- [x] Batch direct-link generation.
- [x] ZIP archive download for folders and selected files.

## Phase 5 - Preview And Editing

- [x] Text file editing and save.
- [x] File descriptions / metadata notes.
- [x] Office preview integration.
- [x] Optional kkFileView / OnlyOffice integration.
- [x] More robust media preview behavior.

## Phase 6 - WebDAV Server

- [x] Real WebDAV protocol implementation.
- [x] WebDAV account and password handling.
- [x] WebDAV mount path support.
- [x] WebDAV read-only policy.
- [x] WebDAV anonymous access policy.

## Phase 7 - Security

- [x] Login rate limiting.
- [x] CSRF protection for mutating requests.
- [x] Share password attempt limiting.
- [x] Session management and forced logout.
- [x] Optional 2FA or captcha.

## Phase 8 - Share And Link Analytics

- [x] Share visit details.
- [x] Download ranking.
- [x] Expired link cleanup.
- [x] Custom share keys.
- [x] More detailed direct-link access stats.

## Phase 9 - Frontend Polish

- [ ] Drag-and-drop upload.
- [ ] Keyboard multi-select.
- [ ] Empty-area context menu coverage on all file views.
- [x] Move / copy target picker.
- [ ] Mobile layout QA for every admin page.
