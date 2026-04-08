import { useEffect } from 'react'
import { Navigate, Outlet, Route, Routes, useLocation } from 'react-router-dom'
import { AppShell } from '@/app/layout/app_shell'
import { LoginPage } from '@/features/auth/login_page'
import { ChatPage } from '@/features/chat/chat_page'
import { MemoryDrawer } from '@/features/memory/memory_drawer'
import { SkillsPage } from '@/features/skills/skills_page'
import { TasksPage } from '@/features/tasks/tasks_page'
import { useBootstrap } from '@/hooks/use_bootstrap'
import { useAuthStore } from '@/store/use_auth_store'

export function AppRouter() {
  const hydrate = useAuthStore((state) => state.hydrate)
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated)
  useEffect(() => {
    hydrate()
  }, [hydrate])
  useBootstrap(isAuthenticated)
  return (
    <>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route element={<ProtectedLayout />}>
          <Route path="/" element={<Navigate to="/chat" replace />} />
          <Route path="/chat" element={<ChatPage />} />
          <Route path="/tasks" element={<TasksPage />} />
          <Route path="/tasks/:taskId" element={<TasksPage />} />
          <Route path="/skills" element={<SkillsPage />} />
        </Route>
      </Routes>
      <MemoryDrawer />
    </>
  )
}

function ProtectedLayout() {
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated)
  const location = useLocation()
  if (!isAuthenticated) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }
  return (
    <AppShell>
      <Outlet />
    </AppShell>
  )
}
