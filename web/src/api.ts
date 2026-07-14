import type {
  AccessLogPage,
  AccountInput,
  AboutData,
  AnnouncementEntry,
  AnnouncementInput,
  AuthSession,
  CaptchaChallenge,
  Dashboard,
  DirectLinkEntry,
  EntryDetails,
  FavoriteEntry,
  FileEntry,
  LinkAnalytics,
  PublicSite,
  SessionEntry,
  ShareDetail,
  ShareEntry,
  StorageSource,
  StorageSourceInput,
  ThemeSettings,
  UserEntry,
} from "./types";

type RequestOptions = RequestInit & {
  skipAuthRedirect?: boolean;
  skipCsrf?: boolean;
};

export const endpoints = {
  about: "/api/v1/system/about",
  publicBootstrap: "/api/v1/public/bootstrap",
  identityBootstrap: "/api/v1/identity/bootstrap",
  identitySession: "/api/v1/identity/session",
  identityChallenge: "/api/v1/identity/challenge",
  overview: "/api/v1/workspace/overview",
  entries: "/api/v1/workspace/entries",
  entryDetails: "/api/v1/workspace/entries/details",
  search: "/api/v1/workspace/search",
  folders: "/api/v1/workspace/folders",
  documents: "/api/v1/workspace/documents",
  uploads: "/api/v1/workspace/uploads",
  textContent: "/api/v1/workspace/text-content",
  metadata: "/api/v1/workspace/metadata",
  content: "/api/v1/workspace/content",
  archives: "/api/v1/workspace/archives",
  move: "/api/v1/workspace/actions/move",
  copy: "/api/v1/workspace/actions/copy",
  favorites: "/api/v1/workspace/favorites",
  shares: "/api/v1/collaboration/shares",
  deliveryLinks: "/api/v1/delivery/links",
  insights: "/api/v1/insights/links",
  audit: "/api/v1/audit/events",
  accounts: "/api/v1/admin/accounts",
  storageNodes: "/api/v1/admin/storage-nodes",
  theme: "/api/v1/admin/theme",
  announcements: "/api/v1/admin/announcements",
  preferences: "/api/v1/preferences",
} as const;

const csrfHeader = "X-CSRF-Token";
let csrfToken = "";
let sessionRequest: Promise<AuthSession> | null = null;

function rememberToken(value: unknown) {
  if (value && typeof value === "object" && "csrfToken" in value) {
    const next = (value as { csrfToken?: unknown }).csrfToken;
    csrfToken = typeof next === "string" ? next : "";
  }
}

async function parseError(response: Response) {
  const text = await response.text();
  if (!text) return response.statusText || `HTTP ${response.status}`;
  try {
    const parsed = JSON.parse(text) as { error?: string };
    return parsed.error || text;
  } catch {
    return text;
  }
}

async function ensureCsrf() {
  if (csrfToken) return csrfToken;
  if (!sessionRequest) {
    sessionRequest = request<AuthSession>(endpoints.identitySession, {
      skipAuthRedirect: true,
      skipCsrf: true,
    }).finally(() => {
      sessionRequest = null;
    });
  }
  const session = await sessionRequest;
  rememberToken(session);
  return csrfToken;
}

export async function request<T>(
  url: string,
  options: RequestOptions = {},
): Promise<T> {
  const method = (options.method || "GET").toUpperCase();
  const headers = new Headers(options.headers);
  if (
    options.body &&
    !(options.body instanceof FormData) &&
    !headers.has("Content-Type")
  ) {
    headers.set("Content-Type", "application/json");
  }
  if (!options.skipCsrf && !["GET", "HEAD", "OPTIONS"].includes(method)) {
    const token = await ensureCsrf();
    if (token) headers.set(csrfHeader, token);
  }
  const { skipAuthRedirect } = options;
  const fetchOptions: RequestInit = { ...options };
  delete (fetchOptions as RequestOptions).skipAuthRedirect;
  delete (fetchOptions as RequestOptions).skipCsrf;
  const response = await fetch(url, {
    ...fetchOptions,
    method,
    headers,
    credentials: "same-origin",
  });
  if (
    response.status === 401 &&
    !skipAuthRedirect &&
    !location.pathname.startsWith("/login") &&
    !location.pathname.startsWith("/share/")
  ) {
    const redirect = encodeURIComponent(location.pathname + location.search);
    location.assign(`/login?redirect=${redirect}`);
    throw new Error("登录状态已失效");
  }
  if (!response.ok) throw new Error(await parseError(response));
  if (response.status === 204) return undefined as T;
  const data = (await response.json()) as T;
  rememberToken(data);
  return data;
}

