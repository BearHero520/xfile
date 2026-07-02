<script setup lang="ts">
import type { LinkAnalytics, ShareEntry } from '~/api'
import { Delete, Link } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { onMounted, ref } from 'vue'
import { api, formatTime } from '~/api'

const shares = ref<ShareEntry[]>([])
const analytics = ref<LinkAnalytics>({
  shareVisits: [],
  downloadRanking: [],
  directLinkAccesses: [],
})
const loading = ref(false)
const cleanupLoading = ref(false)

async function loadShares() {
  loading.value = true
  try {
    const [shareList, analyticsData] = await Promise.all([
      api<ShareEntry[]>('/api/shares'),
      api<LinkAnalytics>('/api/analytics/links'),
    ])
    shares.value = shareList
    analytics.value = analyticsData
  }
  finally {
    loading.value = false
  }
}

async function removeShare(id: number) {
  await api(`/api/shares/${id}`, { method: 'DELETE' })
  ElMessage.success('分享已删除')
  await loadShares()
}

async function cleanupExpiredShares() {
  await ElMessageBox.confirm('确认删除所有已过期的分享链接？', '清理过期分享', { type: 'warning' })
  cleanupLoading.value = true
  try {
    const data = await api<{ deleted: number }>('/api/shares/expired', { method: 'DELETE' })
    ElMessage.success(`已清理 ${data.deleted} 个过期分享`)
    await loadShares()
  }
  finally {
    cleanupLoading.value = false
  }
}

function copyLink(url: string) {
  navigator.clipboard?.writeText(`${location.origin}${url}`)
  ElMessage.success('链接已复制')
}

function shareActionLabel(action: string) {
  const labels: Record<string, string> = {
    'share-view': '访问',
    'share-download': '下载',
    'share-password-failed': '密码错误',
    'share-password-rate-limited': '密码限频',
  }
  return labels[action] || action
}

onMounted(loadShares)
</script>

<template>
  <div v-loading="loading" class="workspace">
    <section class="panel">
      <div class="panel-heading">
        <div>
          <div class="panel-title">
            分享列表
          </div>
          <p class="lede">
            管理公开分享、过期时间、访问入口和访问统计。
          </p>
        </div>
        <div class="panel-actions">
          <el-button :icon="Delete" :loading="cleanupLoading" @click="cleanupExpiredShares">
            清理过期
          </el-button>
        </div>
      </div>
      <el-table :data="shares" empty-text="暂无分享链接">
        <el-table-column prop="path" label="文件路径" min-width="240" show-overflow-tooltip />
        <el-table-column prop="url" label="分享地址" min-width="160" />
        <el-table-column label="密码" width="100">
          <template #default="{ row }">
            <el-tag :type="row.protected ? 'warning' : 'info'">
              {{ row.protected ? '有密码' : '公开' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="访问" width="90">
          <template #default="{ row }">
            {{ row.viewCount || 0 }}
          </template>
        </el-table-column>
        <el-table-column label="下载" width="90">
          <template #default="{ row }">
            {{ row.downloadCount || 0 }}
          </template>
        </el-table-column>
        <el-table-column label="最近访问" width="160">
          <template #default="{ row }">
            {{ formatTime(row.lastAccessAt || '') }}
          </template>
        </el-table-column>
        <el-table-column label="有效期" width="140">
          <template #default="{ row }">
            {{ row.expiresAt || '长期有效' }}
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="160">
          <template #default="{ row }">
            {{ formatTime(row.createdAt) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="130" align="right">
          <template #default="{ row }">
            <el-button text :icon="Link" title="复制链接" @click="copyLink(row.url)" />
            <el-button text type="danger" :icon="Delete" title="删除" @click="removeShare(row.id)" />
          </template>
        </el-table-column>
      </el-table>
    </section>

    <section class="lower-grid">
      <section class="panel">
        <div class="panel-title">
          最近分享访问
        </div>
        <div v-for="log in analytics.shareVisits" :key="log.id" class="list-row">
          <div>
            <strong>{{ log.path || '/' }}</strong>
            <span>{{ log.ip }} / {{ log.userAgent || '-' }}</span>
          </div>
          <el-tag size="small" effect="plain">
            {{ shareActionLabel(log.action) }} / {{ formatTime(log.createdAt) }}
          </el-tag>
        </div>
        <el-empty v-if="!analytics.shareVisits.length" description="暂无分享访问" />
      </section>

      <section class="panel">
        <div class="panel-title">
          下载排行
        </div>
        <div v-for="item in analytics.downloadRanking" :key="item.path" class="list-row">
          <div>
            <strong>{{ item.path }}</strong>
            <span>最近 {{ formatTime(item.lastAccessAt || '') }}</span>
          </div>
          <el-tag size="small" type="warning" effect="plain">
            {{ item.count }} 次
          </el-tag>
        </div>
        <el-empty v-if="!analytics.downloadRanking.length" description="暂无下载记录" />
      </section>
    </section>
  </div>
</template>
