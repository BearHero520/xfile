<script setup lang="ts">
import type { OnlyOfficeDocumentType } from '~/externalPreview'
import { Download, RefreshRight, Warning } from '@element-plus/icons-vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '~/api'
import {
  normalizeExternalPreviewBaseUrl,
  normalizeExternalPreviewProvider,
  onlyOfficeDocumentType,
} from '~/externalPreview'

interface OnlyOfficeEditor {
  destroyEditor?: () => void
}

interface OnlyOfficeEditorConfig {
  documentType: OnlyOfficeDocumentType
  document: {
    fileType: string
    key: string
    title: string
    url: string
    permissions: {
      download: boolean
      edit: boolean
      print: boolean
    }
  }
  editorConfig: {
    lang: string
    mode: 'view'
  }
  height: string
  type: 'desktop'
  width: string
}

declare global {
  interface Window {
    DocsAPI?: {
      DocEditor: new (elementId: string, config: OnlyOfficeEditorConfig) => OnlyOfficeEditor
    }
  }
}

const route = useRoute()
const loading = ref(false)
const errorMessage = ref('')
const editor = ref<OnlyOfficeEditor>()

const fileUrl = computed(() => queryValue('url'))
const fileName = computed(() => queryValue('name') || '文件预览')
const fileExt = computed(() => (queryValue('ext') || fileName.value.split('.').pop() || '').toLowerCase())
const documentKey = computed(() => queryValue('key') || `xfile-${Date.now().toString(36)}`)

function queryValue(name: string) {
  const value = route.query[name]
  if (Array.isArray(value))
    return value[0] || ''
  return typeof value === 'string' ? value : ''
}

function joinUrl(baseUrl: string, path: string) {
  return `${baseUrl.replace(/\/+$/, '')}/${path.replace(/^\/+/, '')}`
}

async function loadOnlyOfficeScript(src: string) {
  if (window.DocsAPI)
    return

  const existing = Array.from(document.scripts).find(script => script.src === src)
  existing?.remove()

  await new Promise<void>((resolve, reject) => {
    const script = document.createElement('script')
    script.src = src
    script.async = true
    script.addEventListener('load', () => resolve(), { once: true })
    script.addEventListener('error', () => reject(new Error('OnlyOffice API 加载失败')), { once: true })
    document.head.appendChild(script)
  })
}

async function renderPreview() {
  loading.value = true
  errorMessage.value = ''
  editor.value?.destroyEditor?.()
  editor.value = undefined

  try {
    if (!fileUrl.value)
      throw new Error('缺少文件地址')

    const settings = await api<Record<string, string>>('/api/settings')
    if (normalizeExternalPreviewProvider(settings.externalPreviewProvider) !== 'onlyoffice')
      throw new Error('OnlyOffice 预览未启用')

    const server = normalizeExternalPreviewBaseUrl(settings.externalPreviewBaseUrl)
    if (!server)
      throw new Error('OnlyOffice 服务地址未配置')

    const documentType = onlyOfficeDocumentType(fileExt.value)
    if (!documentType)
      throw new Error('当前文件类型不支持 OnlyOffice 预览')

    await loadOnlyOfficeScript(joinUrl(server, '/web-apps/apps/api/documents/api.js'))
    if (!window.DocsAPI)
      throw new Error('OnlyOffice API 不可用')

    await nextTick()
    editor.value = new window.DocsAPI.DocEditor('onlyoffice-editor', {
      documentType,
      document: {
        fileType: fileExt.value,
        key: documentKey.value,
        title: fileName.value,
        url: fileUrl.value,
        permissions: {
          download: true,
          edit: false,
          print: true,
        },
      },
      editorConfig: {
        lang: 'zh-CN',
        mode: 'view',
      },
      height: '100%',
      type: 'desktop',
      width: '100%',
    })
  }
  catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'OnlyOffice 预览失败'
  }
  finally {
    loading.value = false
  }
}

function openOriginal() {
  if (fileUrl.value)
    window.open(fileUrl.value, '_blank', 'noopener')
}

onMounted(renderPreview)

onBeforeUnmount(() => {
  editor.value?.destroyEditor?.()
})
</script>

<template>
  <div v-loading="loading" class="workspace external-preview-page">
    <main class="panel external-preview-shell">
      <header class="external-preview-toolbar">
        <div class="external-preview-title">
          <span>OnlyOffice</span>
          <strong>{{ fileName }}</strong>
        </div>
        <div class="external-preview-actions">
          <el-tag effect="plain" type="success">
            只读
          </el-tag>
          <el-button :icon="RefreshRight" @click="renderPreview">
            刷新
          </el-button>
          <el-button :icon="Download" @click="openOriginal">
            原文件
          </el-button>
        </div>
      </header>

      <div v-if="errorMessage" class="external-preview-empty">
        <el-empty :description="errorMessage">
          <template #image>
            <el-icon><Warning /></el-icon>
          </template>
        </el-empty>
      </div>
      <div v-show="!errorMessage" id="onlyoffice-editor" class="external-preview-editor" />
    </main>
  </div>
</template>
