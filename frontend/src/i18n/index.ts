// i18n(v0.12):react-i18next,zh 为默认语言,偏好持久化到 localStorage。
// 覆盖范围:前端页面与组件的用户可见文案;后端接口返回的错误 / 审计文案暂保持中文。
import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import en from './en'
import { zh } from './zh'

const LANG_KEY = 'haproxy-webui-lang'

const saved = localStorage.getItem(LANG_KEY) ?? 'zh'

i18n.use(initReactI18next).init({
  resources: { zh: { translation: zh }, en: { translation: en } },
  lng: saved,
  fallbackLng: 'zh',
  interpolation: { escapeValue: false },
})

// 切换语言并持久化(供顶栏语言切换组件调用)
export function changeLanguage(lang: 'zh' | 'en') {
  void i18n.changeLanguage(lang)
  localStorage.setItem(LANG_KEY, lang)
}

export { LANG_KEY }
