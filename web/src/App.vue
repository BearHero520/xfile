<script setup lang="ts">
import type { PublicSite } from '~/api'
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '~/api'

const route = useRoute()
const isPlain = computed(() => route.path === '/login' || route.path.startsWith('/s/'))
const loggedIn = ref(false)
const navDrawerOpen = ref(false)
const showShell = computed(() => !isPlain.value && loggedIn.value)

async function syncSession() {
  try {
    const site = await api<PublicSite>('/api/public/site', { skipAuthRedirect: true })
    loggedIn.value = site.loggedIn
  }
  catch {
    loggedIn.value = false
  }
}

onMounted(syncSession)

watch(() => route.fullPath, () => {
  navDrawerOpen.value = false
  void syncSession()
})
</script>

<template>
  <el-config-provider namespace="ep">
    <BaseHeader v-if="showShell" @open-nav="navDrawerOpen = true" />
    <div :class="isPlain ? 'login-container' : showShell ? 'main-container' : 'public-container'">
      <BaseSide v-if="showShell" />
      <div :class="isPlain ? 'login-shell' : 'page-shell'">
        <RouterView />
      </div>
    </div>
    <el-drawer
      v-model="navDrawerOpen"
      class="mobile-nav-drawer"
      direction="ltr"
      size="280px"
      :with-header="false"
      append-to-body
    >
      <BaseSide v-if="showShell" />
    </el-drawer>
  </el-config-provider>
</template>

<style>
#app {
  min-height: 100vh;
  color: var(--x-color-text);
  background: transparent;
}

.main-container {
  min-height: calc(100vh - 64px);
  display: grid;
  grid-template-columns: 248px minmax(0, 1fr);
}

.public-container {
  min-height: 100vh;
}

.page-shell {
  min-width: 0;
  padding: 24px;
}

.login-container,
.login-shell {
  min-height: 100vh;
}
</style>
