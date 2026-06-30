export interface FileEntry {
  name: string
  path: string
  type: 'file' | 'folder'
  size: number
  modifiedAt: string
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

export async function api<T>(url: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  if (options.body && !(options.body instanceof FormData) && !headers.has('Content-Type'))
    headers.set('Content-Type', 'application/json')

  const res = await fetch(url, { ...options, headers, credentials: 'same-origin' })
  if (res.status === 401 && !location.pathname.startsWith('/login') && !location.pathname.startsWith('/s/')) {
    location.href = `/login?redirect=${encodeURIComponent(location.pathname)}`
    throw new Error('请先登录')
  }
  if (!res.ok)
    throw new Error(await res.text() || res.statusText)
  if (res.status === 204)
    return undefined as T
  return res.json() as Promise<T>
}

export function fileUrl(path: string) {
  return `/api/files/download?path=${encodeURIComponent(path)}`
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
