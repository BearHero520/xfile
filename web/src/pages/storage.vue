<script setup lang="ts">
import type { StorageSource } from '~/api'
import {
  Connection,
  Delete,
  Folder,
  Plus,
  UploadFilled,
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '~/api'

type SourceType = 'local' | 's3' | 'webdav' | 'aliyun_oss' | 'tencent_cos'

interface StorageSourceInput {
  name: string
  key: string
  type: SourceType
  rootPath: string
  hiddenPaths: string
  blockedPaths: string
  s3Endpoint: string
  s3Bucket: string
  s3Region: string
  s3AccessKey: string
  s3SecretKey: string
  s3Prefix: string
  s3Secure: boolean
  webdavUrl: string
  webdavUsername: string
  webdavPassword: string
  webdavRoot: string
  public: boolean
  enabled: boolean
  orderNum: number
}

const loading = ref(false)
const saving = ref(false)
const sources = ref<StorageSource[]>([])
const editingId = ref<number | null>(null)

const providers = [
  { label: '本地存储', value: 'local', icon: Folder, ready: true },
  { label: 'S3 / MinIO', value: 's3', icon: UploadFilled, ready: true },
  { label: 'WebDAV', value: 'webdav', icon: Connection, ready: true },
  { label: '阿里云 OSS', value: 'aliyun_oss', icon: UploadFilled, ready: true },
  { label: '腾讯云 COS', value: 'tencent_cos', icon: UploadFilled, ready: true },
] as const

const form = reactive<StorageSourceInput>({
  name: '本地文件',
  key: 'local',
  type: 'local',
  rootPath: 'data/files',
  hiddenPaths: '',
  blockedPaths: '',
  s3Endpoint: '',
  s3Bucket: '',
  s3Region: '',
  s3AccessKey: '',
  s3SecretKey: '',
  s3Prefix: '',
  s3Secure: true,
  webdavUrl: '',
  webdavUsername: '',
  webdavPassword: '',
  webdavRoot: '',
  public: true,
  enabled: true,
  orderNum: 0,
})

const activeProvider = computed(() => providers.find(item => item.value === form.type) || providers[0])
const canEnable = computed(() => activeProvider.value.ready)
const editingSource = computed(() => sources.value.find(item => item.id === editingId.value))

function nextSourceKey(type: SourceType) {
  const prefixMap: Record<SourceType, string> = {
    local: 'local',
    s3: 's3',
    webdav: 'webdav',
    aliyun_oss: 'aliyun',
    tencent_cos: 'tencent',
  }
  const prefix = prefixMap[type]
  let index = sources.value.filter(source => source.type === type).length + 1
  let key = `${prefix}-${index}`
  while (sources.value.some(source => source.key === key)) {
    index++
    key = `${prefix}-${index}`
  }
  return key
}

function defaultSourceName(type: SourceType) {
  const provider = providers.find(item => item.value === type)
  const count = sources.value.filter(source => source.type === type).length + 1
  return `${provider?.label || '存储源'} ${count}`
}

function resetForm(type: SourceType = 'local') {
  const provider = providers.find(item => item.value === type) || providers[0]
  editingId.value = null
  Object.assign(form, {
    name: defaultSourceName(type),
    key: nextSourceKey(type),
    type,
    rootPath: type === 'local' ? 'data/files' : '',
    hiddenPaths: '',
    blockedPaths: '',
    s3Endpoint: '',
    s3Bucket: '',
    s3Region: '',
    s3AccessKey: '',
    s3SecretKey: '',
    s3Prefix: '',
    s3Secure: true,
    webdavUrl: '',
    webdavUsername: '',
    webdavPassword: '',
    webdavRoot: '',
    public: true,
    enabled: provider.ready,
    orderNum: sources.value.length,
  } satisfies StorageSourceInput)
}

function editSource(source: StorageSource) {
  const s3Config = parseS3Config(source.rootPath || '')
  const webdavConfig = parseWebDAVConfig(source.rootPath || '')
  editingId.value = source.id
  Object.assign(form, {
    name: source.name,
    key: source.key,
    type: source.type as SourceType,
    rootPath: source.rootPath || '',
    hiddenPaths: source.hiddenPaths || '',
    blockedPaths: source.blockedPaths || '',
    s3Endpoint: s3Config.endpoint,
    s3Bucket: s3Config.bucket,
    s3Region: s3Config.region,
    s3AccessKey: s3Config.accessKey,
    s3SecretKey: s3Config.secretKey,
    s3Prefix: s3Config.prefix,
    s3Secure: s3Config.secure,
    webdavUrl: webdavConfig.url,
    webdavUsername: webdavConfig.username,
    webdavPassword: webdavConfig.password,
    webdavRoot: webdavConfig.root,
    public: source.public,
    enabled: source.enabled,
    orderNum: source.orderNum,
  } satisfies StorageSourceInput)
}

function objectStorageType(type: SourceType) {
  return type === 's3' || type === 'aliyun_oss' || type === 'tencent_cos'
}

function parseS3Config(value: string) {
  try {
    const parsed = JSON.parse(value || '{}') as Partial<Record<string, string | boolean>>
    return {
      endpoint: typeof parsed.endpoint === 'string' ? parsed.endpoint : '',
      bucket: typeof parsed.bucket === 'string' ? parsed.bucket : '',
      region: typeof parsed.region === 'string' ? parsed.region : '',
      accessKey: typeof parsed.accessKey === 'string' ? parsed.accessKey : '',
      secretKey: typeof parsed.secretKey === 'string' ? parsed.secretKey : '',
      prefix: typeof parsed.prefix === 'string' ? parsed.prefix : '',
      secure: typeof parsed.secure === 'boolean' ? parsed.secure : true,
    }
  }
  catch {
    return { endpoint: '', bucket: '', region: '', accessKey: '', secretKey: '', prefix: '', secure: true }
  }
}

function parseWebDAVConfig(value: string) {
  try {
    const parsed = JSON.parse(value || '{}') as Partial<Record<string, string>>
    return {
      url: typeof parsed.url === 'string' ? parsed.url : '',
      username: typeof parsed.username === 'string' ? parsed.username : '',
      password: typeof parsed.password === 'string' ? parsed.password : '',
      root: typeof parsed.root === 'string' ? parsed.root : '',
    }
  }
  catch {
    return { url: '', username: '', password: '', root: '' }
  }
}

function buildRootPath() {
  if (objectStorageType(form.type)) {
    return JSON.stringify({
      endpoint: form.s3Endpoint.trim(),
      bucket: form.s3Bucket.trim(),
      region: form.s3Region.trim(),
      accessKey: form.s3AccessKey.trim(),
      secretKey: form.s3SecretKey.trim(),
      prefix: form.s3Prefix.trim(),
      secure: form.s3Secure,
    })
  }
  if (form.type === 'webdav') {
    return JSON.stringify({
      url: form.webdavUrl.trim(),
      username: form.webdavUsername.trim(),
      password: form.webdavPassword.trim(),
      root: form.webdavRoot.trim(),
    })
  }
  return form.rootPath
}

async function loadSources() {
  loading.value = true
  try {
    sources.value = await api<StorageSource[]>('/api/storage-sources')
    if (editingId.value !== null && !sources.value.some(source => source.id === editingId.value))
      resetForm()
    else if (sources.value.length && editingId.value === null)
      editSource(sources.value[0])
  }
  finally {
    loading.value = false
  }
}

async function saveSource() {
  saving.value = true
  try {
    const payload = { ...form, rootPath: buildRootPath(), enabled: canEnable.value ? form.enabled : false }
    if (editingId.value === null) {
      const source = await api<StorageSource>('/api/storage-sources', {
        method: 'POST',
        body: JSON.stringify(payload),
      })
      ElMessage.success('存储源已创建')
      await loadSources()
      editSource(source)
    }
    else {
      const source = await api<StorageSource>(`/api/storage-sources/${editingId.value}`, {
        method: 'PATCH',
        body: JSON.stringify(payload),
      })
      ElMessage.success('存储源已保存')
      await loadSources()
      editSource(source)
    }
  }
  catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存失败')
  }
  finally {
    saving.value = false
  }
}

