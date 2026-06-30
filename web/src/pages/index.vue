<script setup lang="ts">
import type { AccessLog, AccessLogPage, Dashboard, FileEntry, ShareEntry } from '~/api'
import {
  Clock,
  DataAnalysis,
  Delete,
  Document,
  Download,
  Edit,
  Files,
  Folder,
  Link,
  Plus,
  Search,
  Share,
  Upload,
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, onMounted, ref } from 'vue'
import { api, fileUrl, formatBytes, formatTime } from '~/api'

const emptyDashboard: Dashboard = {
  siteName: 'XFile',
  storageRoot: 'data/files',
  fileCount: 0,
  folderCount: 0,
  totalBytes: 0,
  shareCount: 0,
  recentFiles: [],
  recentLogs: [],
  storageSources: ['本地存储', 'S3 / WebDAV 规划', '离线下载规划'],
}

const dashboard = ref<Dashboard>(emptyDashboard)
const files = ref<FileEntry[]>([])
const shares = ref<ShareEntry[]>([])
const logs = ref<AccessLog[]>([])
const settings = ref<Record<string, string>>({})
const loading = ref(false)
const searching = ref(false)
const activePath = ref('')
const keyword = ref('')
const searchMode = ref<'current' | 'global'>('current')
const globalResults = ref<FileEntry[]>([])
const uploader = ref<HTMLInputElement>()

const searchModeOptions = [
  { label: '当前目录', value: 'current' },
  { label: '全局', value: 'global' },
]

const displayedFiles = computed(() => {
  const term = keyword.value.trim().toLowerCase()
  if (searchMode.value === 'global')
    return term ? globalResults.value : files.value
  if (!term)
    return files.value
  return files.value.filter(file => `${file.name} ${file.path}`.toLowerCase().includes(term))
})

const filteredFiles = displayedFiles

const fileEmptyText = computed(() => {
  if (searchMode.value === 'global' && keyword.value.trim())
    return '未找到匹配文件'
  return '当前目录暂无文件'
})

const breadcrumbs = computed(() => {
  const parts = activePath.value ? activePath.value.split('/') : []
  return [{ label: settings.value.rootName || '首页', path: '' }].concat(
    parts.map((part, index) => ({ label: part, path: parts.slice(0, index + 1).join('/') })),
  )
})

async function loadAll() {
  loading.value = true
  try {
    const [dash, list, shareList, logList, settingMap] = await Promise.all([
      api<Dashboard>('/api/dashboard'),
      api<FileEntry[]>(`/api/files?path=${encodeURIComponent(activePath.value)}`),
      api<ShareEntry[]>('/api/shares'),
      api<AccessLogPage>('/api/logs?pageSize=5'),
      api<Record<string, string>>('/api/settings'),
    ])
    dashboard.value = dash
    files.value = list
    shares.value = shareList
    logs.value = logList.items
    settings.value = settingMap
  }
  catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载失败')
  }
  finally {
    loading.value = false
  }
}

async function runSearch() {
  if (searchMode.value !== 'global') {
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
    globalResults.value = await api<FileEntry[]>(`/api/files/search?q=${encodeURIComponent(term)}&limit=100`)
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

function openFile(file: FileEntry) {
  if (file.type === 'folder') {
    activePath.value = file.path
    searchMode.value = 'current'
    keyword.value = ''
    globalResults.value = []
    loadAll()
    return
  }
  window.open(fileUrl(file.path), '_blank')
}

async function uploadFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file)
    return
  const form = new FormData()
  form.set('path', activePath.value)
  form.set('file', file)
  await api('/api/files/upload', { method: 'POST', body: form })
  input.value = ''
  ElMessage.success('上传完成')
  await loadAll()
}

async function createFolder() {
  const { value } = await ElMessageBox.prompt('输入新目录名称', '新建目录', {
    inputPattern: /^[^\\/]+$/,
    inputErrorMessage: '目录名称不能包含斜杠',
  })
  const path = [activePath.value, value].filter(Boolean).join('/')
  await api('/api/files/folders', { method: 'POST', body: JSON.stringify({ path }) })
  ElMessage.success('目录已创建')
  await loadAll()
}

async function renameFile(file: FileEntry) {
  const { value } = await ElMessageBox.prompt('输入新的名称', '重命名', {
    inputValue: file.name,
    inputPattern: /^[^\\/]+$/,
    inputErrorMessage: '名称不能包含斜杠',
  })
  const base = file.path.split('/').slice(0, -1).join('/')
  const to = [base, value].filter(Boolean).join('/')
  await api('/api/files', { method: 'PATCH', body: JSON.stringify({ from: file.path, to }) })
  ElMessage.success('已重命名')
  await loadAll()
}

async function moveFile(file: FileEntry) {
  const { value } = await ElMessageBox.prompt('输入目标路径，例如 docs/readme.md', '移动文件', {
    inputValue: file.path,
  })
  await api('/api/files', { method: 'PATCH', body: JSON.stringify({ from: file.path, to: value }) })
  ElMessage.success('已移动')
  await loadAll()
}

async function createShare(file: FileEntry) {
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
  await api('/api/direct-links', {
    method: 'POST',
    body: JSON.stringify({ path: file.path }),
  })
  ElMessage.success('直链已生成')
}

