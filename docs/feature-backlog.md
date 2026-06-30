# XFile Feature Backlog

This file tracks the remaining ZFile-inspired features to add while keeping XFile's own UI style.

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
- [ ] Add directory password rules.
- [ ] Add operation permissions for preview, download, upload, rename, move, copy, delete, share, and direct links.
- [ ] Apply rules through a single file-list processing pipeline.

## Phase 3 - Users And Roles

- [ ] Assign users to storage sources.
- [ ] Assign users to root paths within a storage source.
- [ ] Add per-user operation permissions.
- [ ] Add user enable / disable status.
- [ ] Add clearer admin versus normal user behavior.

## Phase 4 - Batch Operations

- [ ] Batch move.
- [ ] Batch copy.
- [ ] Batch share.
- [ ] Batch direct-link generation.
- [ ] ZIP archive download for folders and selected files.

## Phase 5 - Preview And Editing

- [ ] Text file editing and save.
- [ ] File descriptions / metadata notes.
- [ ] Office preview integration.
- [ ] Optional kkFileView / OnlyOffice integration.
- [ ] More robust media preview behavior.

## Phase 6 - WebDAV Server

- [ ] Real WebDAV protocol implementation.
- [ ] WebDAV account and password handling.
- [ ] WebDAV mount path support.
- [ ] WebDAV read-only policy.
- [ ] WebDAV anonymous access policy.

## Phase 7 - Security

- [ ] Login rate limiting.
- [ ] CSRF protection for mutating requests.
- [ ] Share password attempt limiting.
- [ ] Session management and forced logout.
- [ ] Optional 2FA or captcha.

## Phase 8 - Share And Link Analytics

- [ ] Share visit details.
- [ ] Download ranking.
- [ ] Expired link cleanup.
- [ ] Custom share keys.
- [ ] More detailed direct-link access stats.

## Phase 9 - Frontend Polish

- [ ] Drag-and-drop upload.
- [ ] Keyboard multi-select.
- [ ] Empty-area context menu coverage on all file views.
- [ ] Move / copy target picker.
- [ ] Mobile layout QA for every admin page.
