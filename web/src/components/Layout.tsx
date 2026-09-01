import { Link, NavLink, Outlet } from 'react-router'
import { useAuth } from '../auth/AuthContext'
import { useJobStream } from '../hooks/useJobStream'
import { useTheme } from '../hooks/useTheme'

const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  `rounded-md px-3 py-1.5 text-sm font-medium ${
    isActive
      ? 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900'
      : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800'
  }`

export function Layout() {
  const { user, logout } = useAuth()
  const { theme, toggleTheme } = useTheme()
  useJobStream()

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-950">
      <header className="border-b border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-900">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
          <Link to="/jobs" className="text-lg font-semibold text-gray-900 dark:text-gray-100">
            ASR Platform
          </Link>
          <nav className="flex items-center gap-2">
            <NavLink to="/jobs" className={navLinkClass}>
              Jobs
            </NavLink>
            <NavLink to="/api-keys" className={navLinkClass}>
              API Keys
            </NavLink>
            <span className="mx-2 h-4 w-px bg-gray-200 dark:bg-gray-700" />
            <button
              type="button"
              onClick={toggleTheme}
              aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
              className="rounded-md p-1.5 text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800"
            >
              {theme === 'dark' ? (
                <svg viewBox="0 0 20 20" fill="currentColor" className="h-4 w-4">
                  <path d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4.22 1.78a1 1 0 011.41 1.41l-.7.71a1 1 0 11-1.42-1.42l.7-.7zM17 9a1 1 0 110 2h-1a1 1 0 110-2h1zm-2.34 5.66a1 1 0 011.42 1.42l-.71.7a1 1 0 01-1.41-1.41l.7-.71zM10 15a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zm-5.66-.34a1 1 0 011.42 0 1 1 0 010 1.41l-.71.71a1 1 0 01-1.41-1.42l.7-.7zM4 9a1 1 0 110 2H3a1 1 0 110-2h1zm.64-4.9a1 1 0 011.42 1.41l-.7.71A1 1 0 013.93 4.8l.7-.7zM10 6a4 4 0 100 8 4 4 0 000-8z" />
                </svg>
              ) : (
                <svg viewBox="0 0 20 20" fill="currentColor" className="h-4 w-4">
                  <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z" />
                </svg>
              )}
            </button>
            <span className="text-sm text-gray-500 dark:text-gray-400">{user?.email}</span>
            <button
              type="button"
              onClick={logout}
              className="rounded-md px-3 py-1.5 text-sm font-medium text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800"
            >
              Log out
            </button>
          </nav>
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-6 py-8">
        <Outlet />
      </main>
    </div>
  )
}
