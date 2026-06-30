<script setup lang="ts">
import type { AccessLog, AccessLogPage, Dashboard, FileEntry, PublicSite, ShareEntry, StorageSource } from '~/api'
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
  Plus,
  Search,
  Share,
  Upload,
  View,
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api, fileUrl, formatBytes, formatTime, publicFileUrl } from '~/api'

type PreviewKind = 'image' | 'video' | 'audio' | 'pdf' | 'text' | 'unsupported'

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
const activePath = ref('')
const activeSourceKey = ref('')
const keyword = ref('')
const searchMode = ref<'current' | 'global'>('current')
const globalResults = ref<FileEntry[]>([])
const uploader = ref<HTMLInputElement>()
const previewVisible = ref(false)
const previewFile = ref<FileEntry>()
const previewKind = ref<PreviewKind>('unsupported')
const previewText = ref('')
const previewLoading = ref(false)
const selectedFiles = ref<FileEntry[]>([])
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

const searchModeOptions = [
  { label: '当前目录', value: 'current' },
  { label: '全局', value: 'global' },
]

const displayedFiles = computed(() => {
  const term = keyword.value.trim().toLowerCase()
  if (isLoggedIn.value && searchMode.value === 'global')
    return term ? globalResults.value : files.value
  if (!term)
    return files.value
  return files.value.filter(file => `${file.name} ${file.path}`.toLowerCase().includes(term))
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
    files.value = await api<FileEntry[]>(`/api/public/storage/${encodeURIComponent(activeSource.value.key)}/files?path=${encodeURIComponent(activePath.value)}`, { skipAuthRedirect: true } as RequestInit)
  }
  selectedFiles.value = []
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

function currentFileUrl(path: string) {
  if (isLoggedIn.value)
    return fileUrl(path, activeSource.value?.key || 'local')
  return publicFileUrl(activeSource.value?.key || 'local', path)
}

function openFile(file: FileEntry) {
  if (file.type === 'folder') {
    openFolder(file)
    return
  }
  if (previewType(file) !== 'unsupported') {
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
  if (['css', 'csv', 'env', 'go', 'html', 'ini', 'js', 'json', 'log', 'md', 'scss', 'sql', 'svg', 'text', 'toml', 'ts', 'txt', 'vue', 'xml', 'yaml', 'yml'].includes(ext))
    return 'text'
  return 'unsupported'
}

function canPreview(file: FileEntry) {
  return previewType(file) !== 'unsupported'
}

async function openPreview(file: FileEntry) {
  const kind = previewType(file)
  if (kind === 'unsupported') {
    window.open(currentFileUrl(file.path), '_blank')
    return
  }
  previewFile.value = file
  previewKind.value = kind
  previewText.value = ''
  previewVisible.value = true
  if (kind !== 'text')
    return

  previewLoading.value = true
  try {
    if (file.size > 2 * 1024 * 1024) {
      previewText.value = '文本文件超过 2 MB，请下载后查看。'
      return
    }
    const res = await fetch(currentFileUrl(file.path), { credentials: 'same-origin' })
    if (!res.ok)
      throw new Error(await res.text() || res.statusText)
    previewText.value = await res.text()
  }
  catch (error) {
    previewText.value = error instanceof Error ? error.message : '文本预览失败'
  }
  finally {
    previewLoading.value = false
  }
}

function downloadPreview() {
  if (previewFile.value)
    window.open(currentFileUrl(previewFile.value.path), '_blank')
}

function downloadFile(file: FileEntry) {
  if (file.type === 'folder') {
    ElMessage.info('文件夹打包下载待接入')
    return
  }
  window.open(currentFileUrl(file.path), '_blank')
}

async function uploadFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file)
    return
  const form = new FormData()
  form.set('path', activePath.value)
  form.set('storageKey', activeSource.value?.key || 'local')
  form.set('file', file)
  await api('/api/files/upload', { method: 'POST', body: form })
  input.value = ''
  ElMessage.success('上传完成')
  await loadFiles()
}

async function createFolder() {
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
  const { value } = await ElMessageBox.prompt('输入目标路径，例如 docs/readme.md', '移动文件', {
    inputValue: file.path,
  })
  await api('/api/files', { method: 'PATCH', body: JSON.stringify({ storageKey: activeSource.value?.key || 'local', from: file.path, to: value }) })
  ElMessage.success('已移动')
  await loadFiles()
}

async function createShare(file: FileEntry) {
  if ((activeSource.value?.key || 'local') !== 'local') {
    ElMessage.info('跨存储源分享将在下一项接入')
    return
  }
  const { value: password } = await ElMessageBox.prompt('可选：设置分享密码，留空表示公开', '创建分享', {
    inputPlaceholder: '分享密码',
  })
  const share = await api<ShareEntry>('/api/shares', {
    method: 'POST',
    body: JSON.stringify({ path: file.path, password }),
  })
  shares.value.unshift(share)
  ElMessage.success('分享链接已生成')
}

