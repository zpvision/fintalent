import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { apiClient } from '../api/client'

const AUTH_CACHE_KEY = 'fintalent.auth.user.v1'

function readCachedUser() {
  try {
    return JSON.parse(window.sessionStorage.getItem(AUTH_CACHE_KEY)) || null
  } catch {
    return null
  }
}

function cacheUser(user) {
  try {
    if (user) window.sessionStorage.setItem(AUTH_CACHE_KEY, JSON.stringify(user))
    else window.sessionStorage.removeItem(AUTH_CACHE_KEY)
  } catch {
    // Storage can be unavailable in privacy mode; the cookie remains authoritative.
  }
}

const AuthContext = createContext({
  user: null,
  loading: true,
  refresh: async () => null,
})

export function AuthProvider({ children }) {
  const [user, setUser] = useState(readCachedUser)
  const [loading, setLoading] = useState(true)
  const [coursesAccess, setCoursesAccess] = useState(false)
  const [coursesAccessLoading, setCoursesAccessLoading] = useState(true)
  const refreshCoursesAccess = useCallback(async () => {
    try {
      const access = await apiClient.get('/api/courses/access', { redirectOnUnauthorized: false })
      setCoursesAccess(access.allowed === true)
    } catch { setCoursesAccess(false) }
    finally { setCoursesAccessLoading(false) }
  }, [])

  const refresh = useCallback(async () => {
    try {
      const currentUser = await apiClient.get('/api/me', { redirectOnUnauthorized: false })
      cacheUser(currentUser)
      setUser(currentUser)
      return currentUser
    } catch {
      cacheUser(null)
      setUser(null)
      return null
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])
  useEffect(() => {
    refreshCoursesAccess()
    window.addEventListener('focus', refreshCoursesAccess)
    return () => window.removeEventListener('focus', refreshCoursesAccess)
  }, [refreshCoursesAccess, user?.id])

  const value = useMemo(() => ({ user, loading, refresh, coursesAccess, coursesAccessLoading }), [user, loading, refresh, coursesAccess, coursesAccessLoading])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  return useContext(AuthContext)
}
