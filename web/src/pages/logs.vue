<script setup lang="ts">
import type { AccessLog } from '~/api'
import { onMounted, ref } from 'vue'
import { api, formatTime } from '~/api'

const logs = ref<AccessLog[]>([])
const loading = ref(false)

async function loadLogs() {
  loading.value = true
  try {
    logs.value = await api<AccessLog[]>('/api/logs')
  }
  finally {
    loading.value = false
  }
}

onMounted(loadLogs)
</script>

<template>
  <div class="workspace" v-loading="loading">
    <section class="panel">
      <div class="panel-title">
        访问日志
      </div>
      <p class="lede">
        查看文件下载、目录访问、上传、删除、分享和直链访问记录。
      </p>
      <el-table :data="logs" empty-text="暂无访问日志">
        <el-table-column prop="action" label="动作" width="120" />
        <el-table-column prop="path" label="路径" min-width="260" />
        <el-table-column prop="ip" label="IP" width="160" />
        <el-table-column prop="userAgent" label="客户端" min-width="220" show-overflow-tooltip />
        <el-table-column label="时间" width="160">
          <template #default="{ row }">
            {{ formatTime(row.createdAt) }}
          </template>
        </el-table-column>
      </el-table>
    </section>
  </div>
</template>
