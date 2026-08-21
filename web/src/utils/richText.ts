// 富文本内容工具：任务详细内容改为富文本后，列表预览/搜索等场景需要纯文本

// 判断内容是否为富文本 HTML（历史数据是纯文本，需兼容）
export function isHTMLContent(s: string): boolean {
  return /<\w+[^>]*>/.test(s)
}

// 提取纯文本：去标签、压缩空白（列表预览、看板搜索用）
export function stripHTML(s: string): string {
  if (!s || !isHTMLContent(s)) return s || ''
  const doc = new DOMParser().parseFromString(s, 'text/html')
  return (doc.body.textContent || '').replace(/\s+/g, ' ').trim()
}

// 编辑器空内容判定：wangEditor 空内容会产出 <p><br></p> 之类的占位 HTML
export function isEmptyRichText(s: string): boolean {
  return stripHTML(s) === ''
}
