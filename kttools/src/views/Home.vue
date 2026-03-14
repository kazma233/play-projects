<template>
  <div class="page-view page-view--centered home-page">
    <section class="tool-surface home-brand-panel">
      <div class="home-brand-panel__icon">{{ homeMeta.icon }}</div>
      <div class="home-brand-panel__copy">
        <span class="home-brand-panel__eyebrow">KT TOOLS</span>
        <h1>Kt 工具箱</h1>
        <p>开发常用工具集合</p>
      </div>
    </section>
  </div>
</template>

<script setup>
import { homeMeta } from '../constants/tool-meta'
</script>

<style scoped>
.home-page {
  display: flex;
  align-items: center;
  justify-content: center;
}

.home-brand-panel {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.875rem;
  width: min(100%, 28rem);
  min-height: clamp(16rem, 42vh, 22rem);
  padding: clamp(1.5rem, 3vw, 2rem);
  text-align: center;
}

.home-brand-panel__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: clamp(4rem, 10vw, 5rem);
  aspect-ratio: 1;
  border-radius: clamp(1.25rem, 3vw, 1.5rem);
  background: linear-gradient(145deg, rgba(243, 210, 193, 0.8), rgba(255, 255, 255, 0.92));
  box-shadow: 0 12px 24px rgba(0, 24, 88, 0.08);
  font-size: clamp(2rem, 5vw, 2.4rem);
}

.home-brand-panel__copy {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  align-items: center;
}

.home-brand-panel__eyebrow {
  color: rgba(0, 24, 88, 0.54);
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.24em;
}

.home-brand-panel__copy h1 {
  margin: 0;
  color: var(--text-strong);
  font-size: clamp(2rem, 4.8vw, 2.6rem);
  line-height: 1.08;
}

.home-brand-panel__copy p {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.98rem;
  line-height: 1.65;
}

@media (max-width: 980px) {
  .home-page {
    overflow: auto;
  }
}
</style>
