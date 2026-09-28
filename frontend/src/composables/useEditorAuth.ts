import { computed, ref } from 'vue'
import { getSession, login, logout } from '../api'
import type { EditorSession } from '../types'

const session = ref<EditorSession>({ authenticated: false, enabled: false, maxTags: 10, maxUploadBytes: 100 * 1024 * 1024 })
const ready = ref(false)
const error = ref('')
let expiryTimer: ReturnType<typeof setTimeout> | undefined

export function useEditorAuth() {
  async function refresh() {
    clearTimeout(expiryTimer)
    try {
      session.value = await getSession(); error.value = ''
      if (session.value.authenticated && session.value.expiresAt) {
        expiryTimer = setTimeout(expired, Math.max(0, Date.parse(session.value.expiresAt) - Date.now()))
      }
    }
    catch { expired(); error.value = '无法连接编辑服务，请稍后重试。' }
    finally { ready.value = true }
  }
  async function signIn(key: string) {
    await login(key); await refresh()
    if (!session.value.authenticated) throw new Error(error.value || '登录状态未建立，请检查 Cookie 设置后重试。')
  }
  async function signOut() { await logout(); expired() }
  function expired() { clearTimeout(expiryTimer); session.value = { ...session.value, authenticated: false, expiresAt: undefined } }
  return { session, ready, error, authenticated: computed(() => session.value.authenticated), refresh, signIn, signOut, expired }
}
