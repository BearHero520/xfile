<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '~/api'

const loading = ref(false)
const saving = ref(false)
const form = reactive({
  allowUpload: 'enabled',
  maxUploadMB: '512',
  uploadAllowExtensions: '',
  uploadDenyExtensions: '',
  uploadPathAllowList: '',
  uploadPathDenyList: '',
  uploadOverwrite: 'enabled',
})

const uploadState = computed(() => form.allowUpload === 'enabled' ? '已开启' : '已关闭')
const extensionState = computed(() => {
  const allow = countRules(form.uploadAllowExtensions)
  const deny = countRules(form.uploadDenyExtensions)
  if (!allow && !deny)
    return '未限制'
  return `允许 ${allow} / 禁止 ${deny}`
})
const pathState = computed(() => {
  const allow = countRules(form.uploadPathAllowList)
  const deny = countRules(form.uploadPathDenyList)
  if (!allow && !deny)
    return '未限制'
  return `允许 ${allow} / 禁止 ${deny}`
})

function countRules(value: string) {
  return value.trim() ? value.trim().split(/[\s,;]+/).filter(Boolean).length : 0
}

async function loadSettings() {
  loading.value = true
  try {
    const settings = await api<Record<string, string>>('/api/settings')
    form.allowUpload = settings.allowUpload || 'enabled'
    form.maxUploadMB = settings.maxUploadMB || '512'
    form.uploadAllowExtensions = settings.uploadAllowExtensions || ''
    form.uploadDenyExtensions = settings.uploadDenyExtensions || ''
    form.uploadPathAllowList = settings.uploadPathAllowList || ''
    form.uploadPathDenyList = settings.uploadPathDenyList || ''
    form.uploadOverwrite = settings.uploadOverwrite || 'enabled'
  }
  finally {
    loading.value = false
  }
}

async function saveRules() {
  saving.value = true
  try {
    await api('/api/settings', {
      method: 'PUT',
      body: JSON.stringify(form),
    })
    ElMessage.success('上传规则已保存')
  }
  finally {
    saving.value = false
  }
}

onMounted(loadSettings)
</script>

<template>
  <div v-loading="loading" class="workspace">
    <section class="overview-band">
      <div>
        <p class="eyebrow">
          Upload policy
        </p>
        <h1>上传规则</h1>
        <p class="lede">
          控制上传入口、单文件上限、文件扩展名、目标目录和同名文件覆盖策略。
        </p>
      </div>
    </section>

    <section class="policy-summary-grid">
      <article class="policy-summary">
        <span>上传入口</span>
        <el-tag :type="form.allowUpload === 'enabled' ? 'success' : 'warning'" effect="plain">
          {{ uploadState }}
        </el-tag>
      </article>
      <article class="policy-summary">
        <span>单文件上限</span>
        <el-tag type="success" effect="plain">
          {{ form.maxUploadMB || 512 }} MB
        </el-tag>
      </article>
      <article class="policy-summary">
        <span>扩展名</span>
        <el-tag :type="extensionState === '未限制' ? 'info' : 'warning'" effect="plain">
          {{ extensionState }}
        </el-tag>
      </article>
      <article class="policy-summary">
        <span>上传路径</span>
        <el-tag :type="pathState === '未限制' ? 'info' : 'warning'" effect="plain">
          {{ pathState }}
        </el-tag>
      </article>
      <article class="policy-summary">
        <span>同名文件</span>
        <el-tag :type="form.uploadOverwrite === 'enabled' ? 'warning' : 'success'" effect="plain">
          {{ form.uploadOverwrite === 'enabled' ? '允许覆盖' : '禁止覆盖' }}
        </el-tag>
      </article>
    </section>

    <section class="panel access-panel">
      <div class="panel-heading">
        <div>
          <div class="panel-title">
            上传策略
          </div>
          <p class="lede">
            扩展名支持 .jpg、png 或 *.zip；路径规则使用相对目录，命中目录及其子路径。
          </p>
        </div>
      </div>

      <el-form class="access-rules-form" label-position="top" :model="form">
        <div class="access-form-grid">
          <el-form-item label="允许上传">
            <el-switch v-model="form.allowUpload" active-value="enabled" inactive-value="disabled" />
          </el-form-item>
          <el-form-item label="单文件上限 MB">
            <el-input v-model="form.maxUploadMB" placeholder="例如：512" />
          </el-form-item>
        </div>

        <div class="access-form-grid">
          <el-form-item label="允许扩展名">
            <el-input
              v-model="form.uploadAllowExtensions"
              type="textarea"
              :rows="5"
              placeholder="留空表示不限制。每行一个，例如：&#10;.jpg&#10;.png&#10;pdf"
            />
          </el-form-item>
          <el-form-item label="禁止扩展名">
            <el-input
              v-model="form.uploadDenyExtensions"
              type="textarea"
              :rows="5"
              placeholder="命中后优先拒绝，例如：&#10;.exe&#10;.bat&#10;.sh"
            />
          </el-form-item>
        </div>

        <div class="access-form-grid">
          <el-form-item label="允许上传路径">
            <el-input
              v-model="form.uploadPathAllowList"
              type="textarea"
              :rows="5"
              placeholder="留空表示所有目录可上传。例如：&#10;incoming&#10;public/uploads"
            />
          </el-form-item>
          <el-form-item label="禁止上传路径">
            <el-input
              v-model="form.uploadPathDenyList"
              type="textarea"
              :rows="5"
              placeholder="命中后优先拒绝。例如：&#10;private&#10;incoming/tmp"
            />
          </el-form-item>
        </div>

        <el-form-item label="同名文件覆盖">
          <el-switch
            v-model="form.uploadOverwrite"
            active-text="允许覆盖"
            inactive-text="禁止覆盖"
            active-value="enabled"
            inactive-value="disabled"
          />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="saving" @click="saveRules">
            保存规则
          </el-button>
        </el-form-item>
      </el-form>
    </section>
  </div>
</template>
