<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '~/api'

const loading = ref(false)
const saving = ref(false)
const form = reactive({
  siteName: 'XFile',
  rootName: '首页',
  publicIndex: 'enabled',
  externalPreviewProvider: 'disabled',
  externalPreviewBaseUrl: '',
  externalPreviewTemplate: '',
})

const externalPreviewProviders = [
  { label: '关闭', value: 'disabled' },
  { label: 'kkFileView', value: 'kkfileview' },
  { label: 'OnlyOffice', value: 'onlyoffice' },
]

const externalPreviewEnabled = computed(() => form.externalPreviewProvider !== 'disabled')
const externalPreviewState = computed(() => {
  if (!externalPreviewEnabled.value)
    return '未启用'
  return externalPreviewProviders.find(item => item.value === form.externalPreviewProvider)?.label || '已启用'
})
const externalPreviewTemplatePlaceholder = computed(() =>
  form.externalPreviewProvider === 'onlyoffice'
    ? '/external-preview?url={encodedUrl}&name={encodedName}&ext={ext}&key={encodedKey}'
    : '{server}/onlinePreview?url={base64Url}',
)

async function loadSettings() {
  loading.value = true
  try {
    const settings = await api<Record<string, string>>('/api/settings')
    form.siteName = settings.siteName || 'XFile'
    form.rootName = settings.rootName || '首页'
    form.publicIndex = settings.publicIndex || 'enabled'
    form.externalPreviewProvider = settings.externalPreviewProvider || 'disabled'
    form.externalPreviewBaseUrl = settings.externalPreviewBaseUrl || ''
    form.externalPreviewTemplate = settings.externalPreviewTemplate || ''
  }
  finally {
    loading.value = false
  }
}

async function saveSettings() {
  saving.value = true
  try {
    await api('/api/settings', { method: 'PUT', body: JSON.stringify({
      siteName: form.siteName,
      rootName: form.rootName,
      publicIndex: form.publicIndex,
      externalPreviewProvider: form.externalPreviewProvider,
      externalPreviewBaseUrl: form.externalPreviewBaseUrl.trim(),
      externalPreviewTemplate: form.externalPreviewTemplate.trim(),
    }) })
    ElMessage.success('设置已保存')
  }
  finally {
    saving.value = false
  }
}

onMounted(loadSettings)
</script>

<template>
  <div v-loading="loading" class="workspace">
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
        <el-divider />
        <div class="settings-section-title">
          <span>外部预览</span>
          <el-tag :type="externalPreviewEnabled ? 'success' : 'info'" effect="plain">
            {{ externalPreviewState }}
          </el-tag>
        </div>
        <div class="settings-preview-grid">
          <el-form-item label="预览服务">
            <el-select v-model="form.externalPreviewProvider" class="full-control">
              <el-option
                v-for="provider in externalPreviewProviders"
                :key="provider.value"
                :label="provider.label"
                :value="provider.value"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="服务地址">
            <el-input
              v-model="form.externalPreviewBaseUrl"
              :disabled="!externalPreviewEnabled"
              placeholder="https://preview.example.com"
            />
          </el-form-item>
        </div>
        <el-form-item label="URL 模板">
          <el-input
            v-model="form.externalPreviewTemplate"
            type="textarea"
            :rows="3"
            :disabled="!externalPreviewEnabled"
            :placeholder="externalPreviewTemplatePlaceholder"
          />
          <p class="form-help-text">
            可用占位符：{server}、{url}、{encodedUrl}、{base64Url}、{encodedName}、{ext}、{key}
          </p>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="saveSettings">
            保存设置
          </el-button>
        </el-form-item>
      </el-form>
    </section>
  </div>
</template>
