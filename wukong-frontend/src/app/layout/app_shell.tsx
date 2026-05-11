import {
  Brain,
  ChevronRight,
  Database,
  ListTodo,
  LogOut,
  MessageSquarePlus,
  Moon,
  Sparkles,
  Trash2,
} from 'lucide-react'
import type { ReactNode } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/store/use_auth_store'
import { useAppStore } from '@/store/use_app_store'

type AppShellProps = {
  children: ReactNode
}

const navGroups = [
  {
    title: '概览',
    items: [
      { to: '/chat', label: '晴辰助手', icon: Sparkles },
      { to: '/tasks', label: '任务中心', icon: ListTodo },
    ],
  },
  {
    title: '配置',
    items: [{ to: '/skills', label: '服务管理', icon: Brain }],
  },
]

const pageTitles: Record<string, string> = {
  '/chat': '晴辰助手',
  '/tasks': '任务中心',
  '/skills': '服务管理',
}

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

  const pageTitle =
    pageTitles[location.pathname] ??
    (location.pathname.startsWith('/tasks') ? '任务详情' : 'Wukong')

  const handleCreateSession = () => {
    createSession()
      .then((session) => {
        navigate('/chat')
        setCurrentSession(session.sessionId)
      })
      .catch((error: Error) => toast.error(error.message))
  }

  return (
    <div className="grid h-screen min-w-[1040px] grid-cols-[280px_1fr] bg-[#f7f8fb] text-zinc-900">
      <aside className="flex min-h-0 flex-col border-r border-zinc-200 bg-white">
        <div className="flex h-16 items-center justify-between gap-3 px-5">
          <div className="min-w-0">
            <div className="truncate text-lg font-semibold tracking-tight text-zinc-900">Wukong UI</div>
          </div>
          <Button
            size="sm"
            variant="secondary"
            className="h-9 gap-2 rounded-xl border-zinc-200 bg-zinc-100 px-4 text-zinc-700 shadow-none hover:bg-zinc-200"
            onClick={handleCreateSession}
          >
            <MessageSquarePlus className="h-4 w-4" />
            新会话
          </Button>
        </div>

        <div className="min-h-0 flex-1 overflow-auto px-4 pb-4">
          <div className="space-y-6">
            {navGroups.map((group) => (
              <nav key={group.title}>
                <div className="mb-2 px-1 text-xs font-medium text-zinc-400">{group.title}</div>
                <div className="space-y-1">
                  {group.items.map((item) => (
                    <NavLink
                      key={item.to}
                      to={item.to}
                      className={({ isActive }) =>
                        `flex h-11 items-center gap-3 rounded-xl px-3 text-sm font-medium transition-colors ${
                          isActive || location.pathname.startsWith(`${item.to}/`)
                            ? 'bg-[#efeefe] text-[#635bff]'
                            : 'text-zinc-500 hover:bg-zinc-50 hover:text-zinc-800'
                        }`
                      }
                    >
                      <item.icon className="h-5 w-5 shrink-0" />
                      <span className="truncate">{item.label}</span>
                    </NavLink>
                  ))}
                </div>
              </nav>
            ))}

            <section>
              <div className="mb-2 flex items-center justify-between px-1">
                <div className="text-xs font-medium text-zinc-400">数据</div>
                <button
                  className="rounded-md p-1 text-zinc-400 hover:bg-zinc-50 hover:text-indigo-600"
                  onClick={() => toggleMemory(true)}
                  title="记忆文件"
                >
                  <Database className="h-4 w-4" />
                </button>
              </div>
              <button
                className="flex h-11 w-full items-center gap-3 rounded-xl px-3 text-left text-sm font-medium text-zinc-500 transition-colors hover:bg-zinc-50 hover:text-zinc-800"
                onClick={() => toggleMemory(true)}
              >
                <Database className="h-5 w-5 shrink-0" />
                <span className="truncate">记忆文件</span>
              </button>
            </section>

            <section>
              <div className="mb-2 flex items-center justify-between px-1">
                <div className="text-xs font-medium text-zinc-400">会话列表</div>
                <button
                  className="rounded-md p-1 text-zinc-400 hover:bg-zinc-50 hover:text-indigo-600"
                  onClick={handleCreateSession}
                  title="新会话"
                >
                  <MessageSquarePlus className="h-4 w-4" />
                </button>
              </div>
              <div className="space-y-2">
                {sessions.length === 0 ? (
                  <div className="rounded-xl border border-dashed border-zinc-200 px-3 py-3 text-xs text-zinc-400">
                    暂无会话
                  </div>
                ) : (
                  sessions.map((session) => (
                    <div
                      key={session.sessionId}
                      className={`group flex items-center gap-2 rounded-xl px-3 py-2 transition-colors ${
                        session.sessionId === currentSessionId
                          ? 'bg-zinc-100 text-zinc-900'
                          : 'text-zinc-500 hover:bg-zinc-50 hover:text-zinc-800'
                      }`}
                    >
                      <button
                        className="min-w-0 flex-1 truncate text-left text-sm"
                        onClick={() => {
                          setCurrentSession(session.sessionId)
                          navigate('/chat')
                        }}
                      >
                        {session.title}
                      </button>
                      <button
                        className="shrink-0 rounded-md p-1 text-zinc-300 opacity-0 transition-opacity hover:bg-rose-50 hover:text-rose-500 group-hover:opacity-100"
                        onClick={() => {
                          deleteSession(session.sessionId).catch((error: Error) =>
                            toast.error(error.message),
                          )
                        }}
                        title="删除会话"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  ))
                )}
              </div>
            </section>
          </div>
        </div>

        <div className="border-t border-zinc-100 p-4">
          <button className="mb-2 flex h-11 w-full items-center gap-3 rounded-xl px-3 text-sm font-medium text-zinc-500 hover:bg-zinc-50">
            <Moon className="h-5 w-5" />
            <span>夜间模式</span>
          </button>
          <Button
            variant="ghost"
            className="h-10 w-full justify-start gap-3 px-3 text-zinc-500"
            onClick={() => {
              logout()
                .then(() => navigate('/login'))
                .catch((error: Error) => toast.error(error.message))
            }}
          >
            <LogOut className="h-5 w-5" />
            退出登录
          </Button>
        </div>
      </aside>

      <main className="flex min-h-0 flex-col">
        <header className="flex h-16 shrink-0 items-center justify-between border-b border-zinc-200 bg-white/80 px-6 backdrop-blur">
          <div>
            <div className="flex items-center gap-2 text-xs text-zinc-400">
              <span>Wukong</span>
              <ChevronRight className="h-3.5 w-3.5" />
              <span>{pageTitle}</span>
            </div>
            <div className="mt-1 text-lg font-semibold text-zinc-900">{pageTitle}</div>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="secondary" onClick={() => toggleMemory(true)}>
              记忆文件
            </Button>
            <Button onClick={handleCreateSession}>
              <MessageSquarePlus className="mr-2 h-4 w-4" />
              新会话
            </Button>
          </div>
        </header>
        <div className="min-h-0 flex-1 overflow-auto p-6">{children}</div>
      </main>
    </div>
  )
}
