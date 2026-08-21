<script lang="ts">
import { Boot } from '@wangeditor/editor'
import attachmentModule from '@wangeditor/plugin-upload-attachment'
import '@wangeditor/editor/dist/css/style.css'

// 附件上传模块全局注册一次（HMR 重复加载时用标记防重复注册）
declare global {
  interface Window {
    __wangeditorAttachmentRegistered?: boolean
  }
}
if (!window.__wangeditorAttachmentRegistered) {
  Boot.registerModule(attachmentModule)
  window.__wangeditorAttachmentRegistered = true
}
</script>

<script setup lang="ts">
import { onBeforeUnmount, shallowRef } from 'vue'
import { Editor, Toolbar } from '@wangeditor/editor-for-vue'
import type { IDomEditor, IEditorConfig, IToolbarConfig } from '@wangeditor/editor'
import { useMessage } from 'naive-ui'
import { api } from '../api'

const props = withDefaults(defineProps<{ modelValue: string; placeholder?: string }>(), {
  placeholder: '详细内容（可选），支持插入图片和附件',
})
const emit = defineEmits<{ 'update:modelValue': [string] }>()

const message = useMessage()
const editorRef = shallowRef<IDomEditor>()

// 与后端 handler.MaxUploadSize 一致
const MAX_SIZE = 500 * 1024 * 1024

async function upload(file: File): Promise<{ url: string; name: string } | null> {
  if (file.size > MAX_SIZE) {
    message.warning('附件大小不能超过 500M')
    return null
  }
  const loading = message.loading(`正在上传：${file.name}`, { duration: 0 })
  try {
    const { data } = await api.uploadFile(file, (p) => {
      loading.content = `正在上传：${file.name}（${p}%）`
    })
    return data
  } catch (e: any) {
    message.error(e.response?.data?.error || '上传失败，请稍后重试')
    return null
  } finally {
    loading.destroy()
  }
}

// 工具栏：去掉视频菜单组，追加「上传附件」
const toolbarConfig: Partial<IToolbarConfig> = {
  excludeKeys: ['group-video'],
  insertKeys: { index: 23, keys: ['uploadAttachment'] },
}

const editorConfig: Partial<IEditorConfig> = {
  placeholder: props.placeholder,
  // 选中附件节点时悬浮「下载附件」菜单
  hoverbarKeys: {
    attachment: { menuKeys: ['downloadAttachment'] },
  } as IEditorConfig['hoverbarKeys'],
  MENU_CONF: {
    uploadImage: {
      maxFileSize: MAX_SIZE,
      async customUpload(file: File, insertFn: (url: string, alt: string, href: string) => void) {
        const res = await upload(file)
        if (res) insertFn(res.url, file.name, res.url)
      },
    },
    uploadAttachment: {
      maxFileSize: MAX_SIZE,
      async customUpload(file: File, insertFn: (fileName: string, link: string) => void) {
        const res = await upload(file)
        if (res) insertFn(res.name, res.url)
      },
    },
  } as IEditorConfig['MENU_CONF'],
}

function handleCreated(editor: IDomEditor) {
  editorRef.value = editor
}

onBeforeUnmount(() => {
  editorRef.value?.destroy()
  editorRef.value = undefined
})
</script>

<template>
  <div class="rich-editor overflow-hidden rounded-lg border border-slate-200 dark:border-[#2a2e39]">
    <Toolbar :editor="editorRef" :default-config="toolbarConfig" mode="default" class="rich-toolbar" />
    <Editor
      :model-value="modelValue"
      :default-config="editorConfig"
      mode="default"
      @on-created="handleCreated"
      @update:model-value="emit('update:modelValue', $event)"
    />
  </div>
</template>

<style>
/* 编辑器内容区最小高度（组件级样式，wangEditor 类名是全局的，不做 scoped） */
.rich-editor .w-e-text-container [data-slate-editor] {
  min-height: 200px;
}
.rich-editor .w-e-toolbar {
  border-bottom: 1px solid #e2e8f0;
}
/* 深色模式适配（wangEditor 通过 CSS 变量定制） */
html.dark .rich-editor {
  --w-e-toolbar-bg-color: #171a20;
  --w-e-toolbar-color: #c7cad1;
  --w-e-toolbar-active-bg-color: #23262f;
  --w-e-toolbar-active-color: #c4b5fd;
  --w-e-toolbar-disabled-color: #4b5060;
  --w-e-textarea-bg-color: #12151b;
  --w-e-textarea-color: #c7cad1;
  --w-e-textarea-slight-border-color: #2a2e39;
  --w-e-textarea-slight-bg-color: #171a20;
  --w-e-textarea-selected-bg-color: #2a2e39;
  --w-e-modal-bg-color: #1d212b;
  --w-e-modal-color: #c7cad1;
  --w-e-modal-button-bg-color: #23262f;
}
html.dark .rich-editor .w-e-toolbar {
  border-bottom-color: #2a2e39;
}
</style>
