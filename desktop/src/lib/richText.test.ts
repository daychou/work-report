// @vitest-environment jsdom

import { describe, expect, it } from 'vitest'
import { isEmptyRichText, isHTMLContent, sanitizeRichText } from './richText'

describe('richText', () => {
  it('移除富文本首尾空段落但保留正文', () => {
    expect(sanitizeRichText('<p></p><p>任务正文</p><p><br></p>')).toBe('<p>任务正文</p>')
  })

  it('清理危险标签和事件属性', () => {
    const result = sanitizeRichText('<p onclick="alert(1)">安全内容</p><script>alert(1)</script>')
    expect(result).toBe('<p>安全内容</p>')
  })

  it('识别空富文本、HTML 与历史纯文本', () => {
    expect(isEmptyRichText('<p><br></p>')).toBe(true)
    expect(isHTMLContent('<p>正文</p>')).toBe(true)
    expect(isHTMLContent('第一行\n第二行')).toBe(false)
  })
})
