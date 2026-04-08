import { Bot, Brain, ListTodo, MessageSquarePlus, Trash2 } from 'lucide-react'
import type { ReactNode } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { useAuthStore } from '@/store/use_auth_store'
import { useAppStore } from '@/store/use_app_store'

type AppShellProps = {
  children: ReactNode
}

const navItems = [
  { to: '/chat', label: '对话', icon: Bot },
  { to: '/tasks', label: '任务', icon: ListTodo },
  { to: '/skills', label: '技能', icon: Brain },
]

export function AppShell({ children }: AppShellProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const sessions = useAppStore((state) => state.sessions)
  const currentSessionId = useAppStore((state) => state.currentSessionId)
  const setCurrentSession = useAppStore((state) => state.setCurrentSession)
  const createSession = useAppStore((state) => state.createSession)
  const deleteSession = useAppStore((state) => state.deleteSession)
  const toggleMemory = useAppStore((state) => state.toggleMemory)
  const logout = useAuthStore((state) => state.logout)

  return (
    <div className="grid h-screen grid-cols-[300px_1fr] bg-white text-zinc-900">
      <aside className="border-r border-zinc-300 bg-zinc-100 p-4">
        <div className="mb-4 flex items-center justify-between">
          <div className="text-lg font-semibold">Wukong UI</div>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => {
              createSession()
                .then((session) => {
                  navigate('/chat')
                  setCurrentSession(session.sessionId)
                })
                .catch((error: Error) => toast.error(error.message))
            }}
          >
            <MessageSquarePlus className="mr-1 h-4 w-4" />
            新会话
          </Button>
        </div>
        <Card className="mb-4 p-2">
          <div className="mb-2 text-xs uppercase tracking-wide text-zinc-600">导航</div>
          <div className="space-y-1">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) =>
                  `flex items-center gap-2 rounded-md px-3 py-2 text-sm ${
                    isActive ? 'bg-zinc-300 text-zinc-950' : 'text-zinc-700 hover:bg-zinc-200'
                  }`
                }
              >
                <item.icon className="h-4 w-4" />
                {item.label}
              </NavLink>
            ))}
          </div>
        </Card>
        <Card className="p-2">
          <div className="mb-2 text-xs uppercase tracking-wide text-zinc-600">会话列表</div>
          <div className="space-y-1">
            {sessions.map((session) => (
              <div
                key={session.sessionId}
                className={`group flex items-center justify-between rounded-md border p-2 ${
                  session.sessionId === currentSessionId
                    ? 'border-zinc-400 bg-zinc-200'
                    : 'border-zinc-200 bg-white'
                }`}
              >
                <button
                  className="truncate text-left text-sm"
                  onClick={() => {
                    setCurrentSession(session.sessionId)
                    navigate('/chat')
                  }}
                >
                  {session.title}
                </button>
                <button
                  className="opacity-0 transition-opacity group-hover:opacity-100"
                  onClick={() => {
                    deleteSession(session.sessionId).catch((error: Error) =>
                      toast.error(error.message),
                    )
                  }}
                >
                  <Trash2 className="h-4 w-4 text-zinc-500 hover:text-red-500" />
                </button>
              </div>
            ))}
          </div>
        </Card>
      </aside>
      <main className="flex h-screen flex-col bg-white">
        <header className="flex h-14 items-center justify-between border-b border-zinc-300 px-4">
          <div className="text-sm text-zinc-600">{location.pathname}</div>
          <div className="flex items-center gap-2">
            <Button variant="ghost" onClick={() => toggleMemory(true)}>
              记忆抽屉
            </Button>
            <Button
              variant="secondary"
              onClick={() => {
                logout()
                  .then(() => navigate('/login'))
                  .catch((error: Error) => toast.error(error.message))
              }}
            >
              退出登录
            </Button>
          </div>
        </header>
        <div className="min-h-0 flex-1 p-4">{children}</div>
      </main>
    </div>
  )
}
