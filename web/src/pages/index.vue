<script setup lang="ts">
import type { AccessLog, AccessLogPage, Dashboard, DirectLinkEntry, FileEntry, PublicSite, ShareEntry, StorageSource } from '~/api'
import type { OfficePreviewKind } from '~/components/OfficePreview.vue'
import {
  Clock,
  CopyDocument,
  DataAnalysis,
  Delete,
  Document,
  Download,
  Edit,
  Folder,
  Link,
  Picture,
  Plus,
  Search,
  Share,
  Upload,
  VideoPlay,
  View,
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api, csrfHeaders, fileUrl, formatBytes, formatTime, publicFileUrl } from '~/api'
import OfficePreview from '~/components/OfficePreview.vue'
import VideoPlayer from '~/components/VideoPlayer.vue'
import { buildExternalPreviewUrl, stablePreviewKey, supportsExternalPreview } from '~/externalPreview'

type PreviewKind = 'image' | 'video' | 'audio' | 'pdf' | 'text' | 'office' | 'external' | 'unsupported'
type FileViewMode = 'details' | 'large' | 'medium' | 'small'
type TargetPickerMode = 'move' | 'copy'

const route = useRoute()

const emptyDashboard: Dashboard = {
  siteName: 'XFile',
  storageRoot: 'data/files',
  fileCount: 0,
  folderCount: 0,
  totalBytes: 0,
  shareCount: 0,
  recentFiles: [],
  recentLogs: [],
  storageSources: [],
}

const emptySite: PublicSite = {
  siteName: 'XFile',
  rootName: '首页',
  initialized: false,
  loggedIn: false,
  sources: [],
}

const site = ref<PublicSite>(emptySite)
const dashboard = ref<Dashboard>(emptyDashboard)
const files = ref<FileEntry[]>([])
const shares = ref<ShareEntry[]>([])
const logs = ref<AccessLog[]>([])
const settings = ref<Record<string, string>>({})
const loading = ref(false)
const searching = ref(false)
const dragUploadActive = ref(false)
const activePath = ref('')
const activeSourceKey = ref('')
const directoryPassword = ref('')
const keyword = ref('')
const searchMode = ref<'current' | 'global'>('current')
const globalResults = ref<FileEntry[]>([])
const fileViewMode = ref<FileViewMode>('details')
const uploader = ref<HTMLInputElement>()
const previewVisible = ref(false)
const previewFile = ref<FileEntry>()
const previewKind = ref<PreviewKind>('unsupported')
const previewText = ref('')
const previewOriginalText = ref('')
const previewLoading = ref(false)
const previewTextSaving = ref(false)
const previewTextTooLarge = ref(false)
const previewTextError = ref(false)
const metadataVisible = ref(false)
const metadataFile = ref<FileEntry>()
const metadataDescription = ref('')
const metadataSaving = ref(false)
const selectedFiles = ref<FileEntry[]>([])
const targetPickerVisible = ref(false)
const targetPickerLoading = ref(false)
const targetPickerSaving = ref(false)
const targetPickerMode = ref<TargetPickerMode>('move')
const targetPickerItems = ref<FileEntry[]>([])
const targetPickerPath = ref('')
const targetPickerManualPath = ref('')
const targetPickerDirectories = ref<FileEntry[]>([])
const contextMenu = ref({
  visible: false,
  x: 0,
  y: 0,
  file: undefined as FileEntry | undefined,
  directory: false,
})

const isLoggedIn = computed(() => site.value.loggedIn)
const publicSources = computed(() => site.value.sources.filter(source => source.enabled && (isLoggedIn.value || source.public)))
const selectableSources = computed(() => isLoggedIn.value ? site.value.sources : publicSources.value)
const activeSource = computed<StorageSource | undefined>(() => selectableSources.value.find(source => source.key === activeSourceKey.value) || selectableSources.value[0])
const selectedCount = computed(() => selectedFiles.value.length)
const currentStorageSource = computed(() => activeSource.value ? `${activeSource.value.name} / ${activeSource.value.typeLabel}` : '暂无存储源')
const disabledOperations = computed(() => new Set((settings.value.disabledOperations || '').split(/[\s,;]+/).filter(Boolean)))
const previewTextDirty = computed(() => previewKind.value === 'text' && previewText.value !== previewOriginalText.value)
const previewTextEditable = computed(() => isLoggedIn.value && previewKind.value === 'text' && previewFile.value?.type === 'file' && operationAllowed('upload') && !previewLoading.value && !previewTextTooLarge.value && !previewTextError.value)
const targetPickerTitle = computed(() => targetPickerMode.value === 'move' ? '选择移动目标' : '选择复制目标')
const targetPickerConfirmLabel = computed(() => targetPickerMode.value === 'move' ? '移动到此处' : '复制到此处')
const targetPickerActionLabel = computed(() => targetPickerMode.value === 'move' ? '移动' : '复制')
const targetPickerItemSummary = computed(() => {
  const count = targetPickerItems.value.length
  if (!count)
    return '未选择项目'
  if (count === 1)
    return targetPickerItems.value[0].name
  return `${count} 项`
})
const targetPickerBreadcrumbs = computed(() => {
  const rootLabel = site.value.rootName || settings.value.rootName || '首页'
  const parts = targetPickerPath.value ? targetPickerPath.value.split('/') : []
  return [{ label: rootLabel, path: '' }].concat(
    parts.map((part, index) => ({ label: part, path: parts.slice(0, index + 1).join('/') })),
  )
})

const textEditLimitBytes = 2 * 1024 * 1024
const metadataDescriptionLimit = 2000

const searchModeOptions = [
  { label: '当前目录', value: 'current' },
  { label: '全局', value: 'global' },
]

const fileViewModeOptions = [
  { label: '详情', value: 'details' },
  { label: '大图', value: 'large' },
  { label: '中图', value: 'medium' },
  { label: '小图', value: 'small' },
]

const displayedFiles = computed(() => {
  const term = keyword.value.trim().toLowerCase()
  if (isLoggedIn.value && searchMode.value === 'global')
    return term ? globalResults.value : files.value
  if (!term)
    return files.value
  return files.value.filter(file => `${file.name} ${file.path} ${file.description || ''}`.toLowerCase().includes(term))
})

const fileEmptyText = computed(() => {
  if (!activeSource.value)
    return isLoggedIn.value ? '暂无存储源，请先在存储源管理中新建' : '暂无公开存储源'
  if (!activeSource.value.enabled)
    return '当前存储源已停用'
  if (searchMode.value === 'global' && keyword.value.trim())
    return '未找到匹配文件'
  return '当前目录暂无文件'
})

const imagePreviewUrls = computed(() =>
  displayedFiles.value
    .filter(file => isImageFile(file))
    .map(file => currentFileUrl(file.path, true)),
)

