<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const isLogin = computed(() => route.path === '/login')
</script>

<template>
  <el-config-provider namespace="ep">
    <BaseHeader v-if="!isLogin" />
    <div :class="isLogin ? 'login-container' : 'main-container'">
      <BaseSide v-if="!isLogin" />
      <div :class="isLogin ? 'login-shell' : 'page-shell'">
        <RouterView />
      </div>
    </div>
  </el-config-provider>
</template>

<style>
#app {
  min-height: 100vh;
  color: var(--ep-text-color-primary);
  background: #f5f7f4;
}

.main-container {
  min-height: calc(100vh - 64px);
  display: grid;
  grid-template-columns: 248px minmax(0, 1fr);
}

.page-shell {
  min-width: 0;
  padding: 24px;
}

.login-container {
  min-height: 100vh;
}

.login-shell {
  min-height: 100vh;
}
</style>
