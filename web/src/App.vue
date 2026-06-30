<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const isPlain = computed(() => route.path === '/login' || route.path.startsWith('/s/'))
</script>

<template>
  <el-config-provider namespace="ep">
    <BaseHeader v-if="!isPlain" />
    <div :class="isPlain ? 'login-container' : 'main-container'">
      <BaseSide v-if="!isPlain" />
      <div :class="isPlain ? 'login-shell' : 'page-shell'">
        <RouterView />
      </div>
    </div>
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

.page-shell {
  min-width: 0;
  padding: 24px;
}

.login-container,
.login-shell {
  min-height: 100vh;
}
</style>
