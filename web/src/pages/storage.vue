<script setup lang="ts">
import type { StorageSource } from '~/api'
import {
  Connection,
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
  { label: 'S3 / MinIO', value: 's3', icon: UploadFilled, ready: false },
  { label: 'WebDAV', value: 'webdav', icon: Connection, ready: false },
  { label: '阿里云 OSS', value: 'aliyun_oss', icon: UploadFilled, ready: false },
  { label: '腾讯云 COS', value: 'tencent_cos', icon: UploadFilled, ready: false },
] as const

const form = reactive<StorageSourceInput>({
  name: '本地文件',
  key: 'local',
  type: 'local',
  rootPath: 'data/files',
  public: true,
  enabled: true,
  orderNum: 0,
})

const activeProvider = computed(() => providers.find(item => item.value === form.type) || providers[0])
const canEnable = computed(() => activeProvider.value.ready)
const editingSource = computed(() => sources.value.find(item => item.id === editingId.value))

function resetForm() {
  editingId.value = null
  Object.assign(form, {
    name: '本地文件',
    key: '',
    type: 'local',
    rootPath: 'data/files',
    public: true,
    enabled: true,
    orderNum: sources.value.length,
  } satisfies StorageSourceInput)
}

function editSource(source: StorageSource) {
  editingId.value = source.id
  Object.assign(form, {
    name: source.name,
    key: source.key,
    type: source.type as SourceType,
    rootPath: source.rootPath || '',
    public: source.public,
    enabled: source.enabled,
    orderNum: source.orderNum,
  } satisfies StorageSourceInput)
}

async function loadSources() {
  loading.value = true
  try {
    sources.value = await api<StorageSource[]>('/api/storage-sources')
    if (sources.value.length && editingId.value === null)
      editSource(sources.value[0])
  }
  finally {
    loading.value = false
  }
}

async function saveSource() {
  saving.value = true
  try {
    const payload = { ...form, enabled: canEnable.value ? form.enabled : false }
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
  resetForm()
  await loadSources()
}

function onTypeChange() {
  if (!canEnable.value)
    form.enabled = false
  if (form.type !== 'local')
    form.rootPath = ''
}

onMounted(loadSources)
</script>

<template>
  <div class="workspace" v-loading="loading">
    <section class="overview-band">
      <div>
        <p class="eyebrow">
          Storage sources
        </p>
        <h1>存储源管理</h1>
        <p class="lede">
          管理公开入口和后台文件管理使用的存储源实例。当前可读写适配器为本地存储，其余类型会在后续适配器任务中接入。
        </p>
      </div>
      <el-button type="primary" :icon="Plus" @click="resetForm">
        新建存储源
      </el-button>
    </section>

    <section class="storage-source-layout">
      <aside class="panel storage-provider-list">
        <button
          v-for="source in sources"
          :key="source.id"
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

          <div class="access-form-grid">
            <el-form-item label="在公开首页显示">
              <el-switch v-model="form.public" />
            </el-form-item>
            <el-form-item label="启用存储源">
              <el-switch v-model="form.enabled" :disabled="!canEnable" />
            </el-form-item>
          </div>

          <el-form-item>
            <div class="storage-form-actions">
              <el-button type="primary" :loading="saving" @click="saveSource">
                {{ editingId === null ? '创建存储源' : '保存存储源' }}
              </el-button>
              <el-button @click="resetForm">
                重置
              </el-button>
              <el-button v-if="editingSource" type="danger" plain @click="removeSource(editingSource)">
                删除
              </el-button>
            </div>
          </el-form-item>
        </el-form>
      </main>
    </section>
  </div>
</template>
