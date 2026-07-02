export type ExternalPreviewProvider = 'disabled' | 'kkfileview' | 'onlyoffice'
export type OnlyOfficeDocumentType = 'word' | 'cell' | 'slide'

export interface ExternalPreviewFile {
  name: string
  ext: string
  url: string
  key?: string
}

const onlyOfficeDocumentTypes: Record<string, OnlyOfficeDocumentType> = {
  doc: 'word',
  docm: 'word',
  docx: 'word',
  dot: 'word',
  dotm: 'word',
  dotx: 'word',
  odt: 'word',
  rtf: 'word',
  txt: 'word',
  xls: 'cell',
  xlsm: 'cell',
  xlsx: 'cell',
  xlt: 'cell',
  xltm: 'cell',
  xltx: 'cell',
  ods: 'cell',
  csv: 'cell',
  odp: 'slide',
  pot: 'slide',
  potm: 'slide',
  potx: 'slide',
  pps: 'slide',
  ppsm: 'slide',
  ppsx: 'slide',
  ppt: 'slide',
  pptm: 'slide',
  pptx: 'slide',
}

const kkFileViewExtensions = new Set([
  ...Object.keys(onlyOfficeDocumentTypes),
  'bmp',
  'gif',
  'jpeg',
  'jpg',
  'pdf',
  'png',
  'svg',
  'tif',
  'tiff',
  'webp',
  'css',
  'html',
  'ini',
  'java',
  'js',
  'json',
  'log',
  'md',
  'php',
  'sql',
  'ts',
  'vue',
  'xml',
  'yaml',
  'yml',
  '7z',
  'rar',
  'tar',
  'zip',
])

export function normalizeExternalPreviewProvider(value?: string): ExternalPreviewProvider {
  const provider = value?.trim()
  if (provider === 'kkfileview' || provider === 'onlyoffice')
    return provider
  return 'disabled'
}

export function normalizeExternalPreviewBaseUrl(value?: string) {
  return (value || '').trim().replace(/\/+$/, '')
}

export function onlyOfficeDocumentType(ext?: string): OnlyOfficeDocumentType | undefined {
  return onlyOfficeDocumentTypes[(ext || '').toLowerCase()]
}

export function supportsExternalPreview(settings: Record<string, string>, ext: string) {
  const provider = normalizeExternalPreviewProvider(settings.externalPreviewProvider)
  if (provider === 'disabled')
    return false
  const normalizedExt = ext.toLowerCase()
  if (provider === 'onlyoffice')
    return Boolean(onlyOfficeDocumentType(normalizedExt))
  return kkFileViewExtensions.has(normalizedExt)
}

export function buildExternalPreviewUrl(settings: Record<string, string>, file: ExternalPreviewFile) {
  const provider = normalizeExternalPreviewProvider(settings.externalPreviewProvider)
  if (provider === 'disabled' || !supportsExternalPreview(settings, file.ext))
    return ''

  const server = normalizeExternalPreviewBaseUrl(settings.externalPreviewBaseUrl)
  const template = (settings.externalPreviewTemplate || '').trim()
  if (template)
    return applyExternalPreviewTemplate(template, server, file)

  if (!server)
    return ''
  if (provider === 'kkfileview')
    return `${server}/onlinePreview?url=${encodeURIComponent(base64Encode(file.url))}`
  if (!onlyOfficeDocumentType(file.ext))
    return ''

  const params = new URLSearchParams({
    url: file.url,
    name: file.name,
    ext: file.ext,
    key: file.key || stablePreviewKey(`${file.name}:${file.url}`),
  })
  return `/external-preview?${params.toString()}`
}

export function stablePreviewKey(value: string) {
  let hash = 2166136261
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index)
    hash = Math.imul(hash, 16777619)
  }
  return `xfile-${(hash >>> 0).toString(36)}`
}

function applyExternalPreviewTemplate(template: string, server: string, file: ExternalPreviewFile) {
  const key = file.key || stablePreviewKey(`${file.name}:${file.url}`)
  const values: Record<string, string> = {
    '{server}': server,
    '{url}': file.url,
    '{encodedUrl}': encodeURIComponent(file.url),
    '{base64Url}': encodeURIComponent(base64Encode(file.url)),
    '{name}': file.name,
    '{encodedName}': encodeURIComponent(file.name),
    '{ext}': file.ext,
    '{key}': key,
    '{encodedKey}': encodeURIComponent(key),
  }
  return Object.entries(values).reduce((result, [token, value]) => result.split(token).join(value), template)
}

function base64Encode(value: string) {
  const bytes = new TextEncoder().encode(value)
  let binary = ''
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte)
  })
  return btoa(binary)
}
