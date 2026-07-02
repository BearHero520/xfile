<script setup lang="ts">
import type { SessionEntry, StorageSource, UserEntry } from '~/api'
import { Edit, Monitor, Plus, Refresh, SwitchButton, UserFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'
import { api, formatTime } from '~/api'

const loading = ref(false)
const saving = ref(false)
const users = ref<UserEntry[]>([])
const sources = ref<StorageSource[]>([])
const sessions = ref<SessionEntry[]>([])
const sessionLoading = ref(false)
const sessionDialogVisible = ref(false)
const sessionUser = ref<UserEntry | null>(null)
const revokingSessions = ref(false)
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
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const form = reactive({
  username: '',
  password: '',
  role: 'admin',
  enabled: true,
  storageSourceKeys: [] as string[],
  storageSourceRoots: {} as Record<string, string>,
  disabledOperations: [] as string[],
})

const isEditing = computed(() => editingId.value !== null)
const superAdminCount = computed(() => users.value.filter(user => user.role === 'super_admin').length)
const normalUserCount = computed(() => users.value.filter(user => user.role !== 'super_admin').length)
const disabledUserCount = computed(() => users.value.filter(user => user.enabled === false).length)
const activeSessionCount = computed(() => users.value.reduce((total, user) => total + (user.activeSessionCount || 0), 0))

async function loadUsers() {
  loading.value = true
  try {
    const [userList, sourceList] = await Promise.all([
      api<UserEntry[]>('/api/users'),
      api<StorageSource[]>('/api/storage-sources'),
    ])
    users.value = userList
    sources.value = sourceList
  }
  finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  form.username = ''
  form.password = ''
  form.role = 'admin'
  form.enabled = true
  form.storageSourceKeys = []
  form.storageSourceRoots = {}
  form.disabledOperations = []
  dialogVisible.value = true
}

function openEdit(user: UserEntry) {
  editingId.value = user.id
  form.username = user.username
  form.password = ''
  form.role = user.role
  form.enabled = user.enabled !== false
  form.storageSourceKeys = [...(user.storageSourceKeys || [])]
  form.storageSourceRoots = Object.fromEntries(
    Object.entries(user.storageSourceRoots || {}).map(([key, roots]) => [key, roots.join('\n')]),
  )
  form.disabledOperations = [...(user.disabledOperations || [])]
  dialogVisible.value = true
}

async function saveUser() {
  saving.value = true
  try {
    const storageSourceKeys = form.role === 'super_admin' ? [] : form.storageSourceKeys
    const storageSourceRoots = Object.fromEntries(
      storageSourceKeys.map(key => [
        key,
        (form.storageSourceRoots[key] || '').split(/\r?\n/).map(path => path.trim()).filter(Boolean),
      ]),
    )
    const body = JSON.stringify({
      username: form.username,
      password: form.password,
      role: form.role,
      enabled: form.enabled,
      storageSourceKeys,
      storageSourceRoots,
      disabledOperations: form.role === 'super_admin' ? [] : form.disabledOperations,
    })
    if (editingId.value === null) {
      await api('/api/users', { method: 'POST', body })
      ElMessage.success('用户已创建')
    }
    else {
      await api(`/api/users/${editingId.value}`, { method: 'PATCH', body })
      ElMessage.success('用户已更新')
    }
    dialogVisible.value = false
    await loadUsers()
  }
  finally {
    saving.value = false
  }
}

async function setUserEnabled(user: UserEntry, enabled: boolean) {
  try {
    const body = JSON.stringify({
      username: user.username,
      password: '',
      role: user.role,
      enabled,
      storageSourceKeys: user.role === 'super_admin' ? [] : user.storageSourceKeys || [],
      storageSourceRoots: user.role === 'super_admin' ? {} : user.storageSourceRoots || {},
      disabledOperations: user.role === 'super_admin' ? [] : user.disabledOperations || [],
    })
    await api(`/api/users/${user.id}`, { method: 'PATCH', body })
    ElMessage.success(enabled ? 'User enabled' : 'User disabled')
    await loadUsers()
  }
  catch (err) {
    user.enabled = !enabled
    ElMessage.error(err instanceof Error ? err.message : 'Failed to update user status')
  }
}

async function deleteUser(user: UserEntry) {
  try {
    await ElMessageBox.confirm(`确认删除用户「${user.username}」？`, '删除用户', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
  }
  catch {
    return
  }
  await api(`/api/users/${user.id}`, { method: 'DELETE' })
  ElMessage.success('用户已删除')
  await loadUsers()
}

async function openSessions(user: UserEntry) {
  sessionUser.value = user
  sessionDialogVisible.value = true
  await loadUserSessions()
}

async function loadUserSessions() {
  if (!sessionUser.value)
    return
  sessionLoading.value = true
  try {
    sessions.value = await api<SessionEntry[]>(`/api/users/${sessionUser.value.id}/sessions`)
  }
  finally {
    sessionLoading.value = false
  }
}

async function revokeSession(session: SessionEntry) {
  if (!sessionUser.value)
    return
  await ElMessageBox.confirm('Force this browser session to log out?', 'Force logout', {
    type: 'warning',
    confirmButtonText: 'Force logout',
    cancelButtonText: 'Cancel',
  })
  const revokesCurrent = session.current
  await api(`/api/users/${sessionUser.value.id}/sessions/${session.id}`, { method: 'DELETE' })
  ElMessage.success('Session revoked')
  if (revokesCurrent) {
    location.href = '/login'
    return
  }
  await loadUserSessions()
  await loadUsers()
}

async function revokeAllSessions() {
  if (!sessionUser.value)
    return
  await ElMessageBox.confirm(`Force all sessions for ${sessionUser.value.username} to log out?`, 'Force logout all', {
    type: 'warning',
    confirmButtonText: 'Force logout all',
    cancelButtonText: 'Cancel',
  })
  const revokesCurrent = sessions.value.some(session => session.current)
  revokingSessions.value = true
  try {
    const data = await api<{ revoked: number }>(`/api/users/${sessionUser.value.id}/sessions`, { method: 'DELETE' })
    ElMessage.success(`${data.revoked} session(s) revoked`)
    if (revokesCurrent) {
      location.href = '/login'
      return
    }
    await loadUserSessions()
    await loadUsers()
  }
  finally {
    revokingSessions.value = false
  }
}

function userAgentLabel(value: string) {
  return value || '-'
}

function roleLabel(role: string) {
  return role === 'super_admin' ? '超级管理员' : '普通用户'
}

function roleDescription(role: string) {
  return role === 'super_admin'
    ? '拥有系统配置、用户管理和全部文件操作权限。'
    : '只能访问已分配的存储源和根路径，并受操作权限限制。'
}

onMounted(loadUsers)
</script>

<template>
  <div v-loading="loading" class="workspace">
    <section class="overview-band">
      <div>
        <p class="eyebrow">
          User management
        </p>
        <h1>用户管理</h1>
        <p class="lede">
          维护后台登录账号、角色和密码。删除最后一个账号会被系统拒绝。
        </p>
      </div>
      <div class="quick-actions">
        <el-button :icon="Refresh" @click="loadUsers">
          刷新
        </el-button>
        <el-button type="primary" :icon="Plus" @click="openCreate">
          新建用户
        </el-button>
      </div>
    </section>

    <section class="policy-summary-grid">
      <article class="policy-summary">
        <span>用户数</span>
        <el-tag type="success" effect="plain">
          {{ users.length }}
        </el-tag>
      </article>
      <article class="policy-summary">
        <span>超级管理员</span>
        <el-tag :type="superAdminCount > 0 ? 'success' : 'warning'" effect="plain">
          {{ superAdminCount }}
        </el-tag>
      </article>
      <article class="policy-summary">
        <span>普通用户</span>
        <el-tag type="info" effect="plain">
          {{ normalUserCount }}
        </el-tag>
      </article>
      <article class="policy-summary">
        <span>已停用</span>
        <el-tag :type="disabledUserCount > 0 ? 'warning' : 'success'" effect="plain">
          {{ disabledUserCount }}
        </el-tag>
      </article>
      <article class="policy-summary">
        <span>Active sessions</span>
        <el-tag :type="activeSessionCount > 0 ? 'success' : 'info'" effect="plain">
          {{ activeSessionCount }}
        </el-tag>
      </article>
    </section>

    <section class="role-contrast-grid">
      <article class="role-contrast-item">
        <div>
          <strong>超级管理员</strong>
          <p>系统级账号，可管理存储源、访问规则、WebDAV、用户、设置和日志清理。</p>
        </div>
        <el-tag type="success" effect="plain">
          Full control
        </el-tag>
      </article>
      <article class="role-contrast-item">
        <div>
          <strong>普通用户</strong>
          <p>工作区账号，只能使用分配给自己的存储源、根路径和文件操作。</p>
        </div>
        <el-tag type="info" effect="plain">
          Scoped access
        </el-tag>
      </article>
    </section>

    <section class="panel">
      <div class="panel-title">
        <el-icon><UserFilled /></el-icon>
        <span>账号列表</span>
      </div>
      <el-table :data="users" class="user-table">
        <el-table-column prop="username" label="用户名" min-width="180" />
        <el-table-column label="角色" min-width="230">
          <template #default="{ row }">
            <el-tag :type="row.role === 'super_admin' ? 'success' : 'info'" effect="plain">
              {{ roleLabel(row.role) }}
            </el-tag>
            <p class="role-cell-note">
              {{ roleDescription(row.role) }}
            </p>
          </template>
        </el-table-column>
        <el-table-column label="Status" width="130">
          <template #default="{ row }">
            <el-switch
              v-model="row.enabled"
              inline-prompt
              active-text="On"
              inactive-text="Off"
              @change="(value: boolean) => setUserEnabled(row, value)"
            />
          </template>
        </el-table-column>
        <el-table-column label="Sessions" width="140">
          <template #default="{ row }">
            <el-button text :icon="Monitor" @click="openSessions(row)">
              {{ row.activeSessionCount || 0 }} active
            </el-button>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatTime(row.createdAt) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button text :icon="Edit" @click="openEdit(row)">
              编辑
            </el-button>
            <el-button text type="danger" @click="deleteUser(row)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <el-dialog v-model="dialogVisible" :title="isEditing ? '编辑用户' : '新建用户'" width="420px">
      <el-form label-position="top" :model="form">
        <el-form-item label="用户名">
          <el-input v-model="form.username" autocomplete="off" />
        </el-form-item>
        <el-form-item :label="isEditing ? '新密码' : '密码'">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            autocomplete="new-password"
            :placeholder="isEditing ? '留空表示不修改密码' : '至少 8 位'"
          />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.role" class="full-control">
            <el-option label="普通用户" value="admin" />
            <el-option label="超级管理员" value="super_admin" />
          </el-select>
          <p class="form-help-text">
            {{ roleDescription(form.role) }}
          </p>
        </el-form-item>
        <el-form-item label="Status">
          <el-switch
            v-model="form.enabled"
            inline-prompt
            active-text="Enabled"
            inactive-text="Disabled"
          />
        </el-form-item>
        <el-form-item v-if="form.role !== 'super_admin'" label="Storage sources">
          <el-checkbox-group v-model="form.storageSourceKeys" class="source-checks">
            <el-checkbox v-for="source in sources" :key="source.key" :label="source.key">
              {{ source.name }} / {{ source.typeLabel }}
            </el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <template v-if="form.role !== 'super_admin'">
          <el-form-item v-for="source in sources.filter(item => form.storageSourceKeys.includes(item.key))" :key="`roots-${source.key}`" :label="`${source.name} root paths`">
            <el-input
              v-model="form.storageSourceRoots[source.key]"
              type="textarea"
              :rows="3"
              placeholder="One relative root path per line. Leave empty for full source access."
            />
          </el-form-item>
          <el-form-item label="Disabled operations">
            <el-checkbox-group v-model="form.disabledOperations" class="operation-checks">
              <el-checkbox v-for="operation in operationOptions" :key="operation.value" :label="operation.value">
                {{ operation.label }}
              </el-checkbox>
            </el-checkbox-group>
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">
          取消
        </el-button>
        <el-button type="primary" :loading="saving" @click="saveUser">
          保存
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="sessionDialogVisible" title="Session management" width="760px">
      <div class="session-dialog-header">
        <div>
          <strong>{{ sessionUser?.username }}</strong>
          <p class="form-help-text">
            Active browser sessions can be forced to log out immediately.
          </p>
        </div>
        <el-button
          type="danger"
          plain
          :icon="SwitchButton"
          :loading="revokingSessions"
          :disabled="sessions.length === 0"
          @click="revokeAllSessions"
        >
          Force all
        </el-button>
      </div>
      <el-table v-loading="sessionLoading" :data="sessions" class="session-table" empty-text="No active sessions">
        <el-table-column label="State" width="110">
          <template #default="{ row }">
            <el-tag :type="row.current ? 'success' : 'info'" effect="plain">
              {{ row.current ? 'Current' : 'Active' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" width="150" />
        <el-table-column label="User agent" min-width="220">
          <template #default="{ row }">
            <span class="session-user-agent">{{ userAgentLabel(row.userAgent) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Last seen" width="150">
          <template #default="{ row }">
            {{ formatTime(row.lastSeenAt) }}
          </template>
        </el-table-column>
        <el-table-column label="Expires" width="150">
          <template #default="{ row }">
            {{ formatTime(row.expiresAt) }}
          </template>
        </el-table-column>
        <el-table-column label="Action" width="130" fixed="right">
          <template #default="{ row }">
            <el-button text type="danger" :icon="SwitchButton" @click="revokeSession(row)">
              Logout
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>