async function createDirectLink(file: FileEntry) {
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

async function removeFile(file: FileEntry) {
  await ElMessageBox.confirm(`确认删除 ${file.name}？`, '删除文件')
  const params = new URLSearchParams({ storageKey: activeSource.value?.key || 'local', path: file.path })
  await api(`/api/files?${params.toString()}`, { method: 'DELETE' })
  ElMessage.success('已删除')
  await loadFiles()
}

async function removeSelected() {
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

function openDirectoryContextMenu(event: MouseEvent) {
  if (!isLoggedIn.value)
    return
  const target = event.target as HTMLElement
  if (target.closest('.ep-table__row') || target.closest('.bucket-context-menu'))
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
  <div class="workspace" v-loading="loading">
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
          <input ref="uploader" class="hidden-input" type="file" @change="uploadFile">
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
            <el-button type="primary" :icon="Upload" @click="uploader?.click()">
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
          <el-button v-if="isLoggedIn && selectedCount" type="danger" plain :icon="Delete" @click="removeSelected">
            删除 {{ selectedCount }}
          </el-button>
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

      <div class="bucket-table-zone" @contextmenu="openDirectoryContextMenu">
        <el-table
          :data="displayedFiles"
          class="bucket-table"
          :empty-text="fileEmptyText"
          @selection-change="onSelectionChange"
          @row-contextmenu="openContextMenu"
        >
          <el-table-column v-if="isLoggedIn" type="selection" width="44" />
          <el-table-column label="文件名" min-width="360" sortable>
            <template #default="{ row }">
              <button class="file-name" @click="openFile(row)">
                <el-icon>
                  <Folder v-if="row.type === 'folder'" />
                  <Document v-else />
                </el-icon>
                <span>{{ row.name }}</span>
              </button>
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
          <el-table-column label="" width="240" align="right" class-name="bucket-actions-column">
            <template #default="{ row }">
              <div class="bucket-row-actions">
                <el-button v-if="row.type === 'file' && canPreview(row)" text :icon="View" title="预览" @click="openPreview(row)" />
                <el-button text :icon="Download" title="下载" @click="downloadFile(row)" />
                <template v-if="isLoggedIn">
                  <el-button text :icon="Share" title="分享" @click="createShare(row)" />
                  <el-button text type="danger" :icon="Delete" title="删除" @click="removeFile(row)" />
                </template>
              </div>
            </template>
          </el-table-column>
        </el-table>
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
          <button @click="runContextAction(downloadFile)">
            <el-icon><Download /></el-icon><span>打包/下载</span>
          </button>
          <button @click="runContextAction(createShare)">
            <el-icon><Share /></el-icon><span>创建分享</span>
          </button>
          <button @click="runContextAction(createDirectLink)">
            <el-icon><Link /></el-icon><span>生成直链</span>
          </button>
          <button @click="runContextAction(renameFile)">
            <el-icon><Edit /></el-icon><span>重命名</span>
          </button>
          <button @click="runContextAction(moveFile)">
            <el-icon><Link /></el-icon><span>移动</span>
          </button>
          <button @click="runContextAction(copyPath)">
            <el-icon><CopyDocument /></el-icon><span>复制路径</span>
          </button>
          <button class="danger" @click="runContextAction(removeFile)">
            <el-icon><Delete /></el-icon><span>删除</span>
          </button>
          <hr>
        </template>
        <button @click="closeContextMenu(); createFolder()">
          <el-icon><Plus /></el-icon><span>新建文件夹</span>
        </button>
        <button @click="closeContextMenu(); createEmptyFile()">
          <el-icon><Document /></el-icon><span>新建文件</span>
        </button>
        <button @click="closeContextMenu(); uploader?.click()">
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

    <el-dialog v-model="previewVisible" class="preview-dialog" :title="previewFile?.name || '文件预览'" width="min(92vw, 960px)" destroy-on-close>
      <div v-if="previewFile" class="preview-body" v-loading="previewLoading">
        <img v-if="previewKind === 'image'" class="preview-image" :src="currentFileUrl(previewFile.path)" :alt="previewFile.name">
        <video v-else-if="previewKind === 'video'" class="preview-media" :src="currentFileUrl(previewFile.path)" controls />
        <audio v-else-if="previewKind === 'audio'" class="preview-audio" :src="currentFileUrl(previewFile.path)" controls />
        <iframe v-else-if="previewKind === 'pdf'" class="preview-frame" :src="currentFileUrl(previewFile.path)" />
        <pre v-else-if="previewKind === 'text'" class="preview-text">{{ previewText }}</pre>
        <el-empty v-else description="暂不支持预览此文件类型" />
      </div>
      <template #footer>
        <el-button v-if="previewFile" :icon="Download" @click="downloadPreview">
          下载
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>
