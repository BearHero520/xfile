<script lang="ts" setup>
import type { AuthMe } from '~/api'
import {
  Connection,
  DataLine,
  Files,
  Link,
  Lock,
  Management,
  Operation,
  Search,
  Setting,
  Share,
  Upload,
  UserFilled,
} from '@element-plus/icons-vue'
import { computed, onMounted, ref } from 'vue'
import { api } from '~/api'

const me = ref<AuthMe>()
const canManageSystem = computed(() => me.value?.user?.role === 'super_admin')

onMounted(async () => {
  try {
    me.value = await api<AuthMe>('/api/auth/me', { skipAuthRedirect: true })
  }
  catch {
    me.value = undefined
  }
})
</script>

<template>
  <aside class="app-sidebar">
    <el-menu router default-active="/" class="side-menu">
      <el-menu-item index="/">
        <el-icon><Files /></el-icon>
        <template #title>
          文件管理
        </template>
      </el-menu-item>
      <el-menu-item index="/shares">
        <el-icon><Share /></el-icon>
        <template #title>
          分享列表
        </template>
      </el-menu-item>
      <el-menu-item index="/search">
        <el-icon><Search /></el-icon>
        <template #title>
          全局搜索
        </template>
      </el-menu-item>
      <el-menu-item index="/direct-links">
        <el-icon><Link /></el-icon>
        <template #title>
          直链 / 短链
        </template>
      </el-menu-item>
      <el-menu-item v-if="canManageSystem" index="/storage">
        <el-icon><Management /></el-icon>
        <template #title>
          存储源
        </template>
      </el-menu-item>
      <el-menu-item v-if="canManageSystem" index="/webdav">
        <el-icon><Connection /></el-icon>
        <template #title>
          WebDAV
        </template>
      </el-menu-item>
      <el-menu-item v-if="canManageSystem" index="/rules">
        <el-icon><Operation /></el-icon>
        <template #title>
          规则管理
        </template>
      </el-menu-item>
      <el-menu-item v-if="canManageSystem" index="/uploads">
        <el-icon><Upload /></el-icon>
        <template #title>
          上传规则
        </template>
      </el-menu-item>
      <el-menu-item v-if="canManageSystem" index="/access">
        <el-icon><Lock /></el-icon>
        <template #title>
          访问控制
        </template>
      </el-menu-item>
      <el-menu-item v-if="canManageSystem" index="/users">
        <el-icon><UserFilled /></el-icon>
        <template #title>
          用户管理
        </template>
      </el-menu-item>
      <el-menu-item index="/logs">
        <el-icon><DataLine /></el-icon>
        <template #title>
          访问日志
        </template>
      </el-menu-item>
      <el-menu-item v-if="canManageSystem" index="/settings">
        <el-icon><Setting /></el-icon>
        <template #title>
          系统设置
        </template>
      </el-menu-item>
    </el-menu>
  </aside>
</template>
