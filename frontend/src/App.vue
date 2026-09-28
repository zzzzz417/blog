<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import PostCard from './components/PostCard.vue'
import AboutView from './views/AboutView.vue'
import PostView from './views/PostView.vue'
import EditorView from './views/EditorView.vue'
import { getPost, getTags, listPosts } from './api'
import { useEditorAuth } from './composables/useEditorAuth'
import type { Post, PostPage, SortOrder, Tag } from './types'

const path = ref(window.location.pathname)
const search = ref(new URLSearchParams(window.location.search).get('q') ?? '')
const tag = ref(new URLSearchParams(window.location.search).get('tag') ?? '')
const tags = ref<Tag[]>([])
const dirty = ref(false)
const { authenticated, refresh: refreshSession, expired } = useEditorAuth()
let lastURL = window.location.pathname + window.location.search
let restoringHistory = false
const sort = ref<SortOrder>(new URLSearchParams(window.location.search).get('sort') === 'oldest' ? 'oldest' : 'newest')
const initialPage = Number(new URLSearchParams(window.location.search).get('page') ?? '1')
const page = ref(Number.isInteger(initialPage) && initialPage > 0 ? initialPage : 1)
const feed = ref<PostPage | null>(null)
const post = ref<Post | null>(null)
const loading = ref(false)
const error = ref('')
let request: AbortController | null = null
let searchTimer: ReturnType<typeof setTimeout> | null = null

const view = computed(() => path.value === '/about' ? 'about' : path.value.startsWith('/editor/') ? 'editor' : path.value.startsWith('/posts/') ? 'post' : 'home')
const slug = computed(() => view.value === 'post' ? path.value.slice('/posts/'.length) : '')
const editingSlug = computed(() => path.value.startsWith('/editor/posts/') ? path.value.slice('/editor/posts/'.length) : '')
const visibleTags = computed(() => tags.value.filter(item => item.count > 0))
const currentYear = new Date().getFullYear()

function navigate(event: MouseEvent, destination: string) {
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) return
  event.preventDefault()
  go(destination)
}

function go(destination: string) {
  if (dirty.value && !window.confirm('还有未保存的内容，确定离开吗？')) return
  dirty.value = false
  if (window.location.pathname !== destination) history.pushState({}, '', destination)
  path.value = destination
  if (destination === '/') syncHomeURL()
  lastURL = window.location.pathname + window.location.search
  window.scrollTo({ top: 0, behavior: 'instant' })
}

function saved(article: Post) { dirty.value = false; go(`/posts/${article.slug}`) }
function deleted() { dirty.value = false; go('/') }
function filterByTag(name: string) {
  tag.value = name; page.value = 1
  if (view.value !== 'home') { search.value = ''; go('/') }
}

async function loadTags() { try { tags.value = await getTags() } catch { /* The feed has its own visible error state. */ } }

async function loadHome() {
  request?.abort()
  const controller = new AbortController()
  request = controller
  loading.value = true
  error.value = ''
  try {
    feed.value = await listPosts(search.value, sort.value, page.value, tag.value, controller.signal)
  } catch (cause) {
    if (cause instanceof DOMException && cause.name === 'AbortError') return
    error.value = cause instanceof Error ? cause.message : '内容暂时无法加载。'
  } finally {
    if (!controller.signal.aborted) loading.value = false
  }
}

async function loadPost() {
  request?.abort()
  const controller = new AbortController()
  request = controller
  loading.value = true
  error.value = ''
  post.value = null
  try {
    post.value = await getPost(slug.value, controller.signal)
  } catch (cause) {
    if (cause instanceof DOMException && cause.name === 'AbortError') return
    error.value = cause instanceof Error ? cause.message : '内容暂时无法加载。'
  } finally {
    if (!controller.signal.aborted) loading.value = false
  }
}

function syncHomeURL() {
  const params = new URLSearchParams()
  if (search.value.trim()) params.set('q', search.value.trim())
  if (tag.value) params.set('tag', tag.value)
  if (sort.value === 'oldest') params.set('sort', 'oldest')
  if (page.value > 1) params.set('page', String(page.value))
  history.replaceState({}, '', `/${params.size ? `?${params}` : ''}`)
  lastURL = window.location.pathname + window.location.search
}

watch(path, () => {
  if (searchTimer) clearTimeout(searchTimer)
  if (view.value === 'home') { loadHome(); loadTags() }
  else if (view.value === 'post') loadPost()
  else { request?.abort(); loading.value = false; error.value = '' }
}, { immediate: true })

watch([sort, tag], () => {
  if (view.value !== 'home') return
  if (!restoringHistory) { page.value = 1; syncHomeURL() }
  loadHome()
})

watch(page, () => {
  if (view.value !== 'home') return
  if (!restoringHistory) syncHomeURL()
  loadHome()
})

watch(search, () => {
  if (view.value !== 'home') return
  if (!restoringHistory) { page.value = 1; syncHomeURL() }
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(loadHome, 280)
})

