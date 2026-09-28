import type { EditorSession, MediaAsset, Post, PostInput, PostPage, SortOrder, Tag } from './types'

export class APIError extends Error {
  constructor(message: string, public status: number) { super(message) }
}

function apiError(status: number, message?: string) {
  if (status === 401) window.dispatchEvent(new Event('editor-session-expired'))
  return new APIError(message || (status === 404 ? '没有找到这篇文章。' : '操作失败，请稍后再试。'), status)
}

async function request<T>(url: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(url, { credentials: 'same-origin', ...init })
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as { error?: string }
    throw apiError(response.status, body.error)
  }
  if (response.status === 204) return undefined as T
  return await response.json() as T
}

const json = (method: string, body: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })

export function listPosts(search: string, sort: SortOrder, page: number, tag = '', signal?: AbortSignal) {
  const params = new URLSearchParams({ sort, page: String(page), limit: '10' })
  if (search.trim()) params.set('q', search.trim())
  if (tag) params.set('tag', tag)
  return request<PostPage>('/api/v1/posts?' + params, { signal })
}

export const getPost = (slug: string, signal?: AbortSignal) => request<Post>('/api/v1/posts/' + encodeURIComponent(slug), { signal })
export const getTags = () => request<Tag[]>('/api/v1/tags')
export const getSession = () => request<EditorSession>('/api/v1/auth/session')
export const login = (key: string) => request<{ expiresAt: string }>('/api/v1/auth/login', json('POST', { key }))
export const logout = () => request<void>('/api/v1/auth/logout', { method: 'POST' })
export const savePost = (post: PostInput, create: boolean) => request<Post>(create ? '/api/v1/posts' : '/api/v1/posts/' + encodeURIComponent(post.slug), json(create ? 'POST' : 'PUT', post))
export const deletePost = (slug: string) => request<void>('/api/v1/posts/' + encodeURIComponent(slug), { method: 'DELETE' })

export function uploadMedia(file: File, onProgress: (progress: number) => void, signal?: AbortSignal): Promise<MediaAsset> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    const abort = () => xhr.abort()
    xhr.open('POST', '/api/v1/media')
    xhr.withCredentials = true
    xhr.upload.onprogress = event => { if (event.lengthComputable) onProgress(Math.round(event.loaded / event.total * 100)) }
    xhr.onload = () => {
      try {
        const body = JSON.parse(xhr.responseText)
        if (xhr.status >= 200 && xhr.status < 300) resolve(body as MediaAsset)
        else reject(apiError(xhr.status, body.error))
      } catch { reject(new Error('上传响应异常，请重试。')) }
    }
    xhr.onerror = () => reject(new Error('上传连接中断，请重试。'))
    xhr.onabort = () => reject(new DOMException('上传已取消', 'AbortError'))
    xhr.onloadend = () => signal?.removeEventListener('abort', abort)
    if (signal?.aborted) { reject(new DOMException('上传已取消', 'AbortError')); return }
    signal?.addEventListener('abort', abort, { once: true })
    const form = new FormData()
    form.append('file', file)
    xhr.send(form)
  })
}
