# XFile HTTP API v1

XFile exposes only the `/api/v1` contract. The previous unversioned routes and
legacy controller naming styles are intentionally unsupported.

## Naming rules

- Resource nouns are plural: `entries`, `shares`, `accounts`, `storage-nodes`.
- Identity endpoints live under `/identity`; no `/auth/*` aliases are exposed.
- File operations live under `/workspace`; public browsing uses `/public/drives`.
- Administrative resources live under `/admin`; audit and insight data use
  `/audit` and `/insights`.
- Actions that affect multiple resources use `/actions/{verb}` and `POST`.
- Public UI routes use `/share/{token}` and `/open/{token}`.

## Endpoint groups

| Area | Base path |
| --- | --- |
| Health | `/api/v1/system/health` |
| Identity | `/api/v1/identity` |
| Workspace files | `/api/v1/workspace` |
| User favorites | `/api/v1/workspace/favorites` |
| Public drives and shares | `/api/v1/public` |
| Shares | `/api/v1/collaboration/shares` |
| Direct delivery links | `/api/v1/delivery/links` |
| Link insights | `/api/v1/insights/links` |
| Audit events | `/api/v1/audit/events` |
| Accounts | `/api/v1/admin/accounts` |
| Storage nodes | `/api/v1/admin/storage-nodes` |
| Preferences | `/api/v1/preferences` |

Mutating requests require the `X-CSRF-Token` header after authentication.

## Storage-scoped links

Shares and delivery links are always tied to an explicit XFile storage node.
Clients must send the current `storageKey`; omitting it defaults to `local` only
for backwards-compatible data migration inside XFile itself.

```json
POST /api/v1/collaboration/shares
{
  "storageKey": "team-drive",
  "path": "docs/guide.pdf",
  "password": "",
  "expiresAt": ""
}
```

```json
POST /api/v1/delivery/links
{
  "storageKey": "team-drive",
  "path": "docs/guide.pdf"
}
```

Share, share-detail, and delivery-link responses return `storageKey` alongside
`path`. Public share content and `/open/{token}` downloads resolve through the
selected storage adapter, including local, S3-compatible, WebDAV, Aliyun OSS,
and Tencent COS nodes.

## Aggregate shares and idempotent short links

`POST /api/v1/collaboration/shares/batch` accepts multiple `paths` and returns
one share resource. The response includes `paths` and `itemCount`; the public
share page exposes the selected files and folders as one virtual root.

Creating a delivery link for the same `storageKey` and `path` returns the
existing link instead of inserting a duplicate.

## Favorites

Authenticated users can list, add, and delete their own favorites:

```text
GET    /api/v1/workspace/favorites
POST   /api/v1/workspace/favorites
DELETE /api/v1/workspace/favorites/{id}
```
