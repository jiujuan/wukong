import { useState } from 'react'
import { Bot, Sparkles } from 'lucide-react'
import { useLocation, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { useAuthStore } from '@/store/use_auth_store'

export function LoginPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const login = useAuthStore((state) => state.login)
  const navigate = useNavigate()
  const location = useLocation()
  const redirectTo = (location.state as { from?: string } | undefined)?.from ?? '/chat'

  const submit = async () => {
    if (!username.trim() || !password.trim()) {
      toast.warning('请输入用户名和密码')
      return
    }
    try {
      setSubmitting(true)
      await login(username.trim(), password)
      navigate(redirectTo, { replace: true })
      toast.success('登录成功')
    } catch (error) {
      toast.error((error as Error).message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-[#f7f8fb] p-6">
      <div className="grid w-full max-w-5xl overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-[0_24px_80px_rgba(31,36,48,0.10)] md:grid-cols-[1fr_420px]">
        <div className="hidden min-h-[560px] flex-col justify-between bg-indigo-600 p-10 text-white md:flex">
          <div>
            <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-white/15">
              <Bot className="h-6 w-6" />
            </div>
            <h1 className="mt-8 max-w-md text-4xl font-semibold leading-tight">
              Wukong AI Agent 工作台
            </h1>
            <p className="mt-4 max-w-sm text-sm leading-6 text-indigo-100">
              管理对话、任务执行、技能服务和记忆数据的统一控制台。
            </p>
          </div>
          <div className="rounded-2xl border border-white/15 bg-white/10 p-4 text-sm text-indigo-50 backdrop-blur">
            <div className="mb-2 flex items-center gap-2 font-medium">
              <Sparkles className="h-4 w-4" />
              悟空助手
            </div>
            <p className="leading-6 text-indigo-100">
              登录后即可进入浅色控制台，查看实时任务流和 Agent 执行过程。
            </p>
          </div>
        </div>
        <div className="flex min-h-[560px] items-center p-6 sm:p-10">
          <Card className="w-full border-0 p-0 shadow-none">
            <div className="mb-8">
              <div className="mb-4 flex h-11 w-11 items-center justify-center rounded-2xl bg-indigo-50 text-indigo-600 md:hidden">
                <Bot className="h-6 w-6" />
              </div>
              <h2 className="text-2xl font-semibold text-zinc-900">登录 Wukong</h2>
              <p className="mt-2 text-sm text-zinc-500">进入 AI Agent 控制台</p>
            </div>
            <div className="space-y-4">
              <div className="space-y-2">
                <label className="text-sm font-medium text-zinc-700">用户名</label>
                <Input
                  placeholder="admin"
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium text-zinc-700">密码</label>
                <Input
                  type="password"
                  placeholder="请输入密码"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') {
                      event.preventDefault()
                      void submit()
                    }
                  }}
                />
              </div>
              <Button className="mt-2 w-full" onClick={submit} disabled={submitting}>
                {submitting ? '登录中...' : '登录'}
              </Button>
            </div>
          </Card>
        </div>
      </div>
    </div>
  )
}
