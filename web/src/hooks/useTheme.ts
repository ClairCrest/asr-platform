import { useEffect, useState } from 'react'

type Theme = 'light' | 'dark'

const STORAGE_KEY = 'asr-platform:theme'

function getInitialTheme(): Theme {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored === 'light' || stored === 'dark') return stored
  } catch {
    // localStorage unavailable (private browsing, disabled storage) — fall through to system preference.
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(getInitialTheme)

  useEffect(() => {
    document.documentElement.classList.toggle('dark', theme === 'dark')
    try {
      localStorage.setItem(STORAGE_KEY, theme)
    } catch {
      // ignore — theme still applies for this session even if it can't persist.
    }
  }, [theme])

  const toggleTheme = () => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))

  return { theme, toggleTheme }
}
