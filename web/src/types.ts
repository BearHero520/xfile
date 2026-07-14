export type FileKind = "file" | "folder";

export interface FileEntry {
  name: string;
  path: string;
  type: FileKind;
  size: number;
  modifiedAt: string;
  description?: string;
  metadataUpdatedAt?: string;
}

export interface StorageSource {
  id: number;
  name: string;
  key: string;
  type: string;
  typeLabel: string;
  rootPath?: string;
  hiddenPaths?: string;
  blockedPaths?: string;
  public: boolean;
  enabled: boolean;
  orderNum: number;
  createdAt: string;
}

export interface PublicSite {
  siteName: string;
  rootName: string;
  initialized: boolean;
  loggedIn: boolean;
  sources: StorageSource[];
  preferences: Record<string, string>;
}

export interface UserEntry {
  id: number;
  username: string;
  role: string;
  enabled: boolean;
  storageSourceKeys?: string[];
  storageSourceRoots?: Record<string, string[]>;
  disabledOperations?: string[];
  activeSessionCount: number;
  createdAt: string;
}

export interface SessionEntry {
  id: number;
  userId: number;
  username: string;
  ip: string;
  userAgent: string;
  current: boolean;
  createdAt: string;
  lastSeenAt: string;
  expiresAt: string;
  revokedAt?: string;
}

export interface AuthSession {
  initialized: boolean;
  authenticated: boolean;
  captchaRequired?: boolean;
  username: string;
  sessionSeconds: number;
  csrfToken?: string;
  user?: UserEntry;
  session?: SessionEntry;
}

export interface CaptchaChallenge {
  required: boolean;
  id?: string;
  question?: string;
}

export interface ShareEntry {
  id: number;
  token: string;
  storageKey: string;
  path: string;
  paths?: string[];
  itemCount: number;
  url: string;
  protected: boolean;
  expiresAt?: string;
  viewCount: number;
  downloadCount: number;
  lastAccessAt?: string;
  createdAt: string;
}

export interface ShareDetail {
  token: string;
  storageKey: string;
  path: string;
  currentPath?: string;
  name: string;
  type: FileKind;
  size: number;
  description?: string;
  protected: boolean;
  expiresAt?: string;
  createdAt: string;
  files?: FileEntry[];
  itemCount: number;
}

export interface FavoriteEntry {
  id: number;
  userId: number;
  storageKey: string;
  path: string;
  createdAt: string;
}

export interface DirectLinkEntry {
  id: number;
  token: string;
  storageKey: string;
  path: string;
  url: string;
  enabled: boolean;
  accessCount: number;
  lastAccessAt?: string;
  createdAt: string;
}

export interface AccessLog {
  id: number;
  action: string;
  path: string;
  ip: string;
  userAgent: string;
  createdAt: string;
}

export interface AccessLogPage {
  items: AccessLog[];
  total: number;
  page: number;
  pageSize: number;
}

export interface PathMetric {
  path: string;
  count: number;
  lastAccessAt?: string;
}

export interface LinkAnalytics {
  shareVisits: AccessLog[];
  downloadRanking: PathMetric[];
  directLinkAccesses: AccessLog[];
}

export interface Dashboard {
  siteName: string;
  storageRoot: string;
  fileCount: number;
  folderCount: number;
  totalBytes: number;
  shareCount: number;
  recentFiles: FileEntry[];
  recentLogs: AccessLog[];
  storageSources: string[];
}

export interface StorageSourceInput {
  name: string;
  key: string;
  type: string;
  rootPath: string;
  hiddenPaths: string;
  blockedPaths: string;
  public: boolean;
  enabled: boolean;
  orderNum: number;
}

export interface AccountInput {
  username: string;
  password: string;
  role: string;
  enabled: boolean;
  storageSourceKeys: string[];
  storageSourceRoots: Record<string, string[]>;
  disabledOperations: string[];
}
