import DOMPurify from 'dompurify'

const meaningfulMediaSelector = 'img,video,audio,iframe,table,hr'

export function isHTMLContent(value?: string | null) {
  return Boolean(value && /<\/?[a-z][\s\S]*?>/i.test(value))
}

export function isEmptyRichText(value?: string | null) {
  if (!value?.trim()) return true
  const sanitized = DOMPurify.sanitize(value)
  const document = new DOMParser().parseFromString(sanitized, 'text/html')
  const text = document.body.textContent?.replace(/\u00a0/g, ' ').trim()
  return !text && !document.body.querySelector(meaningfulMediaSelector)
}

export function sanitizeRichText(value?: string | null) {
  if (!value) return ''
  const sanitized = DOMPurify.sanitize(value)
  const document = new DOMParser().parseFromString(sanitized, 'text/html')

  // 富文本编辑器经常在正文首尾留下空段落；展示时移除，避免看到多余空白。
  document.body.querySelectorAll('p').forEach((paragraph) => {
    const text = paragraph.textContent?.replace(/\u00a0/g, ' ').trim()
    if (!text && !paragraph.querySelector(meaningfulMediaSelector)) paragraph.remove()
  })

  return document.body.innerHTML.trim()
}