const breadcrumbs = computed(() => {
  const rootLabel = site.value.rootName || settings.value.rootName || '首页'
  const parts = activePath.value ? activePath.value.split('/') : []
  return [{ label: rootLabel, path: '' }].concat(
    parts.map((part, index) => ({ label: part, path: parts.slice(0, index + 1).join('/') })),
  )
})

function setSource(key: string) {
  activeSourceKey.value = key
  activePath.value = ''
  directoryPassword.value = ''
  keyword.value = ''
  searchMode.value = 'current'
  globalResults.value = []
  void loadFiles()
}

async function loadSite() {
  site.value = await api<PublicSite>('/api/public/site', { skipAuthRedirect: true } as RequestInit)
  if (!selectableSources.value.some(source => source.key === activeSourceKey.value))
    activeSourceKey.value = selectableSources.value[0]?.key || ''
}

async function loadFiles() {
  if (!activeSource.value) {
    files.value = []
    return
  }
  if (!activeSource.value.enabled) {
    files.value = []
    selectedFiles.value = []
    return
  }
  if (isLoggedIn.value) {
    const params = new URLSearchParams({ storageKey: activeSource.value.key, path: activePath.value })
    files.value = await api<FileEntry[]>(`/api/files?${params.toString()}`)
  }
  else {
    const params = new URLSearchParams({ path: activePath.value })
    if (directoryPassword.value)
      params.set('directoryPassword', directoryPassword.value)
    try {
      files.value = await api<FileEntry[]>(`/api/public/storage/${encodeURIComponent(activeSource.value.key)}/files?${params.toString()}`, { skipAuthRedirect: true } as RequestInit)
    }
    catch (error) {
      if (!(error instanceof Error) || !error.message.includes('directory password is required'))
        throw error
      const { value } = await ElMessageBox.prompt('请输入目录访问密码', '受保护目录', {
        inputType: 'password',
        inputPlaceholder: '目录密码',
      })
      directoryPassword.value = value
      const retryParams = new URLSearchParams({ path: activePath.value, directoryPassword: directoryPassword.value })
      files.value = await api<FileEntry[]>(`/api/public/storage/${encodeURIComponent(activeSource.value.key)}/files?${retryParams.toString()}`, { skipAuthRedirect: true } as RequestInit)
    }
  }
  selectedFiles.value = []
}

function normalizeTargetPath(value: string) {
  return value.replace(/\\/g, '/').trim().replace(/^\/+|\/+$/g, '')
}

function hasUnsafeTargetSegment(value: string) {
  return normalizeTargetPath(value).split('/').includes('..')
}

function targetPickerParentPath() {
  const parts = targetPickerPath.value.split('/').filter(Boolean)
  parts.pop()
  return parts.join('/')
}

function targetDirectoryDisabled(directory: FileEntry) {
  if (targetPickerMode.value !== 'move')
    return false
  return targetPickerItems.value.some((item) => {
    if (item.type !== 'folder')
      return false
    return directory.path === item.path || directory.path.startsWith(`${item.path}/`)
  })
}

async function loadTargetPickerDirectories() {
  if (!activeSource.value) {
    targetPickerDirectories.value = []
    return
  }
  targetPickerLoading.value = true
  try {
    const params = new URLSearchParams({ storageKey: activeSource.value.key, path: targetPickerPath.value })
    const entries = await api<FileEntry[]>(`/api/files?${params.toString()}`)
    targetPickerDirectories.value = entries.filter(entry => entry.type === 'folder')
  }
  catch (error) {
    targetPickerDirectories.value = []
    ElMessage.error(error instanceof Error ? error.message : '目录加载失败')
  }
  finally {
    targetPickerLoading.value = false
  }
}

function setTargetPickerPath(path: string) {
  const nextPath = normalizeTargetPath(path)
  if (hasUnsafeTargetSegment(nextPath)) {
    ElMessage.warning('目标目录不能包含 ..')
    return
  }
  targetPickerPath.value = nextPath
  targetPickerManualPath.value = nextPath
  void loadTargetPickerDirectories()
}

function openTargetPicker(mode: TargetPickerMode, items: FileEntry[]) {
  if (!items.length)
    return
  targetPickerMode.value = mode
  targetPickerItems.value = [...items]
  targetPickerVisible.value = true
  setTargetPickerPath(activePath.value)
}

async function confirmTargetPicker() {
  if (!targetPickerItems.value.length)
    return
  const targetDir = normalizeTargetPath(targetPickerManualPath.value)
  if (hasUnsafeTargetSegment(targetDir)) {
    ElMessage.warning('目标目录不能包含 ..')
    return
  }
  targetPickerSaving.value = true
  try {
    await api(targetPickerMode.value === 'move' ? '/api/files/batch/move' : '/api/files/batch/copy', {
      method: 'PATCH',
      body: JSON.stringify({
        storageKey: activeSource.value?.key || 'local',
        paths: targetPickerItems.value.map(file => file.path),
        targetDir,
      }),
    })
    const count = targetPickerItems.value.length
    targetPickerVisible.value = false
    targetPickerItems.value = []
    selectedFiles.value = []
    ElMessage.success(`已${targetPickerActionLabel.value} ${count} 项`)
    await loadFiles()
  }
  catch (error) {
    ElMessage.error(error instanceof Error ? error.message : `${targetPickerActionLabel.value}失败`)
  }
  finally {
    targetPickerSaving.value = false
  }
}

async function loadAdminData() {
  if (!isLoggedIn.value)
    return
  const [dash, shareList, logList, settingMap] = await Promise.all([
    api<Dashboard>('/api/dashboard'),
    api<ShareEntry[]>('/api/shares'),
    api<AccessLogPage>('/api/logs?pageSize=5'),
    api<Record<string, string>>('/api/settings'),
  ])
  dashboard.value = dash
  shares.value = shareList
  logs.value = logList.items
  settings.value = settingMap
}

async function loadAll() {
  loading.value = true
  try {
    await loadSite()
    await Promise.all([loadFiles(), loadAdminData()])
  }
  catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载失败')
  }
  finally {
    loading.value = false
  }
}

async function runSearch() {
  if (!isLoggedIn.value || searchMode.value !== 'global') {
    globalResults.value = []
    return
  }
  const term = keyword.value.trim()
  if (!term) {
    globalResults.value = []
    return
  }
  searching.value = true
  try {
    const params = new URLSearchParams({ storageKey: activeSource.value?.key || 'local', q: term, limit: '100' })
    globalResults.value = await api<FileEntry[]>(`/api/files/search?${params.toString()}`)
  }
  catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '搜索失败')
  }
  finally {
    searching.value = false
  }
}

function changeSearchMode() {
  void runSearch()
}

