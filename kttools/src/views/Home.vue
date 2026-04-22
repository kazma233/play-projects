<template>
  <div class="page-view page-view--centered">
    <n-card class="tool-surface" size="small">
      <div class="tool-banner">
        <div class="tool-banner__copy">
          <span class="tool-panel__meta">{{ homeMeta.eyebrow }}</span>
          <h2>Kt 工具箱</h2>
          <p>{{ homeMeta.description }}</p>
        </div>
        <span class="tool-banner__icon">{{ homeMeta.icon }}</span>
      </div>
    </n-card>
  </div>
</template>

<script setup>
import { homeMeta } from '../constants/tool-meta'
</script>
