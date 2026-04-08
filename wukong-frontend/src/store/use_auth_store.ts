import { create } from 'zustand'
import { api, getAuthToken, setAuthToken } from '@/lib/api'

type AuthState = {
  token: string
  isAuthenticated: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  hydrate: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  token: '',
  isAuthenticated: false,
  async login(username, password) {
    const token = await api.login(username, password)
    setAuthToken(token)
    set({ token, isAuthenticated: true })
  },
  async logout() {
    try {
      await api.logout()
    } catch (error) {
      void error
    }
    setAuthToken('')
    set({ token: '', isAuthenticated: false })
  },
  hydrate() {
    const token = getAuthToken()
    set({ token, isAuthenticated: Boolean(token) })
  },
}))
