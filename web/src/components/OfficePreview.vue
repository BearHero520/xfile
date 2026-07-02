<script setup lang="ts">
import { computed, defineAsyncComponent, ref, watch } from 'vue'

export type OfficePreviewKind = 'docx' | 'excel' | 'pptx'

const props = defineProps<{
  src: string
  kind: OfficePreviewKind
}>()

const emit = defineEmits<{
  error: [message: string]
  rendered: []
}>()

const failed = ref(false)

const excelOptions = {
  showContextmenu: false,
}

const VueOfficeDocx = defineAsyncComponent(async () => {
  await import('@vue-office/docx/lib/index.css')
  return (await import('@vue-office/docx')).default
})

const VueOfficeExcel = defineAsyncComponent(async () => {
  await import('@vue-office/excel/lib/index.css')
  return (await import('@vue-office/excel')).default
})

const VueOfficePptx = defineAsyncComponent(async () => {
  return (await import('@vue-office/pptx')).default
})

const viewerComponent = computed(() => {
  if (props.kind === 'docx')
    return VueOfficeDocx
  if (props.kind === 'excel')
    return VueOfficeExcel
  return VueOfficePptx
})

function rendered() {
  failed.value = false
  emit('rendered')
}

function errorHandler(error: unknown) {
  failed.value = true
  emit('error', error instanceof Error ? error.message : 'Office 文件预览失败')
}

watch(() => [props.src, props.kind], () => {
  failed.value = false
})
</script>

<template>
  <div class="office-preview">
    <component
      :is="viewerComponent"
      v-if="!failed"
      :src="src"
      :options="kind === 'excel' ? excelOptions : undefined"
      class="office-preview-viewer"
      @rendered="rendered"
      @error="errorHandler"
    />
    <el-empty v-else description="Office 文件预览失败，请下载后查看" />
  </div>
</template>
