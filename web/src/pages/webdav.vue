<script setup lang="ts">
import {
  CircleCheck,
  Connection,
  CopyDocument,
  Folder,
  Lock,
  Warning,
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '~/api'

const loading = ref(false)
const saving = ref(false)
const form = reactive({
  webdav: 'disabled',
  webdavMountPath: '/dav',
  webdavReadOnly: 'disabled',
  webdavAllowAnonymous: 'disabled',
  webdavUsername: '',
  webdavPassword: '',
})

const origin = computed(() => window.location.origin)
const davUrl = computed(() => `${origin.value}${normalizedMountPath.value}`)
const normalizedMountPath = computed(() => {
  const value = form.webdavMountPath.trim() || '/dav'
  return value.startsWith('/') ? value : `/${value}`
})
const isEnabled = computed(() => form.webdav === 'enabled')

async function loadSettings() {
  loading.value = true
  try {
    const settings = await api<Record<string, string>>('/api/settings')
    form.webdav = settings.webdav || 'disabled'
    form.webdavMountPath = settings.webdavMountPath || '/dav'
    form.webdavReadOnly = settings.webdavReadOnly || 'disabled'
    form.webdavAllowAnonymous = settings.webdavAllowAnonymous || 'disabled'
    form.webdavUsername = settings.webdavUsername || ''
    form.webdavPassword = settings.webdavPassword || ''
  }
  finally {
    loading.value = false
  }
}

async function saveSettings() {
  saving.value = true
  try {
    await api('/api/settings', {
      method: 'PUT',
      body: JSON.stringify({
        webdav: form.webdav,
        webdavMountPath: normalizedMountPath.value,
        webdavReadOnly: form.webdavReadOnly,
        webdavAllowAnonymous: form.webdavAllowAnonymous,
        webdavUsername: form.webdavUsername.trim(),
        webdavPassword: form.webdavPassword,
      }),
    })
    form.webdavMountPath = normalizedMountPath.value
    ElMessage.success('WebDAV 配置已保存')
  }
  finally {
    saving.value = false
  }
}

async function copyText(value: string) {
  await navigator.clipboard.writeText(value)
  ElMessage.success('已复制')
}

onMounted(loadSettings)
</script>

<template>
  <div class="workspace" v-loading="loading">
    <section class="overview-band">
      <div>
        <p class="eyebrow">
          WebDAV access
        </p>
        <h1>WebDAV</h1>
        <p class="lede">
          维护 WebDAV 开关、挂载路径和客户端连接信息。
        </p>
      </div>
      <el-tag :type="isEnabled ? 'warning' : 'info'" effect="plain">
        {{ isEnabled ? '服务已启用' : '未启用' }}
      </el-tag>
    </section>

    <section class="content-grid">
      <main class="panel webdav-panel">
        <div class="panel-heading">
          <div>
            <div class="panel-title">
              <el-icon><Connection /></el-icon>
              <span>服务配置</span>
            </div>
            <p class="lede">
              WebDAV 使用独立账号密码或匿名访问策略，客户端通过挂载地址连接本地文件存储。
            </p>
          </div>
        </div>

        <el-form class="webdav-form" label-position="top" :model="form">
          <div class="access-form-grid">
            <el-form-item label="WebDAV 开关">
              <el-switch v-model="form.webdav" active-value="enabled" inactive-value="disabled" />
            </el-form-item>
            <el-form-item label="只读模式">
              <el-switch v-model="form.webdavReadOnly" active-value="enabled" inactive-value="disabled" />
            </el-form-item>
            <el-form-item label="匿名访问">
              <el-switch v-model="form.webdavAllowAnonymous" active-value="enabled" inactive-value="disabled" />
            </el-form-item>
          </div>

          <div class="access-form-grid">
            <el-form-item label="挂载路径">
              <el-input v-model="form.webdavMountPath" placeholder="/dav" />
            </el-form-item>
            <el-form-item label="账号名">
              <el-input v-model="form.webdavUsername" placeholder="WebDAV 账号" />
            </el-form-item>
            <el-form-item label="密码">
              <el-input v-model="form.webdavPassword" type="password" show-password autocomplete="off" />
            </el-form-item>
          </div>

          <el-form-item label="连接地址">
            <el-input :model-value="davUrl" readonly>
              <template #append>
                <el-button :icon="CopyDocument" @click="copyText(davUrl)" />
              </template>
            </el-input>
          </el-form-item>

          <el-form-item>
            <el-button type="primary" :loading="saving" @click="saveSettings">
              保存配置
            </el-button>
          </el-form-item>
        </el-form>
      </main>

      <aside class="side-stack">
        <section class="panel">
          <div class="panel-title">
            <el-icon><Warning /></el-icon>
            <span>实现状态</span>
          </div>
          <div class="status-list">
            <div class="status-row done">
              <el-icon><CircleCheck /></el-icon>
              <span>本地文件存储已可复用</span>
            </div>
            <div class="status-row done">
              <el-icon><CircleCheck /></el-icon>
              <span>挂载路径、只读策略和匿名访问可配置</span>
            </div>
            <div class="status-row done">
              <el-icon><CircleCheck /></el-icon>
              <span>WebDAV 协议处理器已接入</span>
            </div>
            <div class="status-row done">
              <el-icon><Lock /></el-icon>
              <span>独立账号密码已启用</span>
            </div>
          </div>
        </section>

        <section class="panel">
          <div class="panel-title">
            <el-icon><Folder /></el-icon>
            <span>客户端信息</span>
          </div>
          <div class="client-command">
            <span>macOS / Windows / iOS</span>
            <strong>{{ davUrl }}</strong>
            <el-button text :icon="CopyDocument" @click="copyText(davUrl)">
              复制地址
            </el-button>
          </div>
        </section>
      </aside>
    </section>
  </div>
</template>