export const api = {
  about: (refresh = false) =>
    request<AboutData>(`${endpoints.about}${refresh ? "?refresh=1" : ""}`),
  site: () =>
    request<PublicSite>(endpoints.publicBootstrap, { skipAuthRedirect: true }),
  session: () =>
    request<AuthSession>(endpoints.identitySession, {
      skipAuthRedirect: true,
      skipCsrf: true,
    }),
  challenge: () =>
    request<CaptchaChallenge>(endpoints.identityChallenge, {
      skipAuthRedirect: true,
    }),
  setup: (username: string, password: string) =>
    request<AuthSession>(endpoints.identityBootstrap, {
      method: "POST",
      skipAuthRedirect: true,
      skipCsrf: true,
      body: JSON.stringify({ username, password }),
    }),
  login: (body: {
    username: string;
    password: string;
    captchaID?: string;
    captchaAnswer?: string;
  }) =>
    request<AuthSession>(endpoints.identitySession, {
      method: "POST",
      skipAuthRedirect: true,
      skipCsrf: true,
      body: JSON.stringify(body),
    }),
  logout: () => request<void>(endpoints.identitySession, { method: "DELETE" }),
  overview: () => request<Dashboard>(endpoints.overview),
  preferences: () => request<Record<string, string>>(endpoints.preferences),
  savePreferences: (value: Record<string, string>) =>
    request<Record<string, string>>(endpoints.preferences, {
      method: "PUT",
      body: JSON.stringify(value),
    }),
  themeSettings: () => request<ThemeSettings>(endpoints.theme),
  saveThemeSettings: (value: ThemeSettings) =>
    request<ThemeSettings>(endpoints.theme, {
      method: "PUT",
      body: JSON.stringify(value),
    }),
  announcements: () => request<AnnouncementEntry[]>(endpoints.announcements),
  saveAnnouncement: (value: AnnouncementInput, id?: number) =>
    request<AnnouncementEntry>(
      id ? `${endpoints.announcements}/${id}` : endpoints.announcements,
      {
        method: id ? "PATCH" : "POST",
        body: JSON.stringify(value),
      },
    ),
  deleteAnnouncement: (id: number) =>
    request<void>(`${endpoints.announcements}/${id}`, { method: "DELETE" }),
  storageNodes: () => request<StorageSource[]>(endpoints.storageNodes),
  saveStorageNode: (input: StorageSourceInput, id?: number) =>
    request<StorageSource>(
      id ? `${endpoints.storageNodes}/${id}` : endpoints.storageNodes,
      {
        method: id ? "PATCH" : "POST",
        body: JSON.stringify(input),
      },
    ),
  deleteStorageNode: (id: number) =>
    request<void>(`${endpoints.storageNodes}/${id}`, { method: "DELETE" }),
  entries: (storageKey: string, path = "") =>
    request<FileEntry[]>(`${endpoints.entries}?${query({ storageKey, path })}`),
  entryDetails: (storageKey: string, path: string) =>
    request<EntryDetails>(
      `${endpoints.entryDetails}?${query({ storageKey, path })}`,
    ),
  publicEntries: (storageKey: string, path = "", directoryPassword = "") =>
    request<FileEntry[]>(
      `/api/v1/public/drives/${encodeURIComponent(storageKey)}/entries?${query({ path, directoryPassword })}`,
      { skipAuthRedirect: true },
    ),
  publicEntryDetails: (
    storageKey: string,
    path: string,
    directoryPassword = "",
  ) =>
    request<EntryDetails>(
      `/api/v1/public/drives/${encodeURIComponent(storageKey)}/entries/details?${query({ path, directoryPassword })}`,
      { skipAuthRedirect: true },
    ),
  search: (storageKey: string, q: string) =>
    request<FileEntry[]>(`${endpoints.search}?${query({ storageKey, q })}`),
  createFolder: (storageKey: string, path: string) =>
    request<FileEntry>(endpoints.folders, {
      method: "POST",
      body: JSON.stringify({ storageKey, path }),
    }),
  createDocument: (storageKey: string, path: string) =>
    request<FileEntry>(endpoints.documents, {
      method: "POST",
      body: JSON.stringify({ storageKey, path }),
    }),
  saveText: (storageKey: string, path: string, content: string) =>
    request<FileEntry>(endpoints.textContent, {
      method: "PUT",
      body: JSON.stringify({ storageKey, path, content }),
    }),
  saveMetadata: (storageKey: string, path: string, description: string) =>
    request<FileEntry>(endpoints.metadata, {
      method: "PUT",
      body: JSON.stringify({ storageKey, path, description }),
    }),
  moveEntry: (storageKey: string, from: string, to: string) =>
    request<FileEntry>(endpoints.entries, {
      method: "PATCH",
      body: JSON.stringify({ storageKey, from, to }),
    }),
  batchMove: (storageKey: string, paths: string[], targetDir: string) =>
    request<FileEntry[]>(endpoints.move, {
      method: "POST",
      body: JSON.stringify({ storageKey, paths, targetDir }),
    }),
  batchCopy: (storageKey: string, paths: string[], targetDir: string) =>
    request<FileEntry[]>(endpoints.copy, {
      method: "POST",
      body: JSON.stringify({ storageKey, paths, targetDir }),
    }),
  deleteEntry: (storageKey: string, path: string) =>
    request<void>(`${endpoints.entries}?${query({ storageKey, path })}`, {
      method: "DELETE",
    }),
  shares: () => request<ShareEntry[]>(endpoints.shares),
  createShare: (value: {
    storageKey: string;
    path: string;
    password?: string;
    expiresAt?: string;
    maxAccessCount?: number;
    customKey?: string;
  }) =>
    request<ShareEntry>(endpoints.shares, {
      method: "POST",
      body: JSON.stringify(value),
    }),
  batchShares: (
    storageKey: string,
    paths: string[],
    password = "",
    expiresAt = "",
    maxAccessCount = 0,
  ) =>
    request<ShareEntry>(`${endpoints.shares}/batch`, {
      method: "POST",
      body: JSON.stringify({
        storageKey,
        paths,
        password,
        expiresAt,
        maxAccessCount,
      }),
    }),
  deleteShare: (id: number) =>
    request<void>(`${endpoints.shares}/${id}`, { method: "DELETE" }),
  updateShareLimits: (
    id: number,
    value: { expiresAt?: string; maxAccessCount: number },
  ) =>
    request<{ expiresAt?: string; maxAccessCount: number }>(
      `${endpoints.shares}/${id}`,
      { method: "PATCH", body: JSON.stringify(value) },
    ),
  deleteExpiredShares: () =>
    request<{ deleted: number }>(`${endpoints.shares}/expired`, {
      method: "DELETE",
    }),
  share: (token: string, password = "", path = "") =>
    request<ShareDetail>(
      `/api/v1/public/shares/${encodeURIComponent(token)}?${query({ password, path })}`,
      { skipAuthRedirect: true },
    ),
  deliveryLinks: () => request<DirectLinkEntry[]>(endpoints.deliveryLinks),
  createDeliveryLink: (storageKey: string, path: string) =>
    request<DirectLinkEntry>(endpoints.deliveryLinks, {
      method: "POST",
      body: JSON.stringify({ storageKey, path }),
    }),
  batchDeliveryLinks: (storageKey: string, paths: string[]) =>
    request<DirectLinkEntry[]>(`${endpoints.deliveryLinks}/batch`, {
      method: "POST",
      body: JSON.stringify({ storageKey, paths }),
    }),
  updateDeliveryLink: (id: number, enabled: boolean) =>
    request<{ enabled: boolean }>(`${endpoints.deliveryLinks}/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ enabled }),
    }),
  deleteDeliveryLink: (id: number) =>
    request<void>(`${endpoints.deliveryLinks}/${id}`, { method: "DELETE" }),
  favorites: () => request<FavoriteEntry[]>(endpoints.favorites),
  createFavorite: (storageKey: string, path: string) =>
    request<FavoriteEntry>(endpoints.favorites, {
      method: "POST",
      body: JSON.stringify({ storageKey, path }),
    }),
  deleteFavorite: (id: number) =>
    request<void>(`${endpoints.favorites}/${id}`, { method: "DELETE" }),
  insights: () => request<LinkAnalytics>(endpoints.insights),
  auditEvents: (filters: Record<string, string | number | undefined>) =>
    request<AccessLogPage>(`${endpoints.audit}?${query(filters)}`),
  deleteAuditEvents: (olderThanDays?: number, all?: boolean) =>
    request<{ deleted: number }>(
      `${endpoints.audit}?${query({ olderThanDays, all })}`,
      { method: "DELETE" },
    ),
  accounts: () => request<UserEntry[]>(endpoints.accounts),
  saveAccount: (input: AccountInput, id?: number) =>
    request<UserEntry>(
      id ? `${endpoints.accounts}/${id}` : endpoints.accounts,
      { method: id ? "PATCH" : "POST", body: JSON.stringify(input) },
    ),
  deleteAccount: (id: number) =>
    request<void>(`${endpoints.accounts}/${id}`, { method: "DELETE" }),
  accountSessions: (id: number) =>
    request<SessionEntry[]>(`${endpoints.accounts}/${id}/sessions`),
  revokeAccountSessions: (id: number) =>
    request<{ revoked: number }>(`${endpoints.accounts}/${id}/sessions`, {
      method: "DELETE",
    }),
  revokeAccountSession: (id: number, sessionID: number) =>
    request<void>(`${endpoints.accounts}/${id}/sessions/${sessionID}`, {
      method: "DELETE",
    }),
};

export function query(
  values: Record<string, string | number | boolean | undefined>,
) {
  const params = new URLSearchParams();
  Object.entries(values).forEach(([key, value]) => {
    if (value !== undefined && value !== "") params.set(key, String(value));
  });
  return params.toString();
}

export function contentUrl(storageKey: string, path: string, preview = false) {
  return `${endpoints.content}?${query({ storageKey, path, preview })}`;
}

export function publicContentUrl(
  storageKey: string,
  path: string,
  directoryPassword = "",
  preview = false,
) {
  return `/api/v1/public/drives/${encodeURIComponent(storageKey)}/content?${query({ path, directoryPassword, preview })}`;
}

export function shareContentUrl(token: string, password = "", path = "") {
  return `/api/v1/public/shares/${encodeURIComponent(token)}/content?${query({ password, path })}`;
}

export async function upload(
  storageKey: string,
  path: string,
  file: File,
  onProgress?: (percent: number) => void,
) {
  const body = new FormData();
  body.set("storageKey", storageKey);
  body.set("path", path);
  body.set("file", file);
  onProgress?.(5);
  const result = await request<{ path: string }>(endpoints.uploads, {
    method: "POST",
    body,
  });
  onProgress?.(100);
  return result;
}

export async function downloadArchive(storageKey: string, paths: string[]) {
  const token = await ensureCsrf();
  const response = await fetch(endpoints.archives, {
    method: "POST",
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { [csrfHeader]: token } : {}),
    },
    body: JSON.stringify({ storageKey, paths }),
  });
  if (!response.ok) throw new Error(await parseError(response));
  const blob = await response.blob();
  saveBlob(blob, archiveName(paths));
}

export async function downloadPublicArchive(
  storageKey: string,
  paths: string[],
  directoryPassword = "",
) {
  const response = await fetch(
    `/api/v1/public/drives/${encodeURIComponent(storageKey)}/archives`,
    {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ paths, directoryPassword }),
    },
  );
  if (!response.ok) throw new Error(await parseError(response));
  saveBlob(await response.blob(), archiveName(paths));
}

function saveBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = name;
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

function archiveName(paths: string[]) {
  if (paths.length === 1) return `${paths[0].split("/").pop() || "xfile"}.zip`;
  return `xfile-${new Date().toISOString().slice(0, 10)}.zip`;
}

export function formatBytes(bytes: number) {
  if (!bytes) return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let index = 0;
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }
  return `${value.toFixed(value >= 10 ? 0 : 1)} ${units[index]}`;
}

export function formatTime(value?: string) {
  if (!value) return "—";
  const normalized = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(value)
    ? `${value.replace(" ", "T")}Z`
    : value;
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(normalized));
}

export function timeValue(value?: string) {
  if (!value) return Number.NaN;
  const normalized = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(value)
    ? `${value.replace(" ", "T")}Z`
    : value;
  return new Date(normalized).getTime();
}
