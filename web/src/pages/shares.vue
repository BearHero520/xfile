<script setup lang="ts">
import type { ShareEntry } from '~/api'
import { Delete, Link } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { onMounted, ref } from 'vue'
import { api, formatTime } from '~/api'

const shares = ref<ShareEntry[]>([])
const loading = ref(false)

async function loadShares() {
  loading.value = true
  try {
    shares.value = await api<ShareEntry[]>('/api/shares')
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

function copyLink(url: string) {
  navigator.clipboard?.writeText(`${location.origin}${url}`)
  ElMessage.success('链接已复制')
}

onMounted(loadShares)
</script>

<template>
  <div class="workspace" v-loading="loading">
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
  </div>
</template>
