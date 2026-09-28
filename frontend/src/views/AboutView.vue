<script setup lang="ts">
import { ref } from 'vue'
import ArrowIcon from '../components/ArrowIcon.vue'
import EditorLogin from '../components/EditorLogin.vue'
import { useEditorAuth } from '../composables/useEditorAuth'

const emit = defineEmits<{ navigate: [event: MouseEvent, path: string] }>()
const { authenticated, ready, session, error: connectionError, refresh, signOut } = useEditorAuth()
const showLogin = ref(false)
const error = ref('')
async function leaveEditor() {
  try { await signOut(); showLogin.value = false; error.value = '' }
  catch { error.value = '退出失败，请重试。' }
}
</script>

<template>
  <main class="content-wrap about-page">
    <a class="back-link" href="/" @click="emit('navigate', $event, '/')"><ArrowIcon direction="left" />返回文章</a>
    <h1>关于 Listening</h1>
    <p>用文字、照片与影像，记录走过的路、遇见的风景，以及日常里值得珍藏的小事。</p>
    <section class="editor-access" aria-label="编辑者入口">
      <template v-if="authenticated">
        <p class="editor-status">编辑者模式</p>
        <div class="action-row"><a class="action-button primary" href="/editor/new" @click="emit('navigate', $event, '/editor/new')">新增文章</a><button class="action-button" @click="leaveEditor">退出编辑</button></div>
      </template>
      <template v-else>
        <button v-if="!showLogin" class="action-button" :disabled="!ready" @click="showLogin = true">编辑</button>
        <template v-else>
          <p v-if="connectionError" class="form-error" role="alert">{{ connectionError }} <button class="text-button" @click="refresh">重试</button></p>
          <p v-else-if="!session.enabled" class="form-hint">编辑功能尚未启用，请先配置后端的编辑密钥。</p>
          <EditorLogin v-else @success="showLogin = false" />
        </template>
      </template>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    </section>
  </main>
</template>
