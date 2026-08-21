// @wangeditor/editor-for-vue 的 package.json exports 未暴露类型声明（已知问题），
// 这里声明 Editor / Toolbar 组件类型
declare module '@wangeditor/editor-for-vue' {
  import type { DefineComponent } from 'vue'
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  export const Editor: DefineComponent<any, any, any>
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  export const Toolbar: DefineComponent<any, any, any>
}