async function removeFile(file: FileEntry) {
  await ElMessageBox.confirm(`确认删除 ${file.name}？`, '删除文件')
  await api(`/api/files?path=${encodeURIComponent(file.path)}`, { method: 'DELETE' })
  ElMessage.success('已删除')
  await loadAll()
}

onMounted(loadAll)
</script>

<template>
  <div class="workspace" v-loading="loading">
    <section class="overview-band">
      <div>
        <p class="eyebrow">
          Self-hosted file operations
        </p>
        <h1>{{ dashboard.siteName }}</h1>
        <p class="lede">
          统一管理本地存储、分享链接、直链、访问日志与规则能力。
        </p>
      </div>
      <div class="quick-actions">
        <input ref="uploader" class="hidden-input" type="file" @change="uploadFile">
        <el-button type="primary" :icon="Upload" @click="uploader?.click()">
          上传文件
        </el-button>
        <el-button :icon="Plus" @click="createFolder">
          新建目录
        </el-button>
      </div>
    </section>

    <section class="metric-grid">
      <article class="metric">
        <el-icon><Files /></el-icon>
        <span>文件</span>
        <strong>{{ dashboard.fileCount }}</strong>
      </article>
      <article class="metric">
        <el-icon><Folder /></el-icon>
        <span>文件夹</span>
        <strong>{{ dashboard.folderCount }}</strong>
      </article>
      <article class="metric">
        <el-icon><DataAnalysis /></el-icon>
        <span>占用空间</span>
        <strong>{{ formatBytes(dashboard.totalBytes) }}</strong>
      </article>
      <article class="metric">
        <el-icon><Link /></el-icon>
        <span>分享链接</span>
        <strong>{{ dashboard.shareCount }}</strong>
      </article>
    </section>

    <section class="content-grid">
      <main class="file-pane">
        <div class="pane-toolbar">
          <el-breadcrumb separator="/">
            <el-breadcrumb-item v-for="item in breadcrumbs" :key="item.path">
              <button class="crumb-button" @click="activePath = item.path; loadAll()">
                {{ item.label }}
              </button>
            </el-breadcrumb-item>
          </el-breadcrumb>
          <div class="file-search-tools">
            <el-segmented
              v-model="searchMode"
              :options="searchModeOptions"
              size="small"
              @change="changeSearchMode"
            />
            <el-input
              v-model="keyword"
              class="search-input"
              :placeholder="searchMode === 'global' ? '搜索全部文件' : '搜索当前目录'"
              :prefix-icon="Search"
              :loading="searching"
              clearable
              @input="runSearch"
              @clear="runSearch"
            />
          </div>
        </div>

        <el-table :data="filteredFiles" class="file-table" empty-text="当前目录暂无文件">
          <el-table-column label="文件名" min-width="260">
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
          <el-table-column label="大小" width="120">
            <template #default="{ row }">
              {{ formatBytes(row.size) }}
            </template>
          </el-table-column>
          <el-table-column label="修改时间" width="150">
            <template #default="{ row }">
              {{ formatTime(row.modifiedAt) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="280" align="right">
            <template #default="{ row }">
              <el-button text :icon="Download" title="下载" @click="openFile(row)" />
              <el-button text :icon="Share" title="分享" @click="createShare(row)" />
              <el-button text :icon="Link" title="直链" @click="createDirectLink(row)" />
              <el-button text :icon="Edit" title="重命名" @click="renameFile(row)" />
              <el-button text title="移动" @click="moveFile(row)">
                移动
              </el-button>
              <el-button text type="danger" :icon="Delete" title="删除" @click="removeFile(row)" />
            </template>
          </el-table-column>
        </el-table>
      </main>

      <aside class="side-stack">
        <section class="panel">
          <div class="panel-title">
            <el-icon><Folder /></el-icon>
            <span>存储源</span>
          </div>
          <div v-for="source in dashboard.storageSources" :key="source" class="source-row">
            <span>{{ source }}</span>
            <el-tag size="small" effect="plain">
              Ready
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
              <strong>{{ log.action }} · {{ log.path || '/' }}</strong>
              <span>{{ log.ip }} · {{ formatTime(log.createdAt) }}</span>
            </div>
          </div>
        </section>
      </aside>
    </section>

    <section class="lower-grid">
      <section class="panel">
        <div class="panel-title">
          <el-icon><Share /></el-icon>
          <span>最近分享</span>
        </div>
        <div v-for="share in shares.slice(0, 5)" :key="share.id" class="list-row">
          <div>
            <strong>{{ share.path }}</strong>
            <span>{{ share.url }} · {{ share.protected ? '有密码' : '公开' }}</span>
          </div>
          <el-tag size="small">
            {{ share.expiresAt || '长期有效' }}
          </el-tag>
        </div>
      </section>

      <section class="panel">
        <div class="panel-title">
          <el-icon><Files /></el-icon>
          <span>最近文件</span>
        </div>
        <div v-for="file in dashboard.recentFiles" :key="file.path" class="list-row">
          <div>
            <strong>{{ file.path }}</strong>
            <span>{{ formatBytes(file.size) }} · {{ formatTime(file.modifiedAt) }}</span>
          </div>
        </div>
      </section>
    </section>
  </div>
</template>
