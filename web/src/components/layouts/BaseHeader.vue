<script lang="ts" setup>
import { Bell, Moon, Search, Sunny, SwitchButton } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { api } from '~/api'
import { toggleDark } from '~/composables'

async function logout() {
  await api('/api/auth/logout', { method: 'POST' })
  ElMessage.success('已退出登录')
  location.href = '/login'
}
</script>

<template>
  <header class="app-header">
    <RouterLink class="brand" to="/">
      <span class="brand-mark">XF</span>
      <span>XFile</span>
    </RouterLink>
    <RouterLink class="header-search" to="/search">
      <el-icon><Search /></el-icon>
      <span>搜索文件、分享、直链与日志</span>
    </RouterLink>
    <div class="header-actions">
      <el-button :icon="Bell" circle title="通知" />
      <el-button circle title="切换主题" @click="toggleDark()">
        <el-icon>
          <Moon class="dark-only" />
          <Sunny class="light-only" />
        </el-icon>
      </el-button>
      <el-button :icon="SwitchButton" circle title="退出登录" @click="logout" />
    </div>
  </header>
</template>
