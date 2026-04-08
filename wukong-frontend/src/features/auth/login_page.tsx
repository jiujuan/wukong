import { useState } from 'react'
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
    <div className="flex min-h-screen items-center justify-center bg-zinc-100 p-4">
      <Card className="w-full max-w-md p-6">
        <div className="mb-6">
          <h1 className="text-2xl font-semibold text-zinc-900">登录 Wukong</h1>
          <p className="mt-1 text-sm text-zinc-500">登录后才能访问对话、任务、技能和记忆功能</p>
        </div>
        <div className="space-y-3">
          <Input
            placeholder="用户名"
            value={username}
            onChange={(event) => setUsername(event.target.value)}
          />
          <Input
            type="password"
            placeholder="密码"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault()
                void submit()
              }
            }}
          />
          <Button className="w-full" onClick={submit} disabled={submitting}>
            {submitting ? '登录中...' : '登录'}
          </Button>
        </div>
      </Card>
    </div>
  )
}