function openFolder(file: FileEntry) {
  activePath.value = file.path
  searchMode.value = 'current'
  keyword.value = ''
  globalResults.value = []
  void loadFiles()
}

function operationAllowed(operation: string) {
  return !disabledOperations.value.has(operation)
}

function requireOperation(operation: string) {
  if (operationAllowed(operation))
    return true
  ElMessage.warning(`${operation} operation is disabled`)
  return false
}

function currentFileUrl(path: string, preview = false) {
  if (isLoggedIn.value)
    return fileUrl(path, activeSource.value?.key || 'local', preview)
  return publicFileUrl(activeSource.value?.key || 'local', path, directoryPassword.value, preview)
}

function currentAbsoluteFileUrl(path: string, preview = false) {
  const url = currentFileUrl(path, preview)
  if (typeof window === 'undefined')
    return url
  return new URL(url, window.location.origin).toString()
}

function externalPreviewFileKey(file: FileEntry) {
  return stablePreviewKey(`${activeSource.value?.key || 'local'}:${file.path}:${file.size}:${file.modifiedAt}`)
}

function externalPreviewUrl(file: FileEntry) {
  if (!isLoggedIn.value || file.type !== 'file')
    return ''
  return buildExternalPreviewUrl(settings.value, {
    name: file.name,
    ext: fileExtension(file),
    url: currentAbsoluteFileUrl(file.path, true),
    key: externalPreviewFileKey(file),
  })
}

function canExternalPreview(file: FileEntry) {
  return isLoggedIn.value
    && operationAllowed('preview')
    && file.type === 'file'
    && supportsExternalPreview(settings.value, fileExtension(file))
    && Boolean(externalPreviewUrl(file))
}

function openExternalPreview(file: FileEntry) {
  if (!requireOperation('preview'))
    return
  const url = externalPreviewUrl(file)
  if (!url) {
    ElMessage.warning('外部预览未配置')
    return
  }
  window.open(url, '_blank', 'noopener')
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

async function downloadArchive(paths: string[], filename = 'xfile-archive.zip') {
  if (!requireOperation('download'))
    return
  if (!paths.length)
    return
  try {
    const res = await fetch('/api/files/archive', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', ...await csrfHeaders() },
      body: JSON.stringify({
        storageKey: activeSource.value?.key || 'local',
        paths,
      }),
    })
    if (!res.ok)
      throw new Error(await res.text() || res.statusText)
    downloadBlob(await res.blob(), filename)
  }
  catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '打包下载失败')
  }
}

function openFile(file: FileEntry) {
  if (file.type === 'folder') {
    openFolder(file)
    return
  }
  if (canPreview(file)) {
    void openPreview(file)
    return
  }
  window.open(currentFileUrl(file.path), '_blank')
}

function openInNewTab(file: FileEntry) {
  if (file.type === 'folder') {
    const query = new URLSearchParams({ source: activeSourceKey.value, path: file.path })
    window.open(`/?${query.toString()}`, '_blank')
    return
  }
  window.open(currentFileUrl(file.path), '_blank')
}

function fileExtension(file: FileEntry) {
  const name = file.name || file.path
  const index = name.lastIndexOf('.')
  return index >= 0 ? name.slice(index + 1).toLowerCase() : ''
}

function textByteSize(value: string) {
  return new Blob([value]).size
}

function previewType(file: FileEntry): PreviewKind {
  if (file.type === 'folder')
    return 'unsupported'
  const ext = fileExtension(file)
  if (['apng', 'avif', 'bmp', 'gif', 'jpeg', 'jpg', 'png', 'svg', 'webp'].includes(ext))
    return 'image'
  if (['mp4', 'm4v', 'mov', 'ogg', 'ogv', 'webm'].includes(ext))
    return 'video'
  if (['aac', 'flac', 'm4a', 'mp3', 'oga', 'ogg', 'wav', 'webm'].includes(ext))
    return 'audio'
  if (ext === 'pdf')
    return 'pdf'
  if (officePreviewKind(file))
    return 'office'
  if (['css', 'csv', 'env', 'go', 'html', 'ini', 'js', 'json', 'log', 'md', 'scss', 'sql', 'svg', 'text', 'toml', 'ts', 'txt', 'vue', 'xml', 'yaml', 'yml'].includes(ext))
    return 'text'
  if (canExternalPreview(file))
    return 'external'
  return 'unsupported'
}

function isImageFile(file: FileEntry) {
  return previewType(file) === 'image'
}

function isVideoFile(file: FileEntry) {
  return previewType(file) === 'video'
}

function previewFileUrl(file: FileEntry) {
  return currentFileUrl(file.path, true)
}

function videoThumbnailUrl(file: FileEntry) {
  return `${previewFileUrl(file)}#t=0.1`
}

function videoMimeType(file: FileEntry) {
  const ext = fileExtension(file)
  const types: Record<string, string> = {
    m4v: 'video/mp4',
    mov: 'video/mp4',
    mp4: 'video/mp4',
    ogg: 'video/ogg',
    ogv: 'video/ogg',
    webm: 'video/webm',
  }
  return types[ext]
}

function officePreviewKind(file: FileEntry): OfficePreviewKind | undefined {
  const ext = fileExtension(file)
  if (ext === 'docx')
    return 'docx'
  if (['xls', 'xlsx'].includes(ext))
    return 'excel'
  if (ext === 'pptx')
    return 'pptx'
  return undefined
}

function resolvedOfficePreviewKind(file: FileEntry): OfficePreviewKind {
  return officePreviewKind(file) || 'docx'
}

function seekVideoThumbnail(event: Event) {
  const video = event.target as HTMLVideoElement
  if (!Number.isFinite(video.duration) || video.duration <= 0 || video.currentTime > 0)
    return
  try {
    video.currentTime = Math.min(0.1, video.duration / 2)
  }
  catch {
    // Some browsers/storage adapters reject early seeking; metadata preload still gives a usable frame.
  }
}

function imagePreviewIndex(file: FileEntry) {
  const index = displayedFiles.value.filter(item => isImageFile(item)).findIndex(item => item.path === file.path)
  return Math.max(0, index)
}

function fileBadgeLabel(file: FileEntry) {
  const ext = fileExtension(file)
  return ext ? ext.slice(0, 6).toUpperCase() : 'FILE'
}

function fileMeta(file: FileEntry) {
  if (file.type === 'folder')
    return formatTime(file.modifiedAt)
  return `${formatBytes(file.size)} / ${formatTime(file.modifiedAt)}`
}

function canPreview(file: FileEntry) {
  return operationAllowed('preview') && previewType(file) !== 'unsupported'
}

