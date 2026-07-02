<script setup lang="ts">
import type { DirectLinkEntry, LinkAnalytics } from '~/api'
import { Delete, Link } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { onMounted, ref } from 'vue'
import { api, formatTime } from '~/api'

const links = ref<DirectLinkEntry[]>([])
const analytics = ref<LinkAnalytics>({
  shareVisits: [],
  downloadRanking: [],
  directLinkAccesses: [],
})
const loading = ref(false)

async function loadLinks() {
  loading.value = true
  try {
    const [linkList, analyticsData] = await Promise.all([
      api<DirectLinkEntry[]>('/api/direct-links'),
      api<LinkAnalytics>('/api/analytics/links'),
    ])
    links.value = linkList
    analytics.value = analyticsData
  }
  finally {
    loading.value = false
  }
}

async function removeLink(id: number) {
  await api(`/api/direct-links/${id}`, { method: 'DELETE' })
  ElMessage.success('直链已删除')
  await loadLinks()
}

async function toggleLink(row: DirectLinkEntry) {
  await api(`/api/direct-links/${row.id}`, {
    method: 'PATCH',
    body: JSON.stringify({ enabled: !row.enabled }),
  })
  row.enabled = !row.enabled
  ElMessage.success(row.enabled ? '直链已启用' : '直链已停用')
}

function copyLink(url: string) {
  navigator.clipboard?.writeText(`${location.origin}${url}`)
  ElMessage.success('链接已复制')
}

onMounted(loadLinks)
</script>

<template>
  <div v-loading="loading" class="workspace">
    <section class="panel">
      <div class="panel-title">
        直链 / 短链
      </div>
      <p class="lede">
        为文件生成稳定直链、代理下载地址和访问统计。
      </p>
      <el-table :data="links" empty-text="暂无直链">
        <el-table-column prop="path" label="文件路径" min-width="240" show-overflow-tooltip />
        <el-table-column prop="url" label="直链地址" min-width="160" />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-switch :model-value="row.enabled" @change="toggleLink(row)" />
          </template>
        </el-table-column>
        <el-table-column label="访问" width="90">
          <template #default="{ row }">
            {{ row.accessCount || 0 }}
          </template>
        </el-table-column>
        <el-table-column label="最近访问" width="160">
          <template #default="{ row }">
            {{ formatTime(row.lastAccessAt || '') }}
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
            <el-button text type="danger" :icon="Delete" title="删除" @click="removeLink(row.id)" />
          </template>
        </el-table-column>
      </el-table>
    </section>

    <section class="panel">
      <div class="panel-title">
        最近直链访问
      </div>
      <div v-for="log in analytics.directLinkAccesses" :key="log.id" class="list-row">
        <div>
          <strong>{{ log.path || '/' }}</strong>
          <span>{{ log.ip }} / {{ log.userAgent || '-' }}</span>
        </div>
        <el-tag size="small" type="warning" effect="plain">
          {{ formatTime(log.createdAt) }}
        </el-tag>
      </div>
      <el-empty v-if="!analytics.directLinkAccesses.length" description="暂无直链访问" />
    </section>
  </div>
</template>
