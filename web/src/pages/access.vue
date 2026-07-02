<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { onMounted, reactive, ref } from 'vue'
import { api } from '~/api'

const loading = ref(false)
const saving = ref(false)
const operationOptions = [
  { label: 'Preview', value: 'preview' },
  { label: 'Download', value: 'download' },
  { label: 'Upload / create', value: 'upload' },
  { label: 'Rename', value: 'rename' },
  { label: 'Move', value: 'move' },
  { label: 'Copy', value: 'copy' },
  { label: 'Delete', value: 'delete' },
  { label: 'Share', value: 'share' },
  { label: 'Direct links', value: 'directLinks' },
]
const form = reactive({
  ipAllowList: '',
  ipDenyList: '',
  privatePathList: '',
  directoryPasswordRules: '',
  disabledOperations: [] as string[],
  refererProtection: 'disabled',
  refererAllowList: '',
  downloadLimitPerMinute: '0',
  loginLimitPerMinute: '5',
  loginCaptcha: 'disabled',
  sharePasswordLimitPerMinute: '5',
})

async function loadSettings() {
  loading.value = true
  try {
    const settings = await api<Record<string, string>>('/api/settings')
    form.ipAllowList = settings.ipAllowList || ''
    form.ipDenyList = settings.ipDenyList || ''
    form.privatePathList = settings.privatePathList || ''
    form.directoryPasswordRules = settings.directoryPasswordRules || ''
    form.disabledOperations = (settings.disabledOperations || '').split(/[\s,;]+/).filter(Boolean)
    form.refererProtection = settings.refererProtection || 'disabled'
    form.refererAllowList = settings.refererAllowList || ''
    form.downloadLimitPerMinute = settings.downloadLimitPerMinute || '0'
    form.loginLimitPerMinute = settings.loginLimitPerMinute || '5'
    form.loginCaptcha = settings.loginCaptcha || 'disabled'
    form.sharePasswordLimitPerMinute = settings.sharePasswordLimitPerMinute || '5'
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
      body: JSON.stringify({
        ipAllowList: form.ipAllowList,
        ipDenyList: form.ipDenyList,
        privatePathList: form.privatePathList,
        directoryPasswordRules: form.directoryPasswordRules,
        disabledOperations: form.disabledOperations.join(','),
        refererProtection: form.refererProtection,
        refererAllowList: form.refererAllowList,
        downloadLimitPerMinute: form.downloadLimitPerMinute,
        loginLimitPerMinute: form.loginLimitPerMinute,
        loginCaptcha: form.loginCaptcha,
        sharePasswordLimitPerMinute: form.sharePasswordLimitPerMinute,
      }),
    })
    ElMessage.success('访问控制规则已保存')
  }
  finally {
    saving.value = false
  }
}

onMounted(loadSettings)
</script>

<template>
  <div v-loading="loading" class="workspace">
    <section class="panel access-panel">
      <div class="panel-heading">
        <div>
          <div class="panel-title">
            访问控制
          </div>
          <p class="lede">
            配置后台接口、公开分享和直链访问的 IP、路径、来源与限频策略。
          </p>
        </div>
      </div>

      <el-form class="access-rules-form" label-position="top" :model="form">
        <el-form-item label="Directory password rules">
          <el-input
            v-model="form.directoryPasswordRules"
            type="textarea"
            :rows="5"
            placeholder="One rule per line, format: path=password&#10;docs/private=secret123"
          />
        </el-form-item>

        <el-form-item label="Disabled operations">
          <el-checkbox-group v-model="form.disabledOperations" class="operation-checks">
            <el-checkbox v-for="operation in operationOptions" :key="operation.value" :label="operation.value">
              {{ operation.label }}
            </el-checkbox>
          </el-checkbox-group>
        </el-form-item>

        <div class="access-form-grid">
          <el-form-item label="IP 白名单">
            <el-input
              v-model="form.ipAllowList"
              type="textarea"
              :rows="6"
              placeholder="留空表示允许所有 IP。支持精确 IP 或 CIDR，例如：192.168.1.10&#10;10.0.0.0/8"
            />
          </el-form-item>
          <el-form-item label="IP 黑名单">
            <el-input
              v-model="form.ipDenyList"
              type="textarea"
              :rows="6"
              placeholder="命中黑名单会优先拒绝访问，例如：203.0.113.8&#10;2001:db8::/32"
            />
          </el-form-item>
        </div>

        <el-form-item label="私有路径">
          <el-input
            v-model="form.privatePathList"
            type="textarea"
            :rows="5"
            placeholder="每行一个相对路径。匹配路径及其子路径不能创建或访问公开分享/直链，例如：secret&#10;docs/private"
          />
        </el-form-item>

        <div class="access-form-grid">
          <el-form-item label="Referer 防盗链">
            <el-switch v-model="form.refererProtection" active-value="enabled" inactive-value="disabled" />
          </el-form-item>
          <el-form-item label="允许来源域名">
            <el-input
              v-model="form.refererAllowList"
              type="textarea"
              :rows="4"
              placeholder="留空只允许同站来源和空 Referer。支持域名或通配子域，例如：example.com&#10;*.cdn.example.com"
            />
          </el-form-item>
        </div>

        <div class="access-form-grid">
          <el-form-item label="下载限频">
            <el-input
              v-model="form.downloadLimitPerMinute"
              placeholder="每个 IP 每分钟允许的下载次数，0 表示关闭"
            />
          </el-form-item>
          <el-form-item label="登录限频">
            <el-input
              v-model="form.loginLimitPerMinute"
              placeholder="每个 IP 每分钟允许的登录次数，0 表示关闭"
            />
          </el-form-item>
          <el-form-item label="登录验证码">
            <el-switch v-model="form.loginCaptcha" active-value="enabled" inactive-value="disabled" />
          </el-form-item>
          <el-form-item label="分享密码限频">
            <el-input
              v-model="form.sharePasswordLimitPerMinute"
              placeholder="每个 IP 每分钟允许的错误分享密码次数，0 表示关闭"
            />
          </el-form-item>
        </div>

        <el-form-item>
          <el-button type="primary" :loading="saving" @click="saveRules">
            保存规则
          </el-button>
        </el-form-item>
      </el-form>

      <div class="rule-grid">
        <el-tag type="success">
          IP / CIDR
        </el-tag>
        <el-tag type="success">
          私有目录
        </el-tag>
        <el-tag type="warning">
          黑名单优先
        </el-tag>
        <el-tag type="info">
          Referer 防盗链
        </el-tag>
        <el-tag type="info">
          下载限频
        </el-tag>
        <el-tag type="warning">
          登录限频
        </el-tag>
        <el-tag type="warning">
          分享密码限频
        </el-tag>
      </div>
    </section>
  </div>
</template>
