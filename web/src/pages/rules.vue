<script setup lang="ts">
import {
  DataLine,
  Connection,
  Lock,
  Operation,
  Setting,
  Upload,
} from '@element-plus/icons-vue'
import { computed, onMounted, ref } from 'vue'
import { api } from '~/api'

const settings = ref<Record<string, string>>({})
const loading = ref(false)

const ruleModules = [
  {
    title: '访问控制',
    description: '维护 IP 白名单、黑名单、私有路径、Referer 防盗链和下载限频。',
    icon: Lock,
    to: '/access',
    status: '已接入',
    type: 'success',
  },
  {
    title: '上传规则',
    description: '配置上传开关和单文件大小限制，后续可扩展文件类型和目录配额。',
    icon: Upload,
    to: '/uploads',
    status: '基础可用',
    type: 'success',
  },
  {
    title: '日志审计',
    description: '按动作、路径、IP、客户端和时间范围查询访问记录，并定期清理。',
    icon: DataLine,
    to: '/logs',
    status: '已增强',
    type: 'warning',
  },
  {
    title: 'WebDAV',
    description: '管理 WebDAV 开关、挂载路径、只读模式和客户端连接信息。',
    icon: Connection,
    to: '/webdav',
    status: '配置页',
    type: 'info',
  },
  {
    title: '系统设置',
    description: '管理站点名称、根目录显示名和后续全局策略开关。',
    icon: Setting,
    to: '/settings',
    status: '配置项',
    type: 'info',
  },
] as const

const policySummary = computed(() => [
  {
    label: '上传',
    value: settings.value.allowUpload === 'disabled' ? '已关闭' : `开启 / ${settings.value.maxUploadMB || '512'} MB`,
    type: settings.value.allowUpload === 'disabled' ? 'warning' : 'success',
  },
  {
    label: 'WebDAV',
    value: settings.value.webdav === 'enabled' ? '配置已启用' : '未启用',
    type: settings.value.webdav === 'enabled' ? 'warning' : 'info',
  },
  {
    label: 'Referer',
    value: settings.value.refererProtection === 'enabled' ? '防盗链开启' : '未开启',
    type: settings.value.refererProtection === 'enabled' ? 'success' : 'info',
  },
  {
    label: '下载限频',
    value: Number(settings.value.downloadLimitPerMinute || 0) > 0
      ? `${settings.value.downloadLimitPerMinute} 次/分钟`
      : '未限制',
    type: Number(settings.value.downloadLimitPerMinute || 0) > 0 ? 'success' : 'info',
  },
  {
    label: '私有路径',
    value: settings.value.privatePathList?.trim() ? `${settings.value.privatePathList.trim().split(/\s+/).length} 条` : '未配置',
    type: settings.value.privatePathList?.trim() ? 'success' : 'info',
  },
  {
    label: 'IP 规则',
    value: ipRuleSummary.value,
    type: ipRuleSummary.value === '未配置' ? 'info' : 'warning',
  },
] as const)

const ipRuleSummary = computed(() => {
  const allowCount = settings.value.ipAllowList?.trim() ? settings.value.ipAllowList.trim().split(/\s+/).length : 0
  const denyCount = settings.value.ipDenyList?.trim() ? settings.value.ipDenyList.trim().split(/\s+/).length : 0
  if (!allowCount && !denyCount)
    return '未配置'
  return `白 ${allowCount} / 黑 ${denyCount}`
})

async function loadSettings() {
  loading.value = true
  try {
    settings.value = await api<Record<string, string>>('/api/settings')
  }
  finally {
    loading.value = false
  }
}

onMounted(loadSettings)
</script>

<template>
  <div class="workspace" v-loading="loading">
    <section class="overview-band">
      <div>
        <p class="eyebrow">
          Policy center
        </p>
        <h1>规则管理</h1>
        <p class="lede">
          集中查看和进入文件、访问、上传与审计相关规则。
        </p>
      </div>
    </section>

    <section class="policy-summary-grid">
      <article v-for="item in policySummary" :key="item.label" class="policy-summary">
        <span>{{ item.label }}</span>
        <el-tag :type="item.type" effect="plain">
          {{ item.value }}
        </el-tag>
      </article>
    </section>

    <section class="rule-module-grid">
      <RouterLink
        v-for="item in ruleModules"
        :key="item.to"
        class="rule-module"
        :to="item.to"
      >
        <div class="rule-module-icon">
          <el-icon><component :is="item.icon" /></el-icon>
        </div>
        <div>
          <div class="rule-module-heading">
            <h2>{{ item.title }}</h2>
            <el-tag size="small" :type="item.type" effect="plain">
              {{ item.status }}
            </el-tag>
          </div>
          <p>{{ item.description }}</p>
        </div>
      </RouterLink>
    </section>

    <section class="panel">
      <div class="panel-title">
        <el-icon><Operation /></el-icon>
        <span>已落地能力</span>
      </div>
      <div class="rule-grid">
        <el-tag type="success">
          目录穿越防护
        </el-tag>
        <el-tag type="success">
          IP / CIDR
        </el-tag>
        <el-tag type="success">
          私有路径保护
        </el-tag>
        <el-tag type="warning">
          Referer 防盗链
        </el-tag>
        <el-tag type="warning">
          下载限频
        </el-tag>
        <el-tag type="info">
          日志过滤与清理
        </el-tag>
      </div>
    </section>
  </div>
</template>
