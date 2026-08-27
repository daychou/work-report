// @vitest-environment jsdom

import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import type { Project, User } from '@work-report/shared'
import QuickCreate from './QuickCreate.vue'

const currentUser = { id: 1, name: '林默' } as User
const projects = [{ id: 7, name: '基础架构' } as Project]

function mountPane() {
  return mount(QuickCreate, {
    props: { projects, users: [currentUser], currentUser },
    attachTo: document.body,
  })
}

function statusButton(wrapper: ReturnType<typeof mountPane>, label: string) {
  return wrapper.findAll('.status-choice button').find((button) => button.text() === label)!
}

function dateInput(wrapper: ReturnType<typeof mountPane>, index: number) {
  return wrapper.findAll('input[type="date"]')[index]
}

function dateValue(wrapper: ReturnType<typeof mountPane>, index: number) {
  return (dateInput(wrapper, index).element as HTMLInputElement).value
}

describe('QuickCreate', () => {
  it('默认按进行中提交，带上今天的开始与截止日期', async () => {
    const wrapper = mountPane()
    await wrapper.find('.title-field input').setValue('接入告警链路')
    await wrapper.find('form').trigger('submit')

    const [[input]] = wrapper.emitted('create') as [[Record<string, unknown>]]
    expect(input.status).toBe('doing')
    expect(input.work_date).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    expect(input.due_date).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  })

  it('选择待办后日期显示待定并以空日期提交', async () => {
    const wrapper = mountPane()
    await wrapper.find('.title-field input').setValue('等排期的调研')
    await statusButton(wrapper, '待办').trigger('click')

    expect(wrapper.findAll('.pending-date')).toHaveLength(2)
    expect(wrapper.find('input[type="date"]').exists()).toBe(false)

    await wrapper.find('form').trigger('submit')
    const [[input]] = wrapper.emitted('create') as [[Record<string, unknown>]]
    expect(input.status).toBe('todo')
    expect(input.work_date).toBe('')
    expect(input.due_date).toBe('')
    expect(input.due_remind).toBe(false)
  })

  it('从待办切回进行中会恢复日期', async () => {
    const wrapper = mountPane()
    await statusButton(wrapper, '待办').trigger('click')
    await statusButton(wrapper, '进行中').trigger('click')

    expect(wrapper.findAll('.pending-date')).toHaveLength(0)
    expect(wrapper.findAll('input[type="date"]')).toHaveLength(2)
  })

  it('提醒勾选常驻表单，无需展开更多选项', async () => {
    const wrapper = mountPane()
    const reminders = wrapper.findAll('.remind-row label')
    expect(reminders).toHaveLength(1)
    expect(reminders[0].text()).toContain('截止日提醒')
    expect(reminders[0].find('input').element.checked).toBe(true)
  })

  it('开始日期选到未来时露出并默认勾选开始日提醒', async () => {
    const wrapper = mountPane()
    const future = new Date(Date.now() + 5 * 86_400_000).toISOString().slice(0, 10)
    await wrapper.findAll('input[type="date"]')[0].setValue(future)

    const labels = wrapper.findAll('.remind-row label')
    expect(labels.map((label) => label.text())).toEqual([
      expect.stringContaining('截止日提醒'),
      expect.stringContaining('开始日提醒'),
    ])

    await wrapper.find('.title-field input').setValue('下周开工')
    await wrapper.find('form').trigger('submit')
    const [[input]] = wrapper.emitted('create') as [[Record<string, unknown>]]
    expect(input.start_remind).toBe(true)
  })

  it('开始日期晚于截止日期时把截止日期顺延', async () => {
    const wrapper = mountPane()
    const future = new Date(Date.now() + 9 * 86_400_000).toISOString().slice(0, 10)
    await dateInput(wrapper, 0).setValue(future)

    expect(dateValue(wrapper, 1)).toBe(future)
  })

  it('截止日期早于开始日期时把开始日期提前', async () => {
    const wrapper = mountPane()
    const past = new Date(Date.now() - 6 * 86_400_000).toISOString().slice(0, 10)
    await dateInput(wrapper, 1).setValue(past)

    expect(dateValue(wrapper, 0)).toBe(past)
  })

  it('待办状态提醒区提示暂不排期', async () => {
    const wrapper = mountPane()
    await statusButton(wrapper, '待办').trigger('click')

    expect(wrapper.findAll('.remind-row')).toHaveLength(0)
    expect(wrapper.find('.remind-idle').text()).toContain('暂不排期')
  })

  it('详细内容折叠区展开后随任务一起提交', async () => {
    const wrapper = mountPane()
    await wrapper.find('.title-field input').setValue('带细节的任务')
    await wrapper.find('.detail-field-toggle').trigger('click')
    await wrapper.find('.detail-field-body textarea').setValue('第一行\n第二行')
    await wrapper.find('form').trigger('submit')

    const [[input]] = wrapper.emitted('create') as [[Record<string, unknown>]]
    expect(input.detail).toBe('第一行\n第二行')
  })
})
