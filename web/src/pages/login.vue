<script setup lang="ts">
import { Lock } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '~/api'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const form = reactive({ password: '' })

async function login() {
  loading.value = true
  try {
    await api('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ password: form.password }),
    })
    ElMessage.success('登录成功')
    await router.replace(String(route.query.redirect || '/'))
  }
  catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '登录失败')
  }
  finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="login-page">
    <section class="login-panel">
      <span class="brand-mark">XF</span>
      <h1>XFile</h1>
      <p class="lede">
        登录后管理文件、分享链接、直链和系统规则。
      </p>
      <el-form :model="form" @submit.prevent="login">
        <el-form-item>
          <el-input
            v-model="form.password"
            type="password"
            size="large"
            placeholder="管理员密码"
            show-password
            :prefix-icon="Lock"
            @keydown.enter="login"
          />
        </el-form-item>
        <el-button type="primary" size="large" :loading="loading" class="login-button" @click="login">
          登录
        </el-button>
      </el-form>
    </section>
  </main>
</template>
