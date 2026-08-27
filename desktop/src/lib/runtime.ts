import { invoke } from '@tauri-apps/api/core'
import { getCurrentWindow } from '@tauri-apps/api/window'
import { openUrl } from '@tauri-apps/plugin-opener'

export interface DesktopPreferences {
  server_url: string
  recent_project_id?: number | null
  launch_at_startup?: boolean
}

const defaults: DesktopPreferences = {
  server_url: '',
  recent_project_id: null,
  launch_at_startup: false,
}

let browserKey = ''

export function isTauriRuntime() {
  return '__TAURI_INTERNALS__' in window
}

export async function loadPreferences(): Promise<DesktopPreferences> {
  if (isTauriRuntime()) {
    return { ...defaults, ...(await invoke<DesktopPreferences>('get_preferences')) }
  }
  const saved = localStorage.getItem('work-report-desktop-preferences')
  return saved ? { ...defaults, ...JSON.parse(saved) } : { ...defaults }
}

export async function savePreferences(preferences: DesktopPreferences) {
  if (isTauriRuntime()) {
    await invoke('set_preferences', { preferences })
    return
  }
  localStorage.setItem('work-report-desktop-preferences', JSON.stringify(preferences))
}

export async function saveAPIKey(apiKey: string) {
  if (isTauriRuntime()) {
    await invoke('save_api_key', { apiKey })
    return
  }
  browserKey = apiKey
}

export async function loadAPIKey(): Promise<string> {
  if (isTauriRuntime()) return invoke<string>('get_api_key')
  return browserKey
}

export async function hasAPIKey(): Promise<boolean> {
  if (isTauriRuntime()) return invoke<boolean>('has_api_key')
  return browserKey.length > 0
}

export async function clearAPIKey() {
  if (isTauriRuntime()) {
    await invoke('delete_api_key')
    return
  }
  browserKey = ''
}

export async function hideWindow() {
  if (isTauriRuntime()) await getCurrentWindow().hide()
}

export async function showWindow() {
  if (isTauriRuntime()) {
    const window = getCurrentWindow()
    await window.show()
    await window.setFocus()
  }
}

export async function openExternal(url: string) {
  if (isTauriRuntime()) await openUrl(url)
  else window.open(url, '_blank', 'noopener,noreferrer')
}

export async function setLaunchAtStartup(enabled: boolean) {
  if (isTauriRuntime()) await invoke('set_launch_at_startup', { enabled })
}
