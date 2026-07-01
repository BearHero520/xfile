<script setup lang="ts">
import type { StorageSource, UserEntry } from '~/api'
import { Edit, Plus, Refresh, UserFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'
import { api, formatTime } from '~/api'

const loading = ref(false)
const saving = ref(false)
const users = ref<UserEntry[]>([])
const sources = ref<StorageSource[]>([])
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
  storageSourceKeys: [] as string[],
  storageSourceRoots: {} as Record<string, string>,
  disabledOperations: [] as string[],
})

const isEditing = computed(() => editingId.value !== null)
const superAdminCount = computed(() => users.value.filter(user => user.role === 'super_admin').length)

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

function roleLabel(role: string) {
  return role === 'super_admin' ? '超级管理员' : '管理员'
}

onMounted(loadUsers)
</script>

<template>
  <div class="workspace" v-loading="loading">
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
    </section>

    <section class="panel">
      <div class="panel-title">
        <el-icon><UserFilled /></el-icon>
        <span>账号列表</span>
      </div>
      <el-table :data="users" class="user-table">
        <el-table-column prop="username" label="用户名" min-width="180" />
        <el-table-column label="角色" width="150">
          <template #default="{ row }">
            <el-tag :type="row.role === 'super_admin' ? 'success' : 'info'" effect="plain">
              {{ roleLabel(row.role) }}
            </el-tag>
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
            <el-option label="管理员" value="admin" />
            <el-option label="超级管理员" value="super_admin" />
          </el-select>
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
  </div>
</template>
