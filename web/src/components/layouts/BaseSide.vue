<script lang="ts" setup>
import {
  Connection,
  DataLine,
  Files,
  Link,
  Lock,
  Operation,
  Search,
  Setting,
  Share,
  Upload,
} from '@element-plus/icons-vue'
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
      <el-menu-item index="/webdav">
        <el-icon><Connection /></el-icon>
        <template #title>
          WebDAV
        </template>
      </el-menu-item>
      <el-menu-item index="/rules">
        <el-icon><Operation /></el-icon>
        <template #title>
          规则管理
        </template>
      </el-menu-item>
      <el-menu-item index="/uploads">
        <el-icon><Upload /></el-icon>
        <template #title>
          上传规则
        </template>
      </el-menu-item>
      <el-menu-item index="/access">
        <el-icon><Lock /></el-icon>
        <template #title>
          访问控制
        </template>
      </el-menu-item>
      <el-menu-item index="/logs">
        <el-icon><DataLine /></el-icon>
        <template #title>
          访问日志
        </template>
      </el-menu-item>
      <el-menu-item index="/settings">
        <el-icon><Setting /></el-icon>
        <template #title>
          系统设置
        </template>
      </el-menu-item>
    </el-menu>
  </aside>
</template>
