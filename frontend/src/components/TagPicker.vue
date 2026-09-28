<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Tag } from '../types'

const props = defineProps<{ modelValue: string[]; options: Tag[]; max: number }>()
const emit = defineEmits<{ 'update:modelValue': [tags: string[]] }>()
const text = ref('')
const error = ref('')
const normalize = (value: string) => value.trim().replace(/\s+/g, ' ')
const contains = (value: string) => props.modelValue.some(tag => tag.toLocaleLowerCase() === value.toLocaleLowerCase())
const suggestions = computed(() => props.options.filter(tag => !contains(tag.name) && tag.name.toLocaleLowerCase().includes(text.value.trim().toLocaleLowerCase())).slice(0, 16))
function add(value = text.value) {
  const tag = normalize(value)
  error.value = ''
  if (!tag) return
  if (contains(tag)) { text.value = ''; return }
  if ([...tag].length > 24) { error.value = '每个标签最多 24 个字符。'; return }
  if (props.modelValue.length >= props.max) { error.value = `最多添加 ${props.max} 个标签。`; return }
  const existing = props.options.find(item => item.name.toLocaleLowerCase() === tag.toLocaleLowerCase())
  emit('update:modelValue', [...props.modelValue, existing?.name || tag])
  text.value = ''
}
</script>

<template>
  <div class="tag-picker">
    <div class="field-heading"><label for="tag-input">标签 <span class="optional">可选</span></label><span>{{ modelValue.length }} / {{ max }}</span></div>
    <div v-if="modelValue.length" class="tag-list selected-tags"><button v-for="tag in modelValue" :key="tag" type="button" class="tag-chip selected" :aria-label="`移除标签 ${tag}`" @click="emit('update:modelValue', modelValue.filter(value => value !== tag))">{{ tag }} <span aria-hidden="true">×</span></button></div>
    <div class="tag-input-row"><input id="tag-input" v-model="text" maxlength="48" autocomplete="off" placeholder="输入新标签，按 Enter 添加" @keydown.enter.prevent="add()" /><button class="action-button" type="button" :disabled="!text.trim()" @click="add()">添加</button></div>
    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    <div v-if="suggestions.length" class="tag-suggestions"><span class="form-hint">已有标签</span><div class="tag-list"><button v-for="tag in suggestions" :key="tag.name" class="tag-chip" type="button" :disabled="modelValue.length >= max" @click="add(tag.name)">{{ tag.name }}</button></div></div>
  </div>
</template>
