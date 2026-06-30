<script setup lang="ts">
import type { ShareDetail } from '~/api'
import { Document, Download, Folder, Lock, Share } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api, formatBytes, formatTime, shareDownloadUrl } from '~/api'

const route = useRoute()
const token = computed(() => String((route.params as Record<string, string | string[]>).token || ''))
const password = ref('')
const detail = ref<ShareDetail>()
const loading = ref(false)
const locked = ref(false)
const activeSharePath = ref('')

const currentPath = computed(() => detail.value?.currentPath || detail.value?.path || '')

const breadcrumbs = computed(() => {
  if (!detail.value)
    return []
  const rel = childPath(currentPath.value)
  const parts = rel ? rel.split('/') : []
  return [{ label: detail.value.path.split('/').pop() || detail.value.name, path: '' }].concat(
    parts.map((part, index) => ({ label: part, path: parts.slice(0, index + 1).join('/') })),
  )
})

async function loadShare() {
  loading.value = true
  locked.value = false
  try {
    const params = new URLSearchParams()
    if (password.value)
      params.set('password', password.value)
    if (activeSharePath.value)
      params.set('path', activeSharePath.value)
    const query = params.toString()
    const data = await api<ShareDetail>(`/api/public/shares/${encodeURIComponent(token.value)}${query ? `?${query}` : ''}`)
    detail.value = data
    activeSharePath.value = childPath(data.currentPath || data.path)
  }
  catch {
    locked.value = true
  }
  finally {
    loading.value = false
  }
}

function childPath(path: string) {
  if (!detail.value)
    return ''
  const prefix = `${detail.value.path}/`
  return path.startsWith(prefix) ? path.slice(prefix.length) : path
}

function openFolder(path: string) {
  activeSharePath.value = childPath(path)
  void loadShare()
}

function openBreadcrumb(path: string) {
  activeSharePath.value = path
  void loadShare()
}

function download(path = '') {
  window.open(shareDownloadUrl(token.value, password.value, path), '_blank')
}

function submitPassword() {
  if (!password.value.trim()) {
    ElMessage.warning('请输入分享密码')
    return
  }
  activeSharePath.value = ''
  void loadShare()
}

onMounted(loadShare)
</script>

<template>
  <main class="share-page" v-loading="loading">
    <section class="share-panel">
      <span class="brand-mark">XF</span>
      <template v-if="locked">
        <el-icon class="share-hero-icon"><Lock /></el-icon>
        <h1>受保护的分享</h1>
        <p class="lede">
          输入分享密码后查看文件详情。
        </p>
        <div class="share-password-row">
          <el-input
            v-model="password"
            type="password"
            size="large"
            placeholder="分享密码"
            show-password
            @keydown.enter="submitPassword"
          />
          <el-button type="primary" size="large" @click="submitPassword">
            查看
          </el-button>
        </div>
      </template>

      <template v-else-if="detail">
        <el-icon class="share-hero-icon">
          <Folder v-if="detail.type === 'folder'" />
          <Document v-else />
        </el-icon>
        <h1>{{ detail.name }}</h1>
        <p class="lede">
          {{ detail.type === 'folder' ? '文件夹分享' : '文件分享' }} /
          {{ formatBytes(detail.size) }} /
          {{ formatTime(detail.createdAt) }}
        </p>
        <div class="share-actions">
          <el-button v-if="detail.type === 'file'" type="primary" :icon="Download" @click="download()">
            下载文件
          </el-button>
          <el-tag v-if="detail.protected" type="warning">
            密码保护
          </el-tag>
          <el-tag v-if="detail.expiresAt" type="info">
            {{ detail.expiresAt }} 过期
          </el-tag>
        </div>

        <el-breadcrumb v-if="detail.type === 'folder'" class="share-breadcrumb" separator="/">
          <el-breadcrumb-item v-for="item in breadcrumbs" :key="item.path">
            <button class="crumb-button" @click="openBreadcrumb(item.path)">
              {{ item.label }}
            </button>
          </el-breadcrumb-item>
        </el-breadcrumb>

        <el-table v-if="detail.type === 'folder'" :data="detail.files || []" empty-text="文件夹为空">
          <el-table-column label="文件名" min-width="260">
            <template #default="{ row }">
              <button v-if="row.type === 'folder'" class="file-name" @click="openFolder(row.path)">
                <el-icon>
                  <Folder />
                </el-icon>
                <span>{{ row.name }}</span>
              </button>
              <span v-else class="file-name">
                <el-icon>
                  <Document />
                </el-icon>
                <span>{{ row.name }}</span>
              </span>
            </template>
          </el-table-column>
          <el-table-column label="大小" width="120">
            <template #default="{ row }">
              {{ formatBytes(row.size) }}
            </template>
          </el-table-column>
          <el-table-column label="修改时间" width="160">
            <template #default="{ row }">
              {{ formatTime(row.modifiedAt) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100" align="right">
            <template #default="{ row }">
              <el-button v-if="row.type === 'file'" text :icon="Download" @click="download(childPath(row.path))" />
            </template>
          </el-table-column>
        </el-table>
      </template>

      <template v-else>
        <el-icon class="share-hero-icon"><Share /></el-icon>
        <h1>分享不存在</h1>
        <p class="lede">
          链接可能已过期或被删除。
        </p>
      </template>
    </section>
  </main>
</template>
