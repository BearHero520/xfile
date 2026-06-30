<script setup lang="ts">
import type { FileEntry } from '~/api'
import {
  Document,
  Download,
  Folder,
  Link,
  Refresh,
  Search,
  Share,
  View,
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api, fileUrl, formatBytes, formatTime } from '~/api'

const router = useRouter()
const keyword = ref('')
const results = ref<FileEntry[]>([])
const loading = ref(false)
const searched = ref(false)

let searchTimer: ReturnType<typeof window.setTimeout> | undefined

const folders = computed(() => results.value.filter(file => file.type === 'folder').length)
const files = computed(() => results.value.filter(file => file.type === 'file').length)

watch(keyword, () => {
  if (searchTimer)
    window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(runSearch, 280)
})

async function runSearch() {
  const term = keyword.value.trim()
  searched.value = Boolean(term)
  if (!term) {
    results.value = []
    return
  }
  loading.value = true
  try {
    results.value = await api<FileEntry[]>(`/api/files/search?q=${encodeURIComponent(term)}&limit=200`)
  }
  catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '搜索失败')
  }
  finally {
    loading.value = false
  }
}

function parentPath(path: string) {
  const parts = path.split('/').filter(Boolean)
  parts.pop()
  return parts.join('/')
}

function openLocation(file: FileEntry) {
  const path = file.type === 'folder' ? file.path : parentPath(file.path)
  void router.push({ path: '/', query: path ? { path } : {} })
}

function openFile(file: FileEntry) {
  if (file.type === 'folder') {
    openLocation(file)
    return
  }
  window.open(fileUrl(file.path), '_blank')
}

async function createShare(file: FileEntry) {
  const { value: password } = await ElMessageBox.prompt('可选：设置分享密码，留空表示公开', '创建分享', {
    inputPlaceholder: '分享密码',
  })
  await api('/api/shares', {
    method: 'POST',
    body: JSON.stringify({ path: file.path, password }),
  })
  ElMessage.success('分享链接已生成')
}

async function createDirectLink(file: FileEntry) {
  await api('/api/direct-links', {
    method: 'POST',
    body: JSON.stringify({ path: file.path }),
  })
  ElMessage.success('直链已生成')
}
</script>

<template>
  <div class="workspace">
    <section class="search-hero-panel">
      <div>
        <p class="eyebrow">
          Global file search
        </p>
        <h1>全局搜索</h1>
        <p class="lede">
          按文件名或路径查找整个存储空间里的文件和文件夹。
        </p>
      </div>
      <el-button :icon="Refresh" @click="runSearch">
        刷新结果
      </el-button>
    </section>

    <section class="search-console" v-loading="loading">
      <div class="search-command">
        <el-input
          v-model="keyword"
          size="large"
          clearable
          autofocus
          :prefix-icon="Search"
          placeholder="输入文件名、目录名或路径"
          @keydown.enter="runSearch"
        />
      </div>

      <div class="search-stats">
        <span>{{ results.length }} 条结果</span>
        <span>{{ folders }} 个文件夹</span>
        <span>{{ files }} 个文件</span>
      </div>

      <el-table
        class="file-table"
        :data="results"
        :empty-text="searched ? '未找到匹配文件' : '输入关键词开始搜索'"
      >
        <el-table-column label="名称" min-width="260">
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
        <el-table-column label="路径" min-width="280">
          <template #default="{ row }">
            <button class="path-button" @click="openLocation(row)">
              {{ row.path }}
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
        <el-table-column label="操作" width="230" align="right">
          <template #default="{ row }">
            <el-button text :icon="View" title="打开位置" @click="openLocation(row)" />
            <el-button v-if="row.type === 'file'" text :icon="Download" title="下载" @click="openFile(row)" />
            <el-button text :icon="Share" title="分享" @click="createShare(row)" />
            <el-button text :icon="Link" title="直链" @click="createDirectLink(row)" />
          </template>
        </el-table-column>
      </el-table>
    </section>
  </div>
</template>
