export type SortOrder = 'newest' | 'oldest'

export interface PostSummary {
  slug: string
  title: string
  summary: string
  category: string
  coverImage: string
  publishedAt: string
  tags: string[]
}

export interface Post extends PostSummary {
  markdown: string
}

export interface PostPage {
  items: PostSummary[]
  total: number
  page: number
  limit: number
}

export interface Tag { name: string; count: number }
export interface PostInput {
  slug: string
  title: string
  summary: string
  category: string
  coverImage: string
  publishedAt: string
  markdown: string
  tags: string[]
}
export interface EditorSession {
  authenticated: boolean
  enabled: boolean
  maxTags: number
  maxUploadBytes: number
  expiresAt?: string
}
export interface MediaAsset { url: string; name: string; type: 'image' | 'video'; size: number }