async function openPreview(file: FileEntry) {
  if (!requireOperation('preview'))
    return
  const kind = previewType(file)
  if (kind === 'unsupported') {
    window.open(currentFileUrl(file.path), '_blank')
    return
  }
  if (kind === 'external') {
    openExternalPreview(file)
    return
  }
  previewFile.value = file
  previewKind.value = kind
  previewText.value = ''
  previewOriginalText.value = ''
  previewTextTooLarge.value = false
  previewTextError.value = false
  previewTextSaving.value = false
  previewVisible.value = true
  if (kind !== 'text')
    return

  previewLoading.value = true
  try {
    if (file.size > textEditLimitBytes) {
      previewTextTooLarge.value = true
      previewText.value = '文本文件超过 2 MB，请下载后查看。'
      previewOriginalText.value = previewText.value
      return
    }
    const res = await fetch(currentFileUrl(file.path, true), { credentials: 'same-origin' })
    if (!res.ok)
      throw new Error(await res.text() || res.statusText)
    previewText.value = await res.text()
    previewOriginalText.value = previewText.value
  }
  catch (error) {
    previewTextError.value = true
    previewText.value = error instanceof Error ? error.message : '文本预览失败'
    previewOriginalText.value = previewText.value
  }
  finally {
    previewLoading.value = false
  }
}

function downloadPreview() {
  if (previewFile.value)
    window.open(currentFileUrl(previewFile.value.path), '_blank')
}

function updateFileEntry(entry: FileEntry) {
  files.value = files.value.map(file => file.path === entry.path ? entry : file)
  globalResults.value = globalResults.value.map(file => file.path === entry.path ? entry : file)
  selectedFiles.value = selectedFiles.value.map(file => file.path === entry.path ? entry : file)
  if (previewFile.value?.path === entry.path)
    previewFile.value = entry
}

async function savePreviewText() {
  if (!previewFile.value || !previewTextEditable.value || !previewTextDirty.value)
    return
  if (!requireOperation('upload'))
    return
  if (textByteSize(previewText.value) > textEditLimitBytes) {
    ElMessage.warning('文本内容超过 2 MB')
    return
  }
  previewTextSaving.value = true
  try {
    const entry = await api<FileEntry>('/api/files/text', {
      method: 'PUT',
      body: JSON.stringify({
        storageKey: activeSource.value?.key || 'local',
        path: previewFile.value.path,
        content: previewText.value,
      }),
    })
    updateFileEntry(entry)
    previewOriginalText.value = previewText.value
    ElMessage.success('保存完成')
  }
  catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存失败')
  }
  finally {
    previewTextSaving.value = false
  }
}

function openMetadataDialog(file: FileEntry) {
  if (!isLoggedIn.value || !operationAllowed('rename'))
    return
  metadataFile.value = file
  metadataDescription.value = file.description || ''
  metadataVisible.value = true
}

async function saveMetadata() {
  if (!metadataFile.value)
    return
  if (!requireOperation('rename'))
    return
  metadataSaving.value = true
  try {
    const entry = await api<FileEntry>('/api/files/metadata', {
      method: 'PUT',
      body: JSON.stringify({
        storageKey: activeSource.value?.key || 'local',
        path: metadataFile.value.path,
        description: metadataDescription.value,
      }),
    })
    updateFileEntry(entry)
    metadataFile.value = entry
    metadataDescription.value = entry.description || ''
    metadataVisible.value = false
    ElMessage.success(entry.description ? '备注已保存' : '备注已清除')
  }
  catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '备注保存失败')
  }
  finally {
    metadataSaving.value = false
  }
}

function downloadFile(file: FileEntry) {
  if (!requireOperation('download'))
    return
  if (file.type === 'folder') {
    void downloadArchive([file.path], `${file.name}.zip`)
    return
  }
  window.open(currentFileUrl(file.path), '_blank')
}

async function downloadSelected() {
  if (!requireOperation('download'))
    return
  if (!selectedFiles.value.length)
    return
  const count = selectedFiles.value.length
  await downloadArchive(selectedFiles.value.map(file => file.path), count === 1 ? `${selectedFiles.value[0].name}.zip` : 'xfile-archive.zip')
  selectedFiles.value = []
}

async function uploadFile(event: Event) {
  const input = event.target as HTMLInputElement
  const selected = Array.from(input.files || [])
  await uploadFiles(selected)
  input.value = ''
}

async function uploadFiles(selected: File[]) {
  if (!selected.length)
    return
  if (!requireOperation('upload'))
    return
  const form = new FormData()
  for (const file of selected) {
    form.set('path', activePath.value)
    form.set('storageKey', activeSource.value?.key || 'local')
    form.set('file', file)
    await api('/api/files/upload', { method: 'POST', body: form })
  }
  ElMessage.success(selected.length > 1 ? `已上传 ${selected.length} 个文件` : '上传完成')
  await loadFiles()
}

function canDragUpload() {
  return isLoggedIn.value && operationAllowed('upload') && !!activeSource.value?.enabled
}

function dragHasFiles(event: DragEvent) {
  return Array.from(event.dataTransfer?.types || []).includes('Files')
}

function onUploadDragEnter(event: DragEvent) {
  if (canDragUpload() && dragHasFiles(event))
    dragUploadActive.value = true
}

function onUploadDragLeave() {
  dragUploadActive.value = false
}

async function onUploadDrop(event: DragEvent) {
  dragUploadActive.value = false
  if (!canDragUpload())
    return
  const dropped = Array.from(event.dataTransfer?.files || [])
  await uploadFiles(dropped)
}

async function createFolder() {
  if (!requireOperation('upload'))
    return
  const { value } = await ElMessageBox.prompt('输入新目录名称', '新建目录', {
    inputPattern: /^[^\\/]+$/,
    inputErrorMessage: '目录名称不能包含斜杠',
  })
  const path = [activePath.value, value].filter(Boolean).join('/')
  await api('/api/files/folders', { method: 'POST', body: JSON.stringify({ storageKey: activeSource.value?.key || 'local', path }) })
  ElMessage.success('目录已创建')
  await loadFiles()
}

async function createEmptyFile() {
  if (!requireOperation('upload'))
    return
  const { value } = await ElMessageBox.prompt('输入文件名称，例如 notes.txt', '新建文件', {
    inputPattern: /^[^\\/]+$/,
    inputErrorMessage: '文件名称不能包含斜杠',
  })
  const path = [activePath.value, value].filter(Boolean).join('/')
  await api('/api/files/empty', { method: 'POST', body: JSON.stringify({ storageKey: activeSource.value?.key || 'local', path }) })
  ElMessage.success('文件已创建')
  await loadFiles()
}

async function renameFile(file: FileEntry) {
  if (!requireOperation('rename'))
    return
  const { value } = await ElMessageBox.prompt('输入新的名称', '重命名', {
    inputValue: file.name,
    inputPattern: /^[^\\/]+$/,
    inputErrorMessage: '名称不能包含斜杠',
  })
  const base = file.path.split('/').slice(0, -1).join('/')
  const to = [base, value].filter(Boolean).join('/')
  await api('/api/files', { method: 'PATCH', body: JSON.stringify({ storageKey: activeSource.value?.key || 'local', from: file.path, to }) })
  ElMessage.success('已重命名')
  await loadFiles()
}

