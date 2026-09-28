<script setup lang="ts">
import { ref } from 'vue'
import { useEditorAuth } from '../composables/useEditorAuth'

const emit = defineEmits<{ success: [] }>()
const auth = useEditorAuth()
const key = ref('')
const busy = ref(false)
const error = ref('')
async function submit() {
  busy.value = true; error.value = ''
  try { await auth.signIn(key.value); key.value = ''; emit('success') }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '登录失败' }
  finally { busy.value = false }
}
</script>

<template>
  <form class="login-form" @submit.prevent="submit">
    <label class="field">编辑密钥<input v-model="key" type="password" autocomplete="current-password" required :disabled="busy" placeholder="输入后端配置的编辑密钥" /></label>
    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    <button class="action-button primary" type="submit" :disabled="busy || !key">{{ busy ? '登录中…' : '登录为编辑者' }}</button>
  </form>
</template>
