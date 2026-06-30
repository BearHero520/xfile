<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { onMounted, reactive, ref } from 'vue'
import { api } from '~/api'

const loading = ref(false)
const saving = ref(false)
const form = reactive({
  ipAllowList: '',
  ipDenyList: '',
  privatePathList: '',
  refererProtection: 'disabled',
  refererAllowList: '',
})

async function loadSettings() {
  loading.value = true
  try {
    const settings = await api<Record<string, string>>('/api/settings')
    form.ipAllowList = settings.ipAllowList || ''
    form.ipDenyList = settings.ipDenyList || ''
    form.privatePathList = settings.privatePathList || ''
    form.refererProtection = settings.refererProtection || 'disabled'
    form.refererAllowList = settings.refererAllowList || ''
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
        refererProtection: form.refererProtection,
        refererAllowList: form.refererAllowList,
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
  <div class="workspace" v-loading="loading">
    <section class="panel access-panel">
      <div class="panel-heading">
        <div>
          <div class="panel-title">
            访问控制
          </div>
          <p class="lede">
            配置后台接口、公开分享和直链访问的 IP、路径和来源策略。
          </p>
        </div>
      </div>

      <el-form class="access-rules-form" label-position="top" :model="form">
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
      </div>
    </section>
  </div>
</template>
