import { createContext, useContext, useState, useCallback, ReactNode } from 'react'
import zh from './locales/zh'
import en from './locales/en'
import ru from './locales/ru'

type Lang = 'zh' | 'en' | 'ru'
const locales = { zh, en, ru }

interface I18nCtx {
  lang: Lang
  setLang: (l: Lang) => void
  t: (path: string, vars?: Record<string, string | number>) => string
}

const I18nContext = createContext<I18nCtx>({
  lang: 'zh',
  setLang: () => {},
  t: (p: string) => p,
})

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>('zh')

  const setLang = useCallback((l: Lang) => {
    setLangState(l)
    localStorage.setItem('snishaper-lang', l)
  }, [])

  const t = useCallback((path: string, vars?: Record<string, string | number>) => {
    const keys = path.split('.')
    let val: unknown = locales[lang]
    for (const k of keys) {
      if (typeof val !== 'object' || val === null) return path
      val = (val as Record<string, unknown>)[k]
    }
    if (typeof val !== 'string') return path
    if (vars) {
      return val.replace(/\{(\w+)\}/g, (_, k) => String(vars[k] ?? ''))
    }
    return val
  }, [lang])

  return (
    <I18nContext.Provider value={{ lang, setLang, t }}>
      {children}
    </I18nContext.Provider>
  )
}

export function useI18n() {
  return useContext(I18nContext)
}
