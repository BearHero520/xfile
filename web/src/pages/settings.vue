<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { onMounted, reactive, ref } from 'vue'
import { api } from '~/api'

const loading = ref(false)
const form = reactive({
  siteName: 'XFile',
  rootName: '首页',
  publicIndex: 'enabled',
})

async function loadSettings() {
  loading.value = true
  try {
    const settings = await api<Record<string, string>>('/api/settings')
    form.siteName = settings.siteName || 'XFile'
    form.rootName = settings.rootName || '首页'
    form.publicIndex = settings.publicIndex || 'enabled'
  }
  finally {
    loading.value = false
  }
}

async function saveSettings() {
  await api('/api/settings', { method: 'PUT', body: JSON.stringify({
    siteName: form.siteName,
    rootName: form.rootName,
    publicIndex: form.publicIndex,
  }) })
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
        配置站点名称、根目录名称和公开索引。
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
        <el-form-item>
          <el-button type="primary" @click="saveSettings">
            保存设置
          </el-button>
        </el-form-item>
      </el-form>
    </section>
  </div>
</template>
