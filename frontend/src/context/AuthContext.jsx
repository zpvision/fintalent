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
  const [learningAccess, setLearningAccess] = useState(false)
 const [teachingAccess,setTeachingAccess]=useState(false)
  const refreshLearningAccess = useCallback(async (signal) => {
    try {
      const result = await apiClient.get('/api/academy/learning/access', { redirectOnUnauthorized: false, signal })
      if (!signal.aborted) { setLearningAccess(result.allowed === true); setTeachingAccess(result.teaching === true) }
    } catch { if (!signal.aborted) { setLearningAccess(false); setTeachingAccess(false) } }
  }, [])
  const refreshCoursesAccess = useCallback(async (signal) => {
    try {
      const access = await apiClient.get('/api/courses/access', { redirectOnUnauthorized: false, signal })
      if (!signal.aborted) setCoursesAccess(access.allowed === true)
    } catch { if (!signal.aborted) setCoursesAccess(false) }
    finally { if (!signal.aborted) setCoursesAccessLoading(false) }
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
    let controller
    const refreshAccess = () => {
      controller?.abort()
      controller = new AbortController()
      refreshCoursesAccess(controller.signal)
      refreshLearningAccess(controller.signal)
    }
    refreshAccess()
    window.addEventListener('focus', refreshAccess)
    return () => { controller.abort(); window.removeEventListener('focus', refreshAccess) }
  }, [refreshCoursesAccess, refreshLearningAccess, user?.id])

  const value = useMemo(() => ({ user, loading, refresh, coursesAccess, coursesAccessLoading, learningAccess,teachingAccess }), [user, loading, refresh, coursesAccess, coursesAccessLoading, learningAccess,teachingAccess])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  return useContext(AuthContext)
}
