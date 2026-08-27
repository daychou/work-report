import { describe, expect, it } from 'vitest'
import { normalizeServerURL } from '@work-report/shared'

describe('normalizeServerURL', () => {
  it('为裸域名补齐 HTTPS 与 API 前缀', () => {
    expect(normalizeServerURL('work.example.com/')).toBe('https://work.example.com/api')
  })

  it('保留开发环境 HTTP 地址', () => {
    expect(normalizeServerURL('http://localhost:8092')).toBe('http://localhost:8092/api')
  })

  it('不会重复追加 API 前缀', () => {
    expect(normalizeServerURL('https://work.example.com/api')).toBe('https://work.example.com/api')
  })
})
