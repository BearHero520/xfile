<script setup lang="ts">
import type Player from 'video.js/dist/types/player'
import videojs from 'video.js'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import 'video.js/dist/video-js.css'

const props = defineProps<{
  src: string
  type?: string
  title?: string
}>()

const videoRef = ref<HTMLVideoElement>()
let player: Player | undefined

function initializePlayer() {
  if (!videoRef.value || player)
    return
  player = videojs(videoRef.value, {
    controls: true,
    fluid: true,
    responsive: true,
    preload: 'metadata',
    sources: [{ src: props.src, type: props.type }],
  })
}

watch(() => [props.src, props.type], ([src, type]) => {
  if (!player)
    return
  player.src({ src, type })
  player.load()
})

onMounted(initializePlayer)

onBeforeUnmount(() => {
  player?.dispose()
  player = undefined
})
</script>

<template>
  <div class="x-video-player" :aria-label="title">
    <video
      ref="videoRef"
      class="video-js vjs-big-play-centered vjs-fluid"
      playsinline
    />
  </div>
</template>

<style scoped>
.x-video-player {
  overflow: hidden;
  border-radius: var(--x-radius-panel);
  background: #000;
}

.x-video-player :deep(.video-js) {
  width: 100%;
  max-height: 70vh;
  background: #000;
  font-family:
    Inter, system-ui, Avenir, 'Helvetica Neue', Helvetica, 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', Arial,
    sans-serif;
}

.x-video-player :deep(.vjs-control-bar) {
  background: linear-gradient(180deg, transparent, rgba(0, 0, 0, 0.72));
}
</style>
