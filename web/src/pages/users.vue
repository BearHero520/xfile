<script setup lang="ts">
import type { UserEntry } from '~/api'
import { Edit, Plus, Refresh, UserFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'
import { api, formatTime } from '~/api'

const loading = ref(false)
const saving = ref(false)
const users = ref<UserEntry[]>([])
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const form = reactive({
  username: '',
  password: '',
  role: 'admin',
})

const isEditing = computed(() => editingId.value !== null)
const superAdminCount = computed(() => users.value.filter(user => user.role === 'super_admin').length)

async function loadUsers() {
  loading.value = true
  try {
    users.value = await api<UserEntry[]>('/api/users')
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
  dialogVisible.value = true
}

function openEdit(user: UserEntry) {
  editingId.value = user.id
  form.username = user.username
  form.password = ''
  form.role = user.role
  dialogVisible.value = true
}

async function saveUser() {
  saving.value = true
  try {
    const body = JSON.stringify({
      username: form.username,
      password: form.password,
      role: form.role,
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