async function moveFile(file: FileEntry) {
  if (!requireOperation('move'))
    return
  openTargetPicker('move', [file])
}

async function moveSelected() {
  if (!requireOperation('move'))
    return
  if (!selectedFiles.value.length)
    return
  openTargetPicker('move', selectedFiles.value)
}

async function copySelected() {
  if (!requireOperation('copy'))
    return
  if (!selectedFiles.value.length)
    return
  openTargetPicker('copy', selectedFiles.value)
}

async function createShare(file: FileEntry) {
  if (!requireOperation('share'))
    return
  if ((activeSource.value?.key || 'local') !== 'local') {
    ElMessage.info('跨存储源分享将在下一项接入')
    return
  }
  const { value: customKey } = await ElMessageBox.prompt('可选：自定义分享 Key，留空自动生成', '创建分享', {
    inputPlaceholder: '例如 project-docs',
    inputPattern: /^$|^[\w-]{4,64}$/,
    inputErrorMessage: '请输入 4-64 位字母、数字、下划线或短横线',
  })
  const { value: password } = await ElMessageBox.prompt('可选：设置分享密码，留空表示公开', '创建分享', {
    inputPlaceholder: '分享密码',
  })
  const share = await api<ShareEntry>('/api/shares', {
    method: 'POST',
    body: JSON.stringify({ path: file.path, password, customKey }),
  })
  shares.value.unshift(share)
  ElMessage.success('分享链接已生成')
}

async function shareSelected() {
  if (!requireOperation('share'))
    return
  if (!selectedFiles.value.length)
    return
  if ((activeSource.value?.key || 'local') !== 'local') {
    ElMessage.info('跨存储源批量分享将在下一项接入')
    return
  }
  const { value: password } = await ElMessageBox.prompt('可选：为这批分享设置同一个访问密码，留空表示公开。', '批量分享', {
    inputPlaceholder: '分享密码',
  })
  const count = selectedFiles.value.length
  const created = await api<ShareEntry[]>('/api/shares/batch', {
    method: 'POST',
    body: JSON.stringify({
      paths: selectedFiles.value.map(file => file.path),
      password,
    }),
  })
  shares.value.unshift(...created)
  selectedFiles.value = []
  ElMessage.success(`已生成 ${count} 个分享链接`)
}

async function createDirectLink(file: FileEntry) {
  if (!requireOperation('directLinks'))
    return
  if ((activeSource.value?.key || 'local') !== 'local') {
    ElMessage.info('跨存储源直链将在下一项接入')
    return
  }
  await api('/api/direct-links', {
    method: 'POST',
    body: JSON.stringify({ path: file.path }),
  })
  ElMessage.success('直链已生成')
}

async function directLinkSelected() {
  if (!requireOperation('directLinks'))
    return
  if (!selectedFiles.value.length)
    return
  if ((activeSource.value?.key || 'local') !== 'local') {
    ElMessage.info('跨存储源批量直链将在后续接入')
    return
  }
  const count = selectedFiles.value.length
  await api<DirectLinkEntry[]>('/api/direct-links/batch', {
    method: 'POST',
    body: JSON.stringify({
      paths: selectedFiles.value.map(file => file.path),
    }),
  })
  selectedFiles.value = []
  ElMessage.success(`已生成 ${count} 个直链`)
}

async function removeFile(file: FileEntry) {
  if (!requireOperation('delete'))
    return
  await ElMessageBox.confirm(`确认删除 ${file.name}？`, '删除文件')
  const params = new URLSearchParams({ storageKey: activeSource.value?.key || 'local', path: file.path })
  await api(`/api/files?${params.toString()}`, { method: 'DELETE' })
  ElMessage.success('已删除')
  await loadFiles()
}

async function removeSelected() {
  if (!requireOperation('delete'))
    return
  if (!selectedFiles.value.length)
    return
  await ElMessageBox.confirm(`确认删除选中的 ${selectedFiles.value.length} 项？`, '批量删除')
  for (const file of selectedFiles.value) {
    const params = new URLSearchParams({ storageKey: activeSource.value?.key || 'local', path: file.path })
    await api(`/api/files?${params.toString()}`, { method: 'DELETE' })
  }
  selectedFiles.value = []
  ElMessage.success('已删除')
  await loadFiles()
}

async function copyPath(file: FileEntry) {
  await navigator.clipboard.writeText(file.path)
  ElMessage.success('路径已复制')
}

function onSelectionChange(rows: FileEntry[]) {
  selectedFiles.value = rows
}

function clearSelectedFiles() {
  selectedFiles.value = []
}

function isFileSelected(file: FileEntry) {
  return selectedFiles.value.some(item => item.path === file.path)
}

function setFileSelected(file: FileEntry, selected: boolean) {
  if (!isLoggedIn.value)
    return
  if (selected) {
    if (!isFileSelected(file))
      selectedFiles.value = [...selectedFiles.value, file]
    return
  }
  selectedFiles.value = selectedFiles.value.filter(item => item.path !== file.path)
}

function toggleFileSelection(file: FileEntry) {
  setFileSelected(file, !isFileSelected(file))
}

function updateFileCardSelection(file: FileEntry, checked: string | number | boolean) {
  setFileSelected(file, Boolean(checked))
}

function handleFileCardClick(file: FileEntry) {
  if (isLoggedIn.value) {
    toggleFileSelection(file)
    return
  }
  openFile(file)
}

function menuPosition(event: MouseEvent) {
  const menuWidth = 172
  const menuHeight = 440
  const gap = 12
  return {
    x: Math.max(gap, Math.min(event.clientX, window.innerWidth - menuWidth - gap)),
    y: Math.max(gap, Math.min(event.clientY, window.innerHeight - menuHeight - gap)),
  }
}

function openContextMenu(row: FileEntry, _column: unknown, event: MouseEvent) {
  if (!isLoggedIn.value)
    return
  event.preventDefault()
  const position = menuPosition(event)
  contextMenu.value = {
    visible: true,
    x: position.x,
    y: position.y,
    file: row,
    directory: false,
  }
}

function openFileCardContextMenu(file: FileEntry, event: MouseEvent) {
  if (!isLoggedIn.value)
    return
  openContextMenu(file, undefined, event)
}

function openDirectoryContextMenu(event: MouseEvent) {
  if (!isLoggedIn.value)
    return
  const target = event.target as HTMLElement
  if (target.closest('.ep-table__row') || target.closest('.file-view-card') || target.closest('.bucket-context-menu'))
    return
  event.preventDefault()
  const position = menuPosition(event)
  contextMenu.value = {
    visible: true,
    x: position.x,
    y: position.y,
    file: undefined,
    directory: true,
  }
}

