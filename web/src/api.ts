export interface FileEntry {
  name: string
  path: string
  type: 'file' | 'folder'
  size: number
  modifiedAt: string
  description?: string
  metadataUpdatedAt?: string
}

export interface StorageSource {
  id: number
  name: string
  key: string
  type: string
  typeLabel: string
  rootPath?: string
  hiddenPaths?: string
  blockedPaths?: string
  public: boolean
  enabled: boolean
  orderNum: number
  createdAt: string
}

export interface PublicSite {
  siteName: string
  rootName: string
  initialized: boolean
  loggedIn: boolean
  sources: StorageSource[]
}

export interface AuthMe {
  initialized: boolean
  authenticated: boolean
  captchaRequired?: boolean
  username: string
  sessionSeconds: number
  csrfToken?: string
  user?: UserEntry
  session?: SessionEntry
}

export interface ShareEntry {
  id: number
  token: string
  path: string
  url: string
  protected: boolean
  expiresAt?: string
  viewCount: number
  downloadCount: number
  lastAccessAt?: string
  createdAt: string
}

export interface ShareDetail {
  token: string
  path: string
  currentPath?: string
  name: string
  type: 'file' | 'folder'
  size: number
  description?: string
  protected: boolean
  expiresAt?: string
  createdAt: string
  files?: FileEntry[]
}

export interface DirectLinkEntry {
  id: number
  token: string
  path: string
  url: string
  enabled: boolean
  accessCount: number
  lastAccessAt?: string
  createdAt: string
}

export interface AccessLog {
  id: number
  action: string
  path: string
  ip: string
  userAgent: string
  createdAt: string
}

export interface AccessLogPage {
  items: AccessLog[]
  total: number
  page: number
  pageSize: number
}

export interface PathMetric {
  path: string
  count: number
  lastAccessAt?: string
}

export interface LinkAnalytics {
  shareVisits: AccessLog[]
  downloadRanking: PathMetric[]
  directLinkAccesses: AccessLog[]
}

export interface UserEntry {
  id: number
  username: string
  role: string
  enabled: boolean
  storageSourceKeys?: string[]
  storageSourceRoots?: Record<string, string[]>
  disabledOperations?: string[]
  activeSessionCount: number
  createdAt: string
}

export interface SessionEntry {
  id: number
  userId: number
  username: string
  ip: string
  userAgent: string
  current: boolean
  createdAt: string
  lastSeenAt: string
  expiresAt: string
  revokedAt?: string
}

export interface Dashboard {
  siteName: string
  storageRoot: string
  fileCount: number
  folderCount: number
  totalBytes: number
  shareCount: number
  recentFiles: FileEntry[]
  recentLogs: AccessLog[]
  storageSources: string[]
}

type ApiOptions = RequestInit & { skipAuthRedirect?: boolean, skipCsrf?: boolean }

const csrfHeaderName = 'X-CSRF-Token'
let csrfToken = ''
let csrfTokenRequest: Promise<string> | undefined

function rememberCsrfToken(value: unknown) {
  if (value && typeof value === 'object' && 'csrfToken' in value) {
    const token = (value as { csrfToken?: unknown }).csrfToken
    csrfToken = typeof token === 'string' ? token : ''
  }
}

function requestMethod(options: RequestInit) {
  return (options.method || 'GET').toUpperCase()
}

function isMutatingMethod(method: string) {
  return !['GET', 'HEAD', 'OPTIONS'].includes(method)
}

function skipsCsrf(url: string, options: ApiOptions) {
  return options.skipCsrf || url === '/api/auth/login' || url === '/api/auth/setup'
}

async function ensureCsrfToken() {
  if (csrfToken)
    return csrfToken
  if (!csrfTokenRequest) {
    csrfTokenRequest = fetch('/api/auth/me', { credentials: 'same-origin' })
      .then(async (res) => {
        if (!res.ok)
          return ''
        const data = await res.json()
        rememberCsrfToken(data)
        return csrfToken
      })
      .finally(() => {
        csrfTokenRequest = undefined
      })
  }
  return csrfTokenRequest
}

export async function csrfHeaders(): Promise<Record<string, string>> {
  const token = await ensureCsrfToken()
  return token ? { [csrfHeaderName]: token } : {}
}

export async function api<T>(url: string, options: ApiOptions = {}): Promise<T> {
  const headers = new Headers(options.headers)
  if (options.body && !(options.body instanceof FormData) && !headers.has('Content-Type'))
    headers.set('Content-Type', 'application/json')

  const method = requestMethod(options)
  if (isMutatingMethod(method) && !skipsCsrf(url, options)) {
    const token = await ensureCsrfToken()
    if (token)
      headers.set(csrfHeaderName, token)
  }

  const { skipAuthRedirect } = options
  const fetchOptions: ApiOptions = { ...options }
  delete fetchOptions.skipAuthRedirect
  delete fetchOptions.skipCsrf
  const res = await fetch(url, { ...fetchOptions, headers, credentials: 'same-origin' })
  if (!skipAuthRedirect && res.status === 401 && !location.pathname.startsWith('/login') && !location.pathname.startsWith('/s/')) {
    location.href = `/login?redirect=${encodeURIComponent(location.pathname)}`
    throw new Error('请先登录')
  }
  if (!res.ok)
    throw new Error(await res.text() || res.statusText)
  if (res.status === 204)
    return undefined as T
  const data = await res.json()
  rememberCsrfToken(data)
  return data as T
}

export function fileUrl(path: string, storageKey = 'local', preview = false) {
  const params = new URLSearchParams({ path })
  if (storageKey)
    params.set('storageKey', storageKey)
  if (preview)
    params.set('preview', 'true')
  return `/api/files/download?${params.toString()}`
}

export function publicFileUrl(storageKey: string, path: string, directoryPassword = '', preview = false) {
  const params = new URLSearchParams({ path })
  if (directoryPassword)
    params.set('directoryPassword', directoryPassword)
  if (preview)
    params.set('preview', 'true')
  return `/api/public/storage/${encodeURIComponent(storageKey)}/download?${params.toString()}`
}

export function shareDownloadUrl(token: string, password: string, path = '') {
  const params = new URLSearchParams()
  if (password)
    params.set('password', password)
  if (path)
    params.set('path', path)
  const query = params.toString()
  return `/api/public/shares/${encodeURIComponent(token)}/download${query ? `?${query}` : ''}`
}

export function formatBytes(bytes: number) {
  if (!bytes)
    return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit++
  }
  return `${value.toFixed(value >= 10 ? 0 : 1)} ${units[unit]}`
}

export function formatTime(value: string) {
  if (!value)
    return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}
