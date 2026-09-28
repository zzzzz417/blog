<script setup lang="ts">
import type { PostSummary } from '../types'
import ArrowIcon from './ArrowIcon.vue'

defineProps<{ post: PostSummary }>()
const emit = defineEmits<{ navigate: [event: MouseEvent, path: string]; tag: [name: string] }>()

const formatDate = (date: string) => new Date(date).toISOString().slice(0, 10).replaceAll('-', '.')
</script>

<template>
  <article class="post-row" :class="{ 'post-row--text': !post.coverImage }">
    <div class="post-copy">
      <div class="post-meta"><time :datetime="post.publishedAt">{{ formatDate(post.publishedAt) }}</time><span>{{ post.category }}</span></div>
      <h2><a :href="`/posts/${post.slug}`" @click="emit('navigate', $event, `/posts/${post.slug}`)">{{ post.title }}</a></h2>
      <p>{{ post.summary }}</p>
      <div v-if="post.tags.length" class="tag-list card-tags" aria-label="文章标签"><button v-for="tag in post.tags" :key="tag" class="tag-chip" @click="emit('tag', tag)">{{ tag }}</button></div>
      <a class="read-link" :href="`/posts/${post.slug}`" @click="emit('navigate', $event, `/posts/${post.slug}`)">
        阅读全文
        <ArrowIcon />
      </a>
    </div>
    <a v-if="post.coverImage" class="post-cover" :href="`/posts/${post.slug}`" :aria-label="`阅读${post.title}`"
      @click="emit('navigate', $event, `/posts/${post.slug}`)">
      <img v-if="post.coverImage" :src="post.coverImage" :alt="post.title" loading="lazy" />
    </a>
  </article>
</template>
