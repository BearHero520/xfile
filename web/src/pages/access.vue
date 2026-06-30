<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { onMounted, reactive, ref } from 'vue'
import { api } from '~/api'

const loading = ref(false)
const saving = ref(false)
const form = reactive({
  ipAllowList: '',
  ipDenyList: '',
})

async function loadSettings() {
  loading.value = true
  try {
    const settings = await api<Record<string, string>>('/api/settings')
    form.ipAllowList = settings.ipAllowList || ''
    form.ipDenyList = settings.ipDenyList || ''
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
            配置后台接口、公开分享和直链访问的 IP 白名单与黑名单。
          </p>
        </div>
      </div>

      <el-form class="access-rules-form" label-position="top" :model="form">
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
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="saveRules">
            保存规则
          </el-button>
        </el-form-item>
      </el-form>

      <div class="rule-grid">
        <el-tag type="success">
          精确 IP
        </el-tag>
        <el-tag type="success">
          CIDR 网段
        </el-tag>
        <el-tag type="warning">
          黑名单优先
        </el-tag>
        <el-tag type="info">
          访问拒绝会记录日志
        </el-tag>
      </div>
    </section>
  </div>
</template>