async function removeSource(source: StorageSource) {
  await ElMessageBox.confirm(`确认删除存储源 ${source.name}？`, '删除存储源')
  await api(`/api/storage-sources/${source.id}`, { method: 'DELETE' })
  ElMessage.success('存储源已删除')
  editingId.value = null
  await loadSources()
  if (!sources.value.length)
    resetForm()
}

function onTypeChange() {
  if (!canEnable.value)
    form.enabled = false
  else if (editingId.value === null)
    form.enabled = true
  if (editingId.value === null) {
    form.name = defaultSourceName(form.type)
    form.key = nextSourceKey(form.type)
  }
  if (form.type === 'local')
    form.rootPath = form.rootPath || 'data/files'
  else if (!objectStorageType(form.type) && form.type !== 'webdav')
    form.rootPath = ''
}

onMounted(loadSources)
</script>

<template>
  <div v-loading="loading" class="workspace">
    <section class="overview-band">
      <div>
        <p class="eyebrow">
          Storage sources
        </p>
        <h1>存储源管理</h1>
        <p class="lede">
          管理公开入口和后台文件管理使用的存储源实例。当前可读写适配器为本地存储、S3 / MinIO、WebDAV、阿里云 OSS 和腾讯云 COS。
        </p>
      </div>
      <el-button type="primary" :icon="Plus" @click="resetForm('local')">
        新建存储源
      </el-button>
    </section>

    <section class="storage-source-layout">
      <aside class="panel storage-provider-list">
        <div class="storage-list-section">
          <span>新建实例</span>
        </div>
        <div class="storage-create-grid">
          <button
            v-for="provider in providers"
            :key="provider.value"
            type="button"
            :disabled="!provider.ready"
            @click="resetForm(provider.value)"
          >
            <el-icon><component :is="provider.icon" /></el-icon>
            <span>{{ provider.label }}</span>
          </button>
        </div>

        <div class="storage-list-section">
          <span>已有实例</span>
          <el-tag size="small" effect="plain">
            {{ sources.length }}
          </el-tag>
        </div>
        <button
          v-for="source in sources"
          :key="source.id"
          type="button"
          :class="{ active: editingId === source.id }"
          @click="editSource(source)"
        >
          <el-icon><component :is="providers.find(item => item.value === source.type)?.icon || Folder" /></el-icon>
          <span>
            <strong>{{ source.name }}</strong>
            <small>{{ source.key }} / {{ source.typeLabel }}</small>
          </span>
          <el-tag size="small" :type="source.enabled ? 'success' : 'info'" effect="plain">
            {{ source.enabled ? '启用' : '停用' }}
          </el-tag>
        </button>
      </aside>

      <main class="panel storage-config-panel">
        <div class="panel-title">
          <el-icon><component :is="activeProvider.icon" /></el-icon>
          <span>{{ editingId === null ? '新建存储源' : '编辑存储源' }}</span>
        </div>

        <el-alert
          v-if="!activeProvider.ready"
          class="storage-adapter-alert"
          type="warning"
          show-icon
          :closable="false"
          title="该类型的远端适配器尚未接入，当前只能保存配置草稿，不能启用为可浏览源。"
        />

        <el-form label-position="top" :model="form">
          <div class="access-form-grid">
            <el-form-item label="名称">
              <el-input v-model="form.name" placeholder="例如：公开资料" />
            </el-form-item>
            <el-form-item label="Key">
              <el-input v-model="form.key" placeholder="例如：public-docs" />
            </el-form-item>
          </div>

          <div class="access-form-grid">
            <el-form-item label="类型">
              <el-select v-model="form.type" @change="onTypeChange">
                <el-option
                  v-for="provider in providers"
                  :key="provider.value"
                  :label="provider.label"
                  :value="provider.value"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="排序">
              <el-input-number v-model="form.orderNum" :min="0" :step="1" controls-position="right" />
            </el-form-item>
          </div>

          <el-form-item v-if="form.type === 'local'" label="本地根目录">
            <el-input v-model="form.rootPath" placeholder="data/files" />
          </el-form-item>

          <template v-if="objectStorageType(form.type)">
            <div class="access-form-grid">
              <el-form-item label="Endpoint">
                <el-input v-model="form.s3Endpoint" placeholder="例如：play.min.io、oss-cn-hangzhou.aliyuncs.com、cos.ap-guangzhou.myqcloud.com" />
              </el-form-item>
              <el-form-item label="Bucket">
                <el-input v-model="form.s3Bucket" placeholder="bucket 名称" />
              </el-form-item>
            </div>
            <div class="access-form-grid">
              <el-form-item label="Region">
                <el-input v-model="form.s3Region" placeholder="例如：us-east-1，可留空" />
              </el-form-item>
              <el-form-item label="Prefix">
                <el-input v-model="form.s3Prefix" placeholder="例如：public/files，可留空" />
              </el-form-item>
            </div>
            <div class="access-form-grid">
              <el-form-item label="Access Key">
                <el-input v-model="form.s3AccessKey" autocomplete="off" />
              </el-form-item>
              <el-form-item label="Secret Key">
                <el-input v-model="form.s3SecretKey" type="password" show-password autocomplete="off" />
              </el-form-item>
            </div>
            <el-form-item label="HTTPS">
              <el-switch v-model="form.s3Secure" />
            </el-form-item>
          </template>

          <template v-if="form.type === 'webdav'">
            <el-form-item label="WebDAV URL">
              <el-input v-model="form.webdavUrl" placeholder="例如：https://example.com/dav" />
            </el-form-item>
            <div class="access-form-grid">
              <el-form-item label="用户名">
                <el-input v-model="form.webdavUsername" autocomplete="off" />
              </el-form-item>
              <el-form-item label="密码">
                <el-input v-model="form.webdavPassword" type="password" show-password autocomplete="off" />
              </el-form-item>
            </div>
            <el-form-item label="根路径">
              <el-input v-model="form.webdavRoot" placeholder="例如：public/files，可留空" />
            </el-form-item>
          </template>

          <div class="access-form-grid">
            <el-form-item label="在公开首页显示">
              <el-switch v-model="form.public" />
            </el-form-item>
            <el-form-item label="启用存储源">
              <el-switch v-model="form.enabled" :disabled="!canEnable" />
            </el-form-item>
          </div>

          <el-form-item label="公开隐藏路径">
            <el-input
              v-model="form.hiddenPaths"
              type="textarea"
              :rows="4"
              placeholder="一行一个路径，例如：private&#10;tmp/cache&#10;填写目录会隐藏其下所有内容，暂不支持通配符"
            />
          </el-form-item>

          <el-form-item label="公开禁止访问路径">
            <el-input
              v-model="form.blockedPaths"
              type="textarea"
              :rows="4"
              placeholder="一行一个路径，例如：secret&#10;internal/reports&#10;公开访问这些路径会被直接拒绝"
            />
          </el-form-item>

          <el-form-item>
            <div class="storage-form-actions">
              <el-button type="primary" :loading="saving" @click="saveSource">
                {{ editingId === null ? '创建存储源' : '保存存储源' }}
              </el-button>
              <el-button @click="resetForm()">
                重置
              </el-button>
              <el-button v-if="editingSource" type="danger" plain :icon="Delete" @click="removeSource(editingSource)">
                删除
              </el-button>
            </div>
          </el-form-item>
        </el-form>
      </main>
    </section>
  </div>
</template>
