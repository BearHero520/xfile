<script setup lang="ts">
import { Lock, Refresh, User } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '~/api'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const initialized = ref(true)
const captchaRequired = ref(false)
const captchaChallenge = ref<{ id: string, question: string }>()
const form = reactive({
  username: 'admin',
  password: '',
  confirmPassword: '',
  captchaAnswer: '',
})

async function loadStatus() {
  const status = await api<{ initialized: boolean, authenticated: boolean, captchaRequired?: boolean }>('/api/auth/me')
  initialized.value = status.initialized
  if (status.authenticated)
    await router.replace(String(route.query.redirect || '/'))
  captchaRequired.value = !!status.captchaRequired
  if (captchaRequired.value)
    await loadCaptcha()
}

async function loadCaptcha() {
  const data = await api<{ required: boolean, id?: string, question?: string }>('/api/auth/captcha', { skipAuthRedirect: true })
  captchaRequired.value = data.required
  captchaChallenge.value = data.id && data.question ? { id: data.id, question: data.question } : undefined
  form.captchaAnswer = ''
}

async function submit() {
  if (!initialized.value && form.password !== form.confirmPassword) {
    ElMessage.error('两次输入的密码不一致')
    return
  }
  if (initialized.value && captchaRequired.value && !form.captchaAnswer.trim()) {
    ElMessage.error('请输入验证码答案')
    return
  }
  loading.value = true
  try {
    await api(initialized.value ? '/api/auth/login' : '/api/auth/setup', {
      method: 'POST',
      body: JSON.stringify({
        username: form.username,
        password: form.password,
        captchaID: captchaChallenge.value?.id || '',
        captchaAnswer: form.captchaAnswer,
      }),
    })
    ElMessage.success(initialized.value ? '登录成功' : '初始化完成')
    await router.replace(String(route.query.redirect || '/'))
  }
  catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '操作失败')
    if (captchaRequired.value)
      await loadCaptcha()
  }
  finally {
    loading.value = false
  }
}

onMounted(loadStatus)
</script>

<template>
  <main class="login-page">
    <section class="login-panel">
      <span class="brand-mark">XF</span>
      <h1>{{ initialized ? '登录 XFile' : '初始化系统' }}</h1>
      <p class="lede">
        {{ initialized ? '登录后管理文件、分享链接、直链和系统规则。' : '首次使用前，请创建系统超级管理员账户。' }}
      </p>
      <el-form :model="form" @submit.prevent="submit">
        <el-form-item>
          <el-input
            v-model="form.username"
            size="large"
            placeholder="超级管理员账号"
            :prefix-icon="User"
            @keydown.enter="submit"
          />
        </el-form-item>
        <el-form-item>
          <el-input
            v-model="form.password"
            type="password"
            size="large"
            placeholder="密码，至少 8 位"
            show-password
            :prefix-icon="Lock"
            @keydown.enter="submit"
          />
        </el-form-item>
        <el-form-item v-if="!initialized">
          <el-input
            v-model="form.confirmPassword"
            type="password"
            size="large"
            placeholder="确认密码"
            show-password
            :prefix-icon="Lock"
            @keydown.enter="submit"
          />
        </el-form-item>
        <el-form-item v-if="initialized && captchaRequired">
          <el-input
            v-model="form.captchaAnswer"
            size="large"
            :placeholder="captchaChallenge?.question || '验证码'"
            @keydown.enter="submit"
          >
            <template #append>
              <el-button :icon="Refresh" @click="loadCaptcha" />
            </template>
          </el-input>
        </el-form-item>
        <el-button type="primary" size="large" :loading="loading" class="login-button" @click="submit">
          {{ initialized ? '登录' : '创建超级管理员' }}
        </el-button>
      </el-form>
    </section>
  </main>
</template>