function closeContextMenu() {
  contextMenu.value.visible = false
}

function runContextAction(action: (file: FileEntry) => void | Promise<void>) {
  const file = contextMenu.value.file
  closeContextMenu()
  if (file)
    void action(file)
}

onMounted(() => {
  window.addEventListener('click', closeContextMenu)
  const path = route.query.path
  const source = route.query.source
  if (typeof path === 'string')
    activePath.value = path
  if (typeof source === 'string')
    activeSourceKey.value = source
  void loadAll()
})

onBeforeUnmount(() => {
  window.removeEventListener('click', closeContextMenu)
})
</script>

<template>
  <div v-loading="loading" class="workspace">
    <section class="bucket-shell">
      <div class="bucket-header">
        <div>
          <p class="eyebrow">
            {{ isLoggedIn ? 'Storage bucket' : 'Public bucket' }}
          </p>
          <h1>{{ site.rootName || dashboard.siteName }}</h1>
          <div class="bucket-meta">
            <el-tag effect="plain">
              {{ currentStorageSource }}
            </el-tag>
            <template v-if="isLoggedIn">
              <span>{{ dashboard.fileCount }} 文件</span>
              <span>{{ dashboard.folderCount }} 文件夹</span>
              <span>{{ formatBytes(dashboard.totalBytes) }}</span>
            </template>
            <span v-else>{{ files.length }} 项</span>
          </div>
        </div>
        <div class="quick-actions">
          <input ref="uploader" class="hidden-input" type="file" multiple @change="uploadFile">
          <el-select
            v-if="selectableSources.length > 1"
            v-model="activeSourceKey"
            class="source-select"
            size="default"
            @change="setSource"
          >
            <el-option
              v-for="source in selectableSources"
              :key="source.key"
              :label="`${source.name} / ${source.typeLabel}${isLoggedIn && !source.enabled ? ' / 停用' : ''}`"
              :value="source.key"
            />
          </el-select>
          <RouterLink v-if="!isLoggedIn" to="/login">
            <el-button type="primary">
              登录管理
            </el-button>
          </RouterLink>
          <el-button :icon="Clock" @click="loadAll">
            刷新
          </el-button>
          <template v-if="isLoggedIn">
            <el-button :icon="Plus" @click="createFolder">
              新建文件夹
            </el-button>
            <el-button v-if="operationAllowed('upload')" type="primary" :icon="Upload" @click="uploader?.click()">
              上传文件
            </el-button>
          </template>
        </div>
      </div>

      <div class="bucket-toolbar">
        <el-breadcrumb separator="/">
          <el-breadcrumb-item v-for="item in breadcrumbs" :key="item.path">
            <button class="crumb-button" @click="activePath = item.path; loadFiles()">
              {{ item.label }}
            </button>
          </el-breadcrumb-item>
        </el-breadcrumb>
        <div class="file-search-tools">
          <el-button v-if="isLoggedIn && selectedCount && operationAllowed('move')" plain :icon="Link" @click="moveSelected">
            移动 {{ selectedCount }}
          </el-button>
          <el-button v-if="isLoggedIn && selectedCount && operationAllowed('copy')" plain :icon="CopyDocument" @click="copySelected">
            复制 {{ selectedCount }}
          </el-button>
          <el-button v-if="isLoggedIn && selectedCount && operationAllowed('share')" plain :icon="Share" @click="shareSelected">
            分享 {{ selectedCount }}
          </el-button>
          <el-button v-if="isLoggedIn && selectedCount && operationAllowed('directLinks')" plain :icon="Link" @click="directLinkSelected">
            直链 {{ selectedCount }}
          </el-button>
          <el-button v-if="isLoggedIn && selectedCount && operationAllowed('download')" plain :icon="Download" @click="downloadSelected">
            下载 {{ selectedCount }}
          </el-button>
          <el-button v-if="isLoggedIn && selectedCount && operationAllowed('delete')" type="danger" plain :icon="Delete" @click="removeSelected">
            删除 {{ selectedCount }}
          </el-button>
          <el-segmented
            v-model="fileViewMode"
            class="file-view-switcher"
            :options="fileViewModeOptions"
            size="small"
            @change="clearSelectedFiles"
          />
          <el-segmented
            v-if="isLoggedIn"
            v-model="searchMode"
            :options="searchModeOptions"
            size="small"
            @change="changeSearchMode"
          />
          <el-input
            v-model="keyword"
            class="search-input"
            :placeholder="isLoggedIn && searchMode === 'global' ? '搜索全部文件' : '搜索当前目录'"
            :prefix-icon="Search"
            :loading="searching"
            clearable
            @input="runSearch"
            @clear="runSearch"
          />
        </div>
      </div>

      <div
        class="bucket-table-zone"
        :class="{ 'is-drag-upload-active': dragUploadActive }"
        @contextmenu="openDirectoryContextMenu"
        @dragenter.prevent="onUploadDragEnter"
        @dragover.prevent="onUploadDragEnter"
        @dragleave.prevent="onUploadDragLeave"
        @drop.prevent="onUploadDrop"
      >
        <div v-if="dragUploadActive" class="drag-upload-overlay">
          <el-icon>
            <Upload />
          </el-icon>
          <span>释放以上传到当前目录</span>
        </div>
        <el-table
          v-if="fileViewMode === 'details'"
          :data="displayedFiles"
          class="bucket-table"
          :empty-text="fileEmptyText"
          row-key="path"
          @selection-change="onSelectionChange"
          @row-contextmenu="openContextMenu"
        >
          <el-table-column v-if="isLoggedIn" type="selection" width="44" :reserve-selection="true" />
          <el-table-column label="文件名" min-width="360" sortable>
            <template #default="{ row }">
              <div class="file-title-cell">
                <button class="file-name" @click="openFile(row)">
                  <el-icon>
                    <Folder v-if="row.type === 'folder'" />
                    <Document v-else />
                  </el-icon>
                  <span>{{ row.name }}</span>
                </button>
                <p v-if="row.description" class="file-description">
                  {{ row.description }}
                </p>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="修改时间" width="190" sortable>
            <template #default="{ row }">
              {{ formatTime(row.modifiedAt) }}
            </template>
          </el-table-column>
          <el-table-column label="大小" width="150" sortable>
            <template #default="{ row }">
              {{ row.type === 'folder' ? '-' : formatBytes(row.size) }}
            </template>
          </el-table-column>
          <el-table-column label="" width="280" align="right" class-name="bucket-actions-column">
            <template #default="{ row }">
              <div class="bucket-row-actions">
                <el-button v-if="row.type === 'file' && canPreview(row)" text :icon="View" title="预览" @click="openPreview(row)" />
                <el-button v-if="operationAllowed('download')" text :icon="Download" title="下载" @click="downloadFile(row)" />
                <template v-if="isLoggedIn">
                  <el-button v-if="operationAllowed('rename')" text :icon="Edit" title="备注" @click="openMetadataDialog(row)" />
                  <el-button v-if="operationAllowed('share')" text :icon="Share" title="分享" @click="createShare(row)" />
                  <el-button v-if="operationAllowed('delete')" text type="danger" :icon="Delete" title="删除" @click="removeFile(row)" />
                </template>
              </div>
            </template>
          </el-table-column>
        </el-table>
        <div
          v-else-if="displayedFiles.length"
          class="file-view-grid"
          :class="`file-view-grid--${fileViewMode}`"
        >
          <article
            v-for="file in displayedFiles"
            :key="file.path"
            class="file-view-card"
            :class="{ 'is-selected': isFileSelected(file) }"
            tabindex="0"
            @click="handleFileCardClick(file)"
            @dblclick.stop="openFile(file)"
            @keydown.enter.prevent="openFile(file)"
            @keydown.space.prevent="handleFileCardClick(file)"
            @contextmenu="openFileCardContextMenu(file, $event)"
          >
            <div v-if="isLoggedIn" class="file-card-check" @click.stop>
              <el-checkbox
                :model-value="isFileSelected(file)"
                @change="updateFileCardSelection(file, $event)"
              />
            </div>
            <div class="file-card-thumb">
              <button
                v-if="file.type === 'folder'"
                class="file-card-thumb-button file-card-folder"
                type="button"
                @click.stop="openFile(file)"
              >
                <el-icon><Folder /></el-icon>
              </button>
              <el-image
                v-else-if="isImageFile(file)"
                class="file-card-image"
                :src="previewFileUrl(file)"
                :alt="file.name"
                fit="cover"
                lazy
                :preview-src-list="imagePreviewUrls"
                :initial-index="imagePreviewIndex(file)"
                preview-teleported
                hide-on-click-modal
                @click.stop
              >
                <template #error>
                  <div class="file-card-image-error">
                    <el-icon><Picture /></el-icon>
                  </div>
                </template>
              </el-image>
              <div
                v-else-if="isVideoFile(file)"
                class="file-card-thumb-button file-card-video-thumb"
                role="button"
                @click.stop="openPreview(file)"
              >
                <video
                  class="file-card-video"
                  :src="videoThumbnailUrl(file)"
                  preload="metadata"
                  muted
                  playsinline
                  @loadedmetadata="seekVideoThumbnail"
                />
                <span class="file-card-video-mark">
                  <el-icon><VideoPlay /></el-icon>
                </span>
              </div>
              <button
                v-else
                class="file-card-thumb-button file-card-file"
                type="button"
                @click.stop="openFile(file)"
              >
                <el-icon><Document /></el-icon>
                <span>{{ fileBadgeLabel(file) }}</span>
              </button>
            </div>
            <div class="file-card-info">
              <button class="file-card-name" type="button" @click.stop="openFile(file)">
                {{ file.name }}
              </button>
              <span class="file-card-meta">{{ fileMeta(file) }}</span>
              <p v-if="file.description" class="file-card-description">
                {{ file.description }}
              </p>
            </div>
            <div class="file-card-actions" @click.stop>
              <el-button v-if="file.type === 'file' && canPreview(file)" text :icon="View" title="预览" @click="openPreview(file)" />
              <el-button v-if="operationAllowed('download')" text :icon="Download" title="下载" @click="downloadFile(file)" />
              <template v-if="isLoggedIn">
                <el-button v-if="operationAllowed('rename')" text :icon="Edit" title="备注" @click="openMetadataDialog(file)" />
                <el-button v-if="operationAllowed('share')" text :icon="Share" title="分享" @click="createShare(file)" />
                <el-button v-if="operationAllowed('delete')" text type="danger" :icon="Delete" title="删除" @click="removeFile(file)" />
              </template>
            </div>
          </article>
        </div>
        <el-empty v-else class="file-view-empty" :description="fileEmptyText" />
      </div>

      <div
        v-if="contextMenu.visible && (contextMenu.file || contextMenu.directory)"
        class="bucket-context-menu"
        :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }"
        @click.stop
      >
        <template v-if="contextMenu.file">
          <button @click="runContextAction(openFile)">
            <el-icon><Folder /></el-icon><span>打开</span>
          </button>
          <button @click="runContextAction(openInNewTab)">
            <el-icon><Document /></el-icon><span>新标签打开</span>
          </button>
          <button v-if="operationAllowed('download')" @click="runContextAction(downloadFile)">
            <el-icon><Download /></el-icon><span>打包/下载</span>
          </button>
          <button v-if="operationAllowed('share')" @click="runContextAction(createShare)">
            <el-icon><Share /></el-icon><span>创建分享</span>
          </button>
          <button v-if="operationAllowed('directLinks')" @click="runContextAction(createDirectLink)">
            <el-icon><Link /></el-icon><span>生成直链</span>
          </button>
          <button v-if="isLoggedIn && operationAllowed('rename')" @click="runContextAction(openMetadataDialog)">
            <el-icon><Edit /></el-icon><span>编辑备注</span>
          </button>
          <button v-if="operationAllowed('rename')" @click="runContextAction(renameFile)">
            <el-icon><Edit /></el-icon><span>重命名</span>
          </button>
          <button v-if="operationAllowed('move')" @click="runContextAction(moveFile)">
            <el-icon><Link /></el-icon><span>移动</span>
          </button>
          <button @click="runContextAction(copyPath)">
            <el-icon><CopyDocument /></el-icon><span>复制路径</span>
          </button>
          <button v-if="operationAllowed('delete')" class="danger" @click="runContextAction(removeFile)">
            <el-icon><Delete /></el-icon><span>删除</span>
          </button>
          <hr>
        </template>
        <button v-if="operationAllowed('upload')" @click="closeContextMenu(); createFolder()">
          <el-icon><Plus /></el-icon><span>新建文件夹</span>
        </button>
        <button v-if="operationAllowed('upload')" @click="closeContextMenu(); createEmptyFile()">
          <el-icon><Document /></el-icon><span>新建文件</span>
        </button>
        <button v-if="operationAllowed('upload')" @click="closeContextMenu(); uploader?.click()">
          <el-icon><Upload /></el-icon><span>上传文件</span>
        </button>
        <button @click="closeContextMenu(); loadAll()">
          <el-icon><Clock /></el-icon><span>刷新</span>
        </button>
      </div>
    </section>

    <section v-if="isLoggedIn" class="lower-grid">
      <section class="panel">
        <div class="panel-title">
          <el-icon><DataAnalysis /></el-icon>
          <span>存储概览</span>
        </div>
        <div class="storage-overview-grid">
          <div>
            <span>文件</span>
            <strong>{{ dashboard.fileCount }}</strong>
          </div>
          <div>
            <span>文件夹</span>
            <strong>{{ dashboard.folderCount }}</strong>
          </div>
          <div>
            <span>分享</span>
            <strong>{{ dashboard.shareCount }}</strong>
          </div>
        </div>
      </section>

      <section class="panel">
        <div class="panel-title">
          <el-icon><Share /></el-icon>
          <span>最近分享</span>
        </div>
        <div v-for="share in shares.slice(0, 5)" :key="share.id" class="list-row">
          <div>
            <strong>{{ share.path }}</strong>
            <span>{{ share.url }} / {{ share.protected ? '有密码' : '公开' }}</span>
          </div>
          <el-tag size="small">
            {{ share.expiresAt || '长期有效' }}
          </el-tag>
        </div>
      </section>

      <section class="panel">
        <div class="panel-title">
          <el-icon><Clock /></el-icon>
          <span>最近访问</span>
        </div>
        <div v-for="log in logs.slice(0, 5)" :key="log.id" class="list-row">
          <div>
            <strong>{{ log.action }} / {{ log.path || '/' }}</strong>
            <span>{{ log.ip }} / {{ formatTime(log.createdAt) }}</span>
          </div>
        </div>
      </section>
    </section>

    <el-dialog
      v-model="previewVisible"
      class="preview-dialog"
      :title="previewFile?.name || '文件预览'"
      :width="previewKind === 'office' ? 'min(96vw, 1180px)' : 'min(92vw, 960px)'"
      destroy-on-close
    >
      <div v-if="previewFile" v-loading="previewLoading" class="preview-body">
        <el-image
          v-if="previewKind === 'image'"
          class="preview-image"
          :src="previewFileUrl(previewFile)"
          :alt="previewFile.name"
          fit="contain"
          :preview-src-list="[previewFileUrl(previewFile)]"
          preview-teleported
          hide-on-click-modal
        />
        <VideoPlayer
          v-else-if="previewKind === 'video'"
          class="preview-media"
          :src="previewFileUrl(previewFile)"
          :type="videoMimeType(previewFile)"
          :title="previewFile.name"
        />
        <audio v-else-if="previewKind === 'audio'" class="preview-audio" :src="currentFileUrl(previewFile.path, true)" controls />
        <iframe v-else-if="previewKind === 'pdf'" class="preview-frame" :src="currentFileUrl(previewFile.path, true)" />
        <OfficePreview
          v-else-if="previewKind === 'office' && officePreviewKind(previewFile)"
          class="preview-office"
          :src="previewFileUrl(previewFile)"
          :kind="resolvedOfficePreviewKind(previewFile)"
        />
        <el-input
          v-else-if="previewKind === 'text'"
          v-model="previewText"
          class="preview-text-editor"
          type="textarea"
          resize="none"
          spellcheck="false"
          :readonly="!previewTextEditable || previewTextSaving"
        />
        <el-empty v-else description="暂不支持预览此文件类型" />
      </div>
      <template #footer>
        <el-button
          v-if="previewTextEditable"
          type="primary"
          :icon="Edit"
          :loading="previewTextSaving"
          :disabled="!previewTextDirty"
          @click="savePreviewText"
        >
          保存
        </el-button>
        <el-button v-if="previewFile && canExternalPreview(previewFile)" :icon="Link" @click="openExternalPreview(previewFile)">
          外部预览
        </el-button>
        <el-button v-if="previewFile" :icon="Download" @click="downloadPreview">
          下载
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="targetPickerVisible"
      class="target-picker-dialog"
      :title="targetPickerTitle"
      width="min(92vw, 680px)"
      destroy-on-close
    >
      <div class="target-picker">
        <div class="target-picker-summary">
          <el-icon>
            <Folder />
          </el-icon>
          <div>
            <span>{{ targetPickerActionLabel }}</span>
            <strong>{{ targetPickerItemSummary }}</strong>
          </div>
        </div>

        <div class="target-picker-nav">
          <el-breadcrumb separator="/">
            <el-breadcrumb-item v-for="item in targetPickerBreadcrumbs" :key="item.path">
              <button class="crumb-button" type="button" @click="setTargetPickerPath(item.path)">
                {{ item.label }}
              </button>
            </el-breadcrumb-item>
          </el-breadcrumb>
          <div class="target-picker-actions">
            <el-button :icon="Folder" @click="setTargetPickerPath('')">
              根目录
            </el-button>
            <el-button :disabled="!targetPickerPath" @click="setTargetPickerPath(targetPickerParentPath())">
              上级
            </el-button>
          </div>
        </div>

        <el-input
          v-model="targetPickerManualPath"
          placeholder="留空表示根目录"
          @keyup.enter="setTargetPickerPath(targetPickerManualPath)"
        >
          <template #prepend>
            目标目录
          </template>
          <template #append>
            <el-button @click="setTargetPickerPath(targetPickerManualPath)">
              打开
            </el-button>
          </template>
        </el-input>

        <div v-loading="targetPickerLoading" class="target-picker-list">
          <button
            v-for="directory in targetPickerDirectories"
            :key="directory.path"
            type="button"
            :disabled="targetDirectoryDisabled(directory)"
            @click="setTargetPickerPath(directory.path)"
          >
            <el-icon>
              <Folder />
            </el-icon>
            <span>{{ directory.name }}</span>
            <small>{{ directory.path }}</small>
          </button>
          <el-empty
            v-if="!targetPickerDirectories.length && !targetPickerLoading"
            description="当前目录没有子目录"
          />
        </div>
      </div>
      <template #footer>
        <el-button @click="targetPickerVisible = false">
          取消
        </el-button>
        <el-button type="primary" :loading="targetPickerSaving" @click="confirmTargetPicker">
          {{ targetPickerConfirmLabel }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="metadataVisible" class="metadata-dialog" title="文件备注" width="min(92vw, 560px)" destroy-on-close>
      <div v-if="metadataFile" class="metadata-form">
        <div class="metadata-target">
          <el-icon>
            <Folder v-if="metadataFile.type === 'folder'" />
            <Document v-else />
          </el-icon>
          <div>
            <strong>{{ metadataFile.name }}</strong>
            <span>{{ metadataFile.path }}</span>
          </div>
        </div>
        <el-input
          v-model="metadataDescription"
          type="textarea"
          resize="none"
          :rows="5"
          :maxlength="metadataDescriptionLimit"
          show-word-limit
          placeholder="添加备注"
        />
      </div>
      <template #footer>
        <el-button @click="metadataVisible = false">
          取消
        </el-button>
        <el-button type="primary" :icon="Edit" :loading="metadataSaving" @click="saveMetadata">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>