function onPopState() {
  if (dirty.value && !window.confirm('还有未保存的内容，确定离开吗？')) { history.pushState({}, '', lastURL); return }
  dirty.value = false
  restoringHistory = true
  path.value = window.location.pathname
  search.value = new URLSearchParams(window.location.search).get('q') ?? ''
  tag.value = new URLSearchParams(window.location.search).get('tag') ?? ''
  sort.value = new URLSearchParams(window.location.search).get('sort') === 'oldest' ? 'oldest' : 'newest'
  const requestedPage = Number(new URLSearchParams(window.location.search).get('page') ?? '1')
  page.value = Number.isInteger(requestedPage) && requestedPage > 0 ? requestedPage : 1
  lastURL = window.location.pathname + window.location.search
  nextTick(() => { restoringHistory = false })
}

function beforeUnload(event: BeforeUnloadEvent) { if (dirty.value) { event.preventDefault(); event.returnValue = '' } }
onMounted(() => {
  window.addEventListener('popstate', onPopState)
  window.addEventListener('beforeunload', beforeUnload)
  window.addEventListener('editor-session-expired', expired)
  refreshSession()
})
onUnmounted(() => {
  window.removeEventListener('popstate', onPopState)
  window.removeEventListener('beforeunload', beforeUnload)
  window.removeEventListener('editor-session-expired', expired)
  request?.abort()
  if (searchTimer) clearTimeout(searchTimer)
})
</script>

<template>
  <div class="site-shell">
    <header class="site-header">
      <div class="header-inner">
        <nav aria-label="主导航">
          <a href="/" :class="{ active: view === 'home' || view === 'post' }" :aria-current="view === 'home' ? 'page' : undefined" @click="navigate($event, '/')">文章</a>
          <a href="/about" :class="{ active: view === 'about' }" :aria-current="view === 'about' ? 'page' : undefined" @click="navigate($event, '/about')">关于</a>
          <a v-if="authenticated" href="/editor/new" :class="{ active: view === 'editor' }" @click="navigate($event, '/editor/new')">新增文章</a>
        </nav>
      </div>
    </header>

    <main v-if="view === 'home'" class="content-wrap home-page">
      <section class="feed" aria-label="文章列表">
        <div class="feed-controls">
          <h1 class="feed-heading">
            {{ search.trim() ? '搜索结果' : tag ? '标签文章' : '全部文章' }}
            <span v-if="feed" class="feed-count" aria-live="polite" :aria-label="`${feed.total} 篇文章`">{{ String(feed.total).padStart(2, '0') }}</span>
          </h1>
          <label class="tag-filter"><span class="sr-only">按标签筛选</span><select v-model="tag" aria-label="按标签筛选"><option value="">全部标签</option><option v-for="item in visibleTags" :key="item.name" :value="item.name">{{ item.name }} · {{ item.count }}</option><option v-if="tag && !visibleTags.some(item => item.name === tag)" :value="tag">{{ tag }}</option></select></label>
          <label class="search-field">
            <svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><circle cx="10.8" cy="10.8" r="6.6" stroke="currentColor" stroke-width="1.7" /><path d="m16 16 4.4 4.4" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" /></svg>
            <span class="sr-only">全文搜索</span>
            <input v-model="search" type="search" maxlength="120" placeholder="搜索文章与关键词" autocomplete="off" />
          </label>
          <div class="sort-control" role="group" aria-label="排序顺序">
            <button type="button" :class="{ selected: sort === 'newest' }" :aria-pressed="sort === 'newest'" @click="sort = 'newest'">最新发布</button>
            <button type="button" :class="{ selected: sort === 'oldest' }" :aria-pressed="sort === 'oldest'" @click="sort = 'oldest'">最早发布</button>
          </div>
        </div>

        <div v-if="error" class="status-message" role="alert">{{ error }} <button type="button" @click="loadHome">重试</button></div>
        <div v-else-if="loading && !feed" class="status-message" role="status">正在加载文章…</div>
        <div v-else-if="feed && feed.items.length === 0" class="status-message empty-state">没有找到相关文章。试试别的关键词吧。</div>
        <div v-else-if="feed" class="post-list" :class="{ refreshing: loading }">
          <PostCard v-for="item in feed.items" :key="item.slug" :post="item" @navigate="navigate" @tag="filterByTag" />
        </div>
        <div v-if="feed && feed.total > feed.limit" class="pagination" aria-label="翻页">
          <button type="button" :disabled="page <= 1" @click="page--">上一页</button>
          <span>{{ page }} / {{ Math.ceil(feed.total / feed.limit) }}</span>
          <button type="button" :disabled="page >= Math.ceil(feed.total / feed.limit)" @click="page++">下一页</button>
        </div>
      </section>
    </main>

    <PostView v-else-if="view === 'post'" :post="post" :loading="loading" :error="error" :editor="authenticated" @navigate="navigate" @tag="filterByTag" />
    <EditorView v-else-if="view === 'editor'" :key="path" :slug="editingSlug" @navigate="navigate" @dirty="dirty = $event" @saved="saved" @deleted="deleted" />
    <AboutView v-else @navigate="navigate" />

    <footer class="site-footer content-wrap">
      <span>© {{ currentYear }} Listening</span>
    </footer>
  </div>
</template>
