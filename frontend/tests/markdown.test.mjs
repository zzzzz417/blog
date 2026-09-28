import test from 'node:test'
import assert from 'node:assert/strict'
import { JSDOM } from 'jsdom'

const window = new JSDOM('', { url: 'http://localhost:5173/' }).window
globalThis.window = window
const { renderMarkdown } = await import('../src/markdown.ts')
const documentFor = source => new JSDOM(renderMarkdown(source)).window.document
const fence = String.fromCharCode(96).repeat(3)

test('Markdown preserves multiple images/videos in content order', () => {
  const source = [
    '# 标题', '第一段', '![山](/images/mountain.jpg)', '中间文字',
    '<video src="/media/a.mp4"></video>', '![海](/images/sea.png)', '另一段',
    '<video controls><source src="/media/b.webm" type="video/webm"></video>', '末尾文字',
  ].join('\n\n')
  const document = documentFor(source)
  assert.deepEqual([...document.querySelectorAll('img,video')].map(node=>node.tagName), ['IMG','VIDEO','IMG','VIDEO'])
  for (const video of document.querySelectorAll('video')) {
    assert.ok(video.hasAttribute('controls'))
    assert.equal(video.getAttribute('preload'), 'metadata')
  }
  assert.ok(document.body.textContent.includes('中间文字'))
  assert.ok(document.body.textContent.includes('末尾文字'))
})

test('tables, nested lists and fenced code use a real Markdown parser', () => {
  const document = documentFor('| A | B |\n| --- | --- |\n| 1 | 2 |\n\n- outer\n  - inner\n\n' + fence + 'html\n<script>literal example</script>\n' + fence)
  assert.equal(document.querySelectorAll('table tbody tr').length, 1)
  assert.equal(document.querySelectorAll('ul ul li').length, 1)
  assert.equal(document.querySelector('pre code').textContent.trim(), '<script>literal example</script>')
  assert.equal(document.querySelectorAll('script').length, 0)
})

test('active HTML and unsafe URLs are removed from reader and preview', () => {
  const document = documentFor('<script>alert(1)</script><iframe src="https://example.com"></iframe><style>body{display:none}</style><svg onload="alert(1)"></svg>\n\n<img src="javascript:alert(1)" onerror="alert(1)"><video src="data:text/html,bad" autoplay onplay="alert(1)"></video>\n\n[危险](javascript:alert(1))\n\n[网站](https://example.com)')
  assert.equal(document.querySelectorAll('script,iframe,style,svg').length, 0)
  for (const element of document.querySelectorAll('*')) {
    for (const attribute of element.attributes) {
      assert.ok(!/^on/i.test(attribute.name))
      assert.ok(!/^(javascript|data):/i.test(attribute.value))
    }
  }
  assert.equal(document.querySelector('video').hasAttribute('autoplay'), false)
  const link = document.querySelector('a[href="https://example.com"]')
  assert.equal(link.getAttribute('rel'), 'noopener noreferrer')
})

test('code fences containing media remain code rather than players', () => {
  const document = documentFor(fence + 'html\n<video src="/videos/a.mp4"></video>\n' + fence)
  assert.equal(document.querySelectorAll('video').length, 0)
  assert.ok(document.querySelector('code').textContent.includes('<video'))
})
