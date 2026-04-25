<script setup>
import ImageTools from "./components/ImageTools.vue";
</script>

<template>
  <main class="app-shell">
    <div class="app-shell__glow app-shell__glow--warm"></div>
    <div class="app-shell__glow app-shell__glow--cool"></div>

    <section class="app-frame">
      <ImageTools />
    </section>
  </main>
</template>

<style scoped>
.app-shell {
  position: relative;
  height: 100dvh;
  overflow: hidden;
}

.app-shell__glow {
  position: fixed;
  inset: auto;
  pointer-events: none;
  filter: blur(90px);
  opacity: 0.6;
}

.app-shell__glow--warm {
  top: -6rem;
  left: -4rem;
  width: 18rem;
  height: 18rem;
  background: rgba(251, 191, 36, 0.2);
}

.app-shell__glow--cool {
  right: -4rem;
  bottom: 6rem;
  width: 22rem;
  height: 22rem;
  background: rgba(59, 130, 246, 0.16);
}

.app-frame {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 100%;
  height: 100%;
  padding: 24px;
  overflow: hidden;
}

.app-frame > * {
  min-height: 0;
}
@media (max-width: 720px) {
  .app-frame {
    padding: 16px;
  }
}
</style>
