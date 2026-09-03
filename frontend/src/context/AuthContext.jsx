import { createContext, useContext, useEffect, useMemo, useState } from 'react'
import { apiClient } from '../api/client'

const AuthContext = createContext({
  user: null,
  loading: true,
  refresh: async () => null,
})

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    try {
      const currentUser = await apiClient.get('/api/me', { redirectOnUnauthorized: false })
      setUser(currentUser)
      return currentUser
    } catch {
      setUser(null)
      return null
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    refresh()
  }, [])

  const value = useMemo(() => ({ user, loading, refresh }), [user, loading])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  return useContext(AuthContext)
}
