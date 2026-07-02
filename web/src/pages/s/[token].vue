<script setup lang="ts">
import type { FileEntry, ShareDetail } from '~/api'
import { ArrowRight, Document, Download, Folder, Lock, Share } from '@element-plus/icons-vue'
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
const errorText = ref('')

const currentPath = computed(() => detail.value?.currentPath || detail.value?.path || '')
const currentChildPath = computed(() => childPath(currentPath.value))
const rootLabel = computed(() => {
  if (!detail.value)
    return '分享文件'
  return detail.value.path.split('/').filter(Boolean).pop() || detail.value.name || '分享文件'
})
const fileCount = computed(() => detail.value?.files?.filter(file => file.type === 'file').length || 0)
const folderCount = computed(() => detail.value?.files?.filter(file => file.type === 'folder').length || 0)

const breadcrumbs = computed(() => {
  if (!detail.value)
    return []
  const rel = currentChildPath.value
  const parts = rel ? rel.split('/') : []
  return [{ label: rootLabel.value, path: '' }].concat(
    parts.map((part, index) => ({ label: part, path: parts.slice(0, index + 1).join('/') })),
  )
})

async function loadShare() {
  loading.value = true
  locked.value = false
  errorText.value = ''
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
  catch (error) {
    locked.value = true
    detail.value = undefined
    errorText.value = error instanceof Error ? error.message : '分享暂时无法访问'
  }
  finally {
    loading.value = false
  }
}

function childPath(path: string) {
  if (!detail.value)
    return ''
  if (path === detail.value.path)
    return ''
  const prefix = `${detail.value.path}/`
  return path.startsWith(prefix) ? path.slice(prefix.length) : path
}

function parentPath(path: string) {
  const parts = path.split('/').filter(Boolean)
  parts.pop()
  return parts.join('/')
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

function downloadFile(file: FileEntry) {
  download(childPath(file.path))
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
  <main v-loading="loading" class="share-page">
    <section class="share-shell">
      <header class="share-topbar">
        <span class="brand-mark">XF</span>
        <span class="share-topbar-text">XFile Share</span>
      </header>

      <template v-if="locked">
        <section class="share-state-panel">
          <el-icon class="share-hero-icon">
            <Lock />
          </el-icon>
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
          <p v-if="password && errorText" class="share-error">
            {{ errorText }}
          </p>
        </section>
      </template>

      <template v-else-if="detail">
        <section class="share-hero">
          <div class="share-title-row">
            <el-icon class="share-hero-icon">
              <Folder v-if="detail.type === 'folder'" />
              <Document v-else />
            </el-icon>
            <div>
              <p class="eyebrow">
                {{ detail.type === 'folder' ? 'Folder share' : 'File share' }}
              </p>
              <h1>{{ detail.name }}</h1>
              <p class="lede">
                创建于 {{ formatTime(detail.createdAt) }}
              </p>
              <p v-if="detail.description" class="share-description">
                {{ detail.description }}
              </p>
            </div>
          </div>
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
        </section>

        <section v-if="detail.type === 'folder'" class="share-browser">
          <div class="share-browser-toolbar">
            <el-breadcrumb class="share-breadcrumb" separator="/">
              <el-breadcrumb-item v-for="item in breadcrumbs" :key="item.path">
                <button class="crumb-button" @click="openBreadcrumb(item.path)">
                  {{ item.label }}
                </button>
              </el-breadcrumb-item>
            </el-breadcrumb>
            <div class="share-browser-summary">
              <span>{{ folderCount }} 个文件夹</span>
              <span>{{ fileCount }} 个文件</span>
            </div>
          </div>

          <button v-if="currentChildPath" class="share-up-row" @click="openBreadcrumb(parentPath(currentChildPath))">
            <el-icon><ArrowRight /></el-icon>
            <span>返回上一级</span>
          </button>

          <el-table class="share-file-table" :data="detail.files || []" empty-text="文件夹为空">
            <el-table-column label="名称" min-width="260">
              <template #default="{ row }">
                <div class="file-title-cell">
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
                  <p v-if="row.description" class="file-description">
                    {{ row.description }}
                  </p>
                </div>
              </template>
            </el-table-column>
            <el-table-column class-name="share-secondary-column" label="大小" width="120">
              <template #default="{ row }">
                {{ formatBytes(row.size) }}
              </template>
            </el-table-column>
            <el-table-column class-name="share-secondary-column" label="修改时间" width="160">
              <template #default="{ row }">
                {{ formatTime(row.modifiedAt) }}
              </template>
            </el-table-column>
            <el-table-column label="操作" width="112" align="right">
              <template #default="{ row }">
                <el-button v-if="row.type === 'file'" text :icon="Download" title="下载" @click="downloadFile(row)" />
                <el-button v-else text :icon="ArrowRight" title="打开" @click="openFolder(row.path)" />
              </template>
            </el-table-column>
          </el-table>
        </section>
      </template>

      <template v-else>
        <section class="share-state-panel">
          <el-icon class="share-hero-icon">
            <Share />
          </el-icon>
          <h1>分享不存在</h1>
          <p class="lede">
            链接可能已过期或被删除。
          </p>
        </section>
      </template>
    </section>
  </main>
</template>
