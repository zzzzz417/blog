<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import ArrowIcon from '../components/ArrowIcon.vue'
import EditorLogin from '../components/EditorLogin.vue'
import TagPicker from '../components/TagPicker.vue'
import { deletePost, getPost, getTags, savePost, uploadMedia } from '../api'
import { useEditorAuth } from '../composables/useEditorAuth'
import { renderMarkdown } from '../markdown'
import type { Post, PostInput, Tag } from '../types'

const props = defineProps<{ slug: string }>()
const emit = defineEmits<{ navigate: [event: MouseEvent, path: string]; saved: [post: Post]; deleted: []; dirty: [value: boolean] }>()
const { authenticated, session, ready, error: connectionError, refresh } = useEditorAuth()
const today = new Date(Date.now() - new Date().getTimezoneOffset() * 60000).toISOString().slice(0, 10)
const draft = reactive<PostInput>({ slug: 'post-' + Date.now().toString(36), title: '', summary: '', category: '', publishedAt: today, coverImage: '', markdown: '', tags: [] })
const tags = ref<Tag[]>([])
const loading = ref(true)
const loaded = ref(false)
const error = ref('')
const busy = ref(false)
const uploading = ref(false)
const progress = ref(0)
const uploadLabel = ref('')
const confirmDelete = ref(false)
const initial = ref('')
const textarea = ref<HTMLTextAreaElement | null>(null)
const mediaInput = ref<HTMLInputElement | null>(null)
const coverInput = ref<HTMLInputElement | null>(null)
const uploadController = new AbortController()
const loadController = new AbortController()
const html = computed(() => renderMarkdown(draft.markdown))
const dirty = computed(() => loaded.value && (JSON.stringify(draft) !== initial.value || uploading.value))
const disabled = computed(() => !authenticated.value || busy.value || uploading.value)
const bodyImages = computed(() => {
  const doc = new DOMParser().parseFromString(html.value, 'text/html')
  return [...new Set([...doc.querySelectorAll('img')].map(image => image.getAttribute('src') || '').filter(url => /^\/(images|media)\//.test(url)))]
})
watch(dirty, value => emit('dirty', value), { flush: 'sync' })

onMounted(async () => {
  try {
    const [article, availableTags] = await Promise.all([props.slug ? getPost(props.slug, loadController.signal) : Promise.resolve(null), getTags()])
    if (loadController.signal.aborted) return
    tags.value = availableTags
    if (article) Object.assign(draft, { slug: article.slug, title: article.title, summary: article.summary, category: article.category, publishedAt: article.publishedAt.slice(0, 10), coverImage: article.coverImage, markdown: article.markdown, tags: [...article.tags] })
    initial.value = JSON.stringify(draft)
    loaded.value = true
  } catch (cause) { if (!loadController.signal.aborted) error.value = cause instanceof Error ? cause.message : '文章加载失败。' }
  finally { loading.value = false }
})
onUnmounted(() => { uploadController.abort(); loadController.abort() })

async function save() {
  if (disabled.value) return
  busy.value = true; error.value = ''
  try {
    const input = { ...draft, tags: [...draft.tags] }
    if (!input.summary.trim()) {
      input.summary = (new DOMParser().parseFromString(html.value, 'text/html').body.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 160)
    }
    const article = await savePost(input, !props.slug)
    initial.value = JSON.stringify(draft)
    emit('dirty', false)
    emit('saved', article)
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败，内容仍保留在编辑器中。' }
  finally { busy.value = false }
}

async function remove() {
  if (!props.slug || disabled.value) return
  busy.value = true; error.value = ''
  try { await deletePost(props.slug); initial.value = JSON.stringify(draft); emit('dirty', false); emit('deleted') }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '删除失败。'; confirmDelete.value = false }
  finally { busy.value = false }
}

async function filesSelected(event: Event, cover = false) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  await upload(files, cover)
}

async function upload(files: File[], cover = false) {
  if (!files.length || disabled.value) return
  const chosen = cover ? files.slice(0, 1) : files
  const tooLarge = chosen.find(file => file.size > session.value.maxUploadBytes)
  if (tooLarge) { error.value = `“${tooLarge.name}”超过 ${Math.round(session.value.maxUploadBytes / 1024 / 1024)} MB 限制。`; return }
  if (cover && !chosen[0]?.type.startsWith('image/')) { error.value = '封面请选择图片。'; return }
  let start = textarea.value?.selectionStart ?? draft.markdown.length
  let end = textarea.value?.selectionEnd ?? start
  uploading.value = true; error.value = ''
  try {
    for (let i = 0; i < chosen.length; i++) {
      const file = chosen[i]!
      progress.value = 0
      uploadLabel.value = `正在上传 ${i + 1} / ${chosen.length}：${file.name}`
      const asset = await uploadMedia(file, value => { progress.value = value }, uploadController.signal)
      if (cover) { draft.coverImage = asset.url; continue }
      const caption = file.name.replace(/\.[^.]+$/, '').replace(/[\[\]<>\\]/g, '')
      const media = asset.type === 'image' ? `![${caption}](${asset.url})` : `<video controls playsinline preload="metadata" src="${asset.url}"></video>`
      const insertion = '\n\n' + media + '\n\n'
      draft.markdown = draft.markdown.slice(0, start) + insertion + draft.markdown.slice(end)
      start += insertion.length; end = start
    }
  } catch (cause) {
    if (!(cause instanceof DOMException && cause.name === 'AbortError')) error.value = cause instanceof Error ? cause.message : '上传失败。已插入的内容会保留。'
  } finally {
    uploading.value = false
    if (!cover && !uploadController.signal.aborted) { await nextTick(); textarea.value?.focus(); textarea.value?.setSelectionRange(start, end) }
  }
}

function paste(event: ClipboardEvent) {
  const files = Array.from(event.clipboardData?.files || [])
  if (files.length) { event.preventDefault(); upload(files) }
}

function drop(event: DragEvent) {
  const files = Array.from(event.dataTransfer?.files || [])
  if (files.length) { event.preventDefault(); upload(files) }
}
</script>

<template>
  <main class="content-wrap editor-page">
    <a class="back-link" :href="slug ? `/posts/${slug}` : '/'" @click="emit('navigate', $event, slug ? `/posts/${slug}` : '/')"><ArrowIcon direction="left" />{{ slug ? '返回文章' : '返回列表' }}</a>
    <div class="editor-heading"><h1>{{ slug ? '编辑文章' : '新增文章' }}</h1><span v-if="dirty" class="form-hint">有未保存的修改</span></div>
    <section v-if="!authenticated" class="editor-auth-notice" aria-label="编辑者登录">
      <p v-if="!ready">正在检查登录状态…</p>
      <template v-else-if="connectionError"><p class="form-error">{{ connectionError }}</p><button class="action-button" @click="refresh">重试</button></template>
      <p v-else-if="!session.enabled">请先在后端配置编辑密钥，启用编辑功能。</p>
      <template v-else><p>登录为编辑者后即可保存文章。当前填写的内容会保留。</p><EditorLogin /></template>
    </section>
    <p v-if="loading" class="status-message" role="status">正在准备编辑器…</p>
    <p v-if="error" class="form-error editor-error" role="alert">{{ error }}</p>
    <form v-if="loaded" @submit.prevent="save">
      <fieldset :disabled="disabled" class="editor-fields">
        <label class="field title-field">标题<input v-model="draft.title" name="title" required maxlength="160" placeholder="为这篇文章起个标题" /></label>
        <div class="metadata-grid">
          <label class="field">文章标识<input v-model="draft.slug" name="slug" required maxlength="100" pattern="[a-z0-9]+(-[a-z0-9]+)*" :readonly="!!slug" placeholder="例如 my-first-post" /><span class="form-hint">小写字母、数字和连字符，发布后固定。</span></label>
          <label class="field">发布日期<input v-model="draft.publishedAt" type="date" name="publishedAt" required /></label>
          <label class="field"><span>分类 <span class="optional">可选</span></span><input v-model="draft.category" maxlength="30" placeholder="随记" /></label>
        </div>
        <label class="field"><span>摘要 <span class="optional">可选</span></span><input v-model="draft.summary" maxlength="500" placeholder="留空时从正文自动提取" /></label>
        <TagPicker v-model="draft.tags" :options="tags" :max="session.maxTags" />
        <div class="cover-field">
          <div class="field-heading"><span>封面 <span class="optional">可选，仅显示在文章列表</span></span></div>
          <div class="cover-actions"><img v-if="draft.coverImage" :src="draft.coverImage" class="cover-preview" alt="当前封面" /><button class="action-button" type="button" @click="coverInput?.click()">{{ draft.coverImage ? '更换封面' : '上传封面' }}</button><button v-if="draft.coverImage" class="text-button" type="button" @click="draft.coverImage = ''">移除封面</button></div>
          <input ref="coverInput" class="sr-only" type="file" accept="image/jpeg,image/png,image/gif,image/webp" aria-label="选择封面文件" @change="filesSelected($event, true)" />
          <div v-if="bodyImages.length" class="cover-candidates"><span class="form-hint">也可以选择正文中的图片</span><div class="cover-thumbnails"><button v-for="(url, index) in bodyImages" :key="url" type="button" :class="{ selected: draft.coverImage === url }" :aria-label="`选择正文图片 ${index + 1} 作为封面`" :aria-pressed="draft.coverImage === url" @click="draft.coverImage = url"><img :src="url" alt="" loading="lazy" /></button></div></div>
        </div>
        <div class="editor-toolbar"><label for="markdown-source">Markdown 正文</label><button class="action-button" type="button" @click="mediaInput?.click()">插入图片 / 视频</button></div>
        <input ref="mediaInput" class="sr-only" type="file" multiple accept="image/jpeg,image/png,image/gif,image/webp,video/mp4,video/webm" aria-label="上传正文图片或视频" @change="filesSelected($event)" />
        <p class="form-hint">可多选、拖入或粘贴图片与视频，上传后插入光标处。单个文件不超过 {{ Math.round(session.maxUploadBytes / 1024 / 1024) }} MB。</p>
      </fieldset>
      <div v-if="uploading" class="upload-status" role="status"><span>{{ uploadLabel }} · {{ progress }}%</span><progress :value="progress" max="100"></progress></div>
      <div class="editor-panes">
        <textarea id="markdown-source" ref="textarea" v-model="draft.markdown" class="markdown-source" required :disabled="disabled" spellcheck="false" placeholder="从一段文字开始…" @paste="paste" @dragover.prevent @drop="drop"></textarea>
        <section class="preview-pane" aria-label="文章实时预览"><div class="preview-label">实时预览</div><p v-if="!draft.markdown.trim()" class="form-hint">正文预览会显示在这里。</p><div v-else class="markdown-body" v-html="html"></div></section>
      </div>
      <div class="editor-savebar"><button class="action-button primary" type="submit" :disabled="disabled">{{ busy ? '保存中…' : slug ? '保存修改' : '发布文章' }}</button><button v-if="slug" class="text-button danger" type="button" :disabled="disabled" @click="confirmDelete = true">删除文章</button></div>
      <div v-if="confirmDelete" class="delete-confirmation" role="alertdialog" aria-labelledby="delete-title"><h2 id="delete-title">删除“{{ draft.title }}”？</h2><p>文章和对应的 Markdown 文件将被删除，已上传的媒体会保留。</p><div class="action-row"><button class="action-button" type="button" :disabled="busy" @click="confirmDelete = false">取消</button><button class="action-button destructive" type="button" :disabled="disabled" @click="remove">确认删除</button></div></div>
    </form>
  </main>
</template>
