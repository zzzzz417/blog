<script setup lang="ts">
defineProps<{ direction?: 'left' | 'right' }>()
</script>

<template>
  <svg class="arrow-icon" :class="{ 'arrow-icon--left': direction === 'left' }" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <path d="M4 12h15m-5-5 5 5-5 5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
  </svg>
</template>
