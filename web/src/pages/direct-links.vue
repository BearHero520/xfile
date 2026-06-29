<script setup lang="ts">
import type { DirectLinkEntry } from '~/api'
import { Delete, Link } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { onMounted, ref } from 'vue'
import { api, formatTime } from '~/api'

const links = ref<DirectLinkEntry[]>([])
const loading = ref(false)

async function loadLinks() {
  loading.value = true
  try {
    links.value = await api<DirectLinkEntry[]>('/api/direct-links')
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
  <div class="workspace" v-loading="loading">
    <section class="panel">
      <div class="panel-title">
        直链 / 短链
      </div>
      <p class="lede">
        为文件生成稳定直链、代理下载地址和短链入口。
      </p>
      <el-table :data="links" empty-text="暂无直链">
        <el-table-column prop="path" label="文件路径" min-width="260" />
        <el-table-column prop="url" label="直链地址" min-width="180" />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-switch :model-value="row.enabled" @change="toggleLink(row)" />
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="160">
          <template #default="{ row }">
            {{ formatTime(row.createdAt) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="130" align="right">
          <template #default="{ row }">
            <el-button text :icon="Link" @click="copyLink(row.url)" />
            <el-button text type="danger" :icon="Delete" @click="removeLink(row.id)" />
          </template>
        </el-table-column>
      </el-table>
    </section>
  </div>
</template>
