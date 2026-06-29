<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { onMounted, reactive, ref } from 'vue'
import { api } from '~/api'

const loading = ref(false)
const form = reactive({
  siteName: 'XFile',
  rootName: '首页',
  publicIndex: 'enabled',
  webdav: 'enabled',
  allowUpload: 'enabled',
  maxUploadMB: '512',
})

async function loadSettings() {
  loading.value = true
  try {
    Object.assign(form, await api<Record<string, string>>('/api/settings'))
  }
  finally {
    loading.value = false
  }
}

async function saveSettings() {
  await api('/api/settings', { method: 'PUT', body: JSON.stringify(form) })
  ElMessage.success('设置已保存')
}

onMounted(loadSettings)
</script>

<template>
  <div class="workspace" v-loading="loading">
    <section class="panel settings-panel">
      <div class="panel-title">
        系统设置
      </div>
      <p class="lede">
        配置站点名称、根目录名称、公开索引、WebDAV 和上传策略。
      </p>
      <el-form label-width="120px" :model="form">
        <el-form-item label="站点名称">
          <el-input v-model="form.siteName" />
        </el-form-item>
        <el-form-item label="根目录名称">
          <el-input v-model="form.rootName" />
        </el-form-item>
        <el-form-item label="公开索引">
          <el-switch v-model="form.publicIndex" active-value="enabled" inactive-value="disabled" />
        </el-form-item>
        <el-form-item label="WebDAV">
          <el-switch v-model="form.webdav" active-value="enabled" inactive-value="disabled" />
        </el-form-item>
        <el-form-item label="允许上传">
          <el-switch v-model="form.allowUpload" active-value="enabled" inactive-value="disabled" />
        </el-form-item>
        <el-form-item label="上传上限 MB">
          <el-input v-model="form.maxUploadMB" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="saveSettings">
            保存设置
          </el-button>
        </el-form-item>
      </el-form>
    </section>
  </div>
</template>
