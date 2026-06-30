<script setup lang="ts">
import type { AccessLog, AccessLogPage } from '~/api'
import { Delete, Refresh, Search } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { onMounted, reactive, ref } from 'vue'
import { api, formatTime } from '~/api'

const logs = ref<AccessLog[]>([])
const total = ref(0)
const loading = ref(false)
const cleanupLoading = ref(false)
const filters = reactive({
  action: '',
  path: '',
  ip: '',
  page: 1,
  pageSize: 20,
})

const actionOptions = [
  { label: '全部动作', value: '' },
  { label: '浏览', value: 'list' },
  { label: '搜索', value: 'search' },
  { label: '下载', value: 'download' },
  { label: '上传', value: 'upload' },
  { label: '删除', value: 'delete' },
  { label: '移动', value: 'move' },
  { label: '新建文件夹', value: 'mkdir' },
  { label: '分享访问', value: 'share-view' },
  { label: '分享下载', value: 'share-download' },
  { label: '直链访问', value: 'direct' },
  { label: 'IP 拦截', value: 'ip-blocked' },
  { label: '日志清理', value: 'logs-cleanup' },
]

function logQuery() {
  const params = new URLSearchParams({
    page: String(filters.page),
    pageSize: String(filters.pageSize),
  })
  if (filters.action)
    params.set('action', filters.action)
  if (filters.path.trim())
    params.set('path', filters.path.trim())
  if (filters.ip.trim())
    params.set('ip', filters.ip.trim())
  return params.toString()
}

async function loadLogs() {
  loading.value = true
  try {
    const data = await api<AccessLogPage>(`/api/logs?${logQuery()}`)
    logs.value = data.items
    total.value = data.total
    filters.page = data.page
    filters.pageSize = data.pageSize
  }
  finally {
    loading.value = false
  }
}

function searchLogs() {
  filters.page = 1
  void loadLogs()
}

function resetFilters() {
  filters.action = ''
  filters.path = ''
  filters.ip = ''
  filters.page = 1
  void loadLogs()
}

function changePage(page: number) {
  filters.page = page
  void loadLogs()
}

function changePageSize(pageSize: number) {
  filters.pageSize = pageSize
  filters.page = 1
  void loadLogs()
}

async function cleanupLogs(mode: 'old' | 'all') {
  const all = mode === 'all'
  await ElMessageBox.confirm(
    all ? '确认清空全部访问日志？此操作不可恢复。' : '确认删除 30 天前的访问日志？',
    all ? '清空日志' : '清理旧日志',
    { type: 'warning' },
  )
  cleanupLoading.value = true
  try {
    const data = await api<{ deleted: number }>(all ? '/api/logs?all=true' : '/api/logs?olderThanDays=30', {
      method: 'DELETE',
    })
    ElMessage.success(`已删除 ${data.deleted} 条日志`)
    filters.page = 1
    await loadLogs()
  }
  finally {
    cleanupLoading.value = false
  }
}

onMounted(loadLogs)
</script>

<template>
  <div class="workspace" v-loading="loading">
    <section class="panel">
      <div class="panel-heading">
        <div>
          <div class="panel-title">
            访问日志
          </div>
          <p class="lede">
            查看文件操作、分享、直链、搜索和访问控制拦截记录。
          </p>
        </div>
        <div class="panel-actions">
          <el-button :icon="Delete" :loading="cleanupLoading" @click="cleanupLogs('old')">
            清理 30 天前
          </el-button>
          <el-button type="danger" :icon="Delete" :loading="cleanupLoading" @click="cleanupLogs('all')">
            清空全部
          </el-button>
        </div>
      </div>

      <el-form class="log-filters" :inline="true" @submit.prevent="searchLogs">
        <el-form-item label="动作">
          <el-select v-model="filters.action" class="log-action-select">
            <el-option
              v-for="item in actionOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="路径">
          <el-input v-model="filters.path" clearable placeholder="输入路径关键词" @keyup.enter="searchLogs" />
        </el-form-item>
        <el-form-item label="IP">
          <el-input v-model="filters.ip" clearable placeholder="输入 IP 关键词" @keyup.enter="searchLogs" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="searchLogs">
            查询
          </el-button>
          <el-button :icon="Refresh" @click="resetFilters">
            重置
          </el-button>
        </el-form-item>
      </el-form>

      <el-table :data="logs" empty-text="暂无访问日志">
        <el-table-column prop="action" label="动作" width="130" />
        <el-table-column prop="path" label="路径" min-width="260" show-overflow-tooltip />
        <el-table-column prop="ip" label="IP" width="160" />
        <el-table-column prop="userAgent" label="客户端" min-width="220" show-overflow-tooltip />
        <el-table-column label="时间" width="160">
          <template #default="{ row }">
            {{ formatTime(row.createdAt) }}
          </template>
        </el-table-column>
      </el-table>

      <div class="table-pagination">
        <el-pagination
          background
          layout="total, sizes, prev, pager, next"
          :current-page="filters.page"
          :page-size="filters.pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="total"
          @current-change="changePage"
          @size-change="changePageSize"
        />
      </div>
    </section>
  </div>
</template>
