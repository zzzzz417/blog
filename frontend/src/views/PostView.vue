<script setup lang="ts">
import { computed } from 'vue'
import ArrowIcon from '../components/ArrowIcon.vue'
import { renderMarkdown } from '../markdown'
import type { Post } from '../types'

const props = defineProps<{ post: Post | null; loading: boolean; error: string; editor: boolean }>()
const emit = defineEmits<{ navigate: [event: MouseEvent, path: string]; tag: [name: string] }>()
const html = computed(() => props.post ? renderMarkdown(props.post.markdown) : '')
const date = computed(() => props.post ? new Intl.DateTimeFormat('zh-CN', {dateStyle:'long', timeZone:'UTC'}).format(new Date(props.post.publishedAt)) : '')
</script>

<template>
  <main class="content-wrap detail-page">
    <div class="page-actions">
      <a class="back-link" href="/" @click="emit('navigate', $event, '/')"><ArrowIcon direction="left" />返回文章</a>
      <a v-if="editor && post" class="action-button" :href="`/editor/posts/${post.slug}`" @click="emit('navigate', $event, `/editor/posts/${post.slug}`)">编辑文章</a>
    </div>
    <div v-if="error" class="status-message" role="alert">{{ error }}</div>
    <div v-else-if="loading || !post" class="status-message" role="status">正在加载文章…</div>
    <article v-else>
      <div class="detail-meta"><time :datetime="post.publishedAt">{{ date }}</time><span>{{ post.category }}</span></div>
      <h1>{{ post.title }}</h1>
      <div v-if="post.tags.length" class="tag-list detail-tags" aria-label="文章标签"><button v-for="tag in post.tags" :key="tag" class="tag-chip" @click="emit('tag', tag)">{{ tag }}</button></div>
      <div class="markdown-body" v-html="html"></div>
    </article>
  </main>
</template>
