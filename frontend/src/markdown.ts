import { marked } from 'marked'
import DOMPurify from 'dompurify'

function safeURL(value: string, link: boolean): boolean {
  const url = value.trim()
  if (/[\u0000-\u0020\\]/.test(url)) return false
  if (/^https?:\/\//i.test(url)) return true
  if (url.startsWith('/') && !url.startsWith('//')) return true
  return link && (/^#[^\s]*$/.test(url) || /^mailto:[^\s]+$/i.test(url))
}

DOMPurify.addHook('uponSanitizeAttribute', (node, data) => {
  if (['href', 'src', 'poster'].includes(data.attrName)) {
    data.keepAttr = safeURL(data.attrValue, node.nodeName === 'A' && data.attrName === 'href')
  }
})

DOMPurify.addHook('afterSanitizeAttributes', node => {
  if (node.nodeName === 'VIDEO') {
    node.setAttribute('controls', '')
    node.setAttribute('playsinline', '')
    node.setAttribute('preload', 'metadata')
  }
  if (node.nodeName === 'IMG') { node.setAttribute('loading', 'lazy'); node.setAttribute('referrerpolicy', 'no-referrer') }
  if (node.nodeName === 'A' && /^https?:/i.test(node.getAttribute('href') || '')) {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noopener noreferrer')
  }
})

// Reader and editor preview share exactly the same parser and sanitizer.
export function renderMarkdown(source: string): string {
  const html = marked.parse(source, { async: false, gfm: true, breaks: false })
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS: ['p', 'br', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'hr', 'strong', 'em', 'del', 's', 'a', 'img', 'video', 'source', 'figure', 'figcaption', 'blockquote', 'pre', 'code', 'kbd', 'ul', 'ol', 'li', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'details', 'summary'],
    ALLOWED_ATTR: ['href', 'src', 'alt', 'title', 'controls', 'playsinline', 'preload', 'poster', 'type', 'start', 'colspan', 'rowspan', 'open', 'class'],
    ALLOW_DATA_ATTR: false,
    RETURN_TRUSTED_TYPE: false,
  })
}
