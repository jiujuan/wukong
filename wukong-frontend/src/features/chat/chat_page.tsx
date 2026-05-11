import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Bot, Loader2, Send, UserRound } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import { MarkdownView } from '@/components/common/markdown_view'
import { api, streamUrl } from '@/lib/api'
import { createSSE } from '@/lib/sse'
import { useAppStore } from '@/store/use_app_store'

export function ChatPage() {
  const [input, setInput] = useState('')
  const [isResponding, setIsResponding] = useState(false)
  const typingRef = useRef<number | null>(null)
  const chunkQueueRef = useRef<string[]>([])
  const draftRef = useRef('')

  const currentSessionId = useAppStore((state) => state.currentSessionId)
  const currentSession = useAppStore(
    (state) => state.sessions.find((session) => session.sessionId === state.currentSessionId) ?? null,
  )
  const messagesBySession = useAppStore((state) => state.messagesBySession)
  const appendMessage = useAppStore((state) => state.appendMessage)
  const updateLastAssistantMessage = useAppStore((state) => state.updateLastAssistantMessage)
  const loadMessages = useAppStore((state) => state.loadMessages)
  const createSession = useAppStore((state) => state.createSession)
  const updateSessionTitle = useAppStore((state) => state.updateSessionTitle)

  const messages = useMemo(
    () => messagesBySession[currentSessionId] ?? [],
    [currentSessionId, messagesBySession],
  )

  const stopTyping = useCallback(() => {
    chunkQueueRef.current = []
    draftRef.current = ''
    if (typingRef.current) {
      window.clearInterval(typingRef.current)
      typingRef.current = null
    }
  }, [])

  const buildSessionTitle = useCallback((content: string) => {
    const normalized = content.replace(/\s+/g, ' ').trim()
    const chars = Array.from(normalized)
    if (chars.length <= 24) {
      return normalized || '新会话'
    }
    return `${chars.slice(0, 24).join('')}...`
  }, [])

  useEffect(() => {
    if (!currentSessionId) {
      stopTyping()
      return
    }
    loadMessages(currentSessionId).catch((error: Error) => {
      toast.error(error.message)
    })
  }, [currentSessionId, loadMessages, stopTyping])

  useEffect(() => {
    if (!currentSessionId) {
      stopTyping()
      queueMicrotask(() => setIsResponding(false))
      return
    }
    const close = createSSE(
      streamUrl('/api/v1/stream/chat', { sessionId: currentSessionId }),
      `chat:${currentSessionId}`,
      (event) => {
        if (event.msgType === 'CHUNK') {
          if (event.content) {
            chunkQueueRef.current.push(event.content)
          }
          setIsResponding(true)
          if (!typingRef.current) {
            typingRef.current = window.setInterval(() => {
              const queue = chunkQueueRef.current
              if (queue.length === 0) {
                if (typingRef.current) {
                  window.clearInterval(typingRef.current)
                  typingRef.current = null
                }
                return
              }
              const next = queue[0]
              if (next.length === 0) {
                queue.shift()
                return
              }
              const char = next[0]
              queue[0] = next.slice(1)
              draftRef.current += char
              updateLastAssistantMessage(currentSessionId, draftRef.current)
            }, 20)
          }
        }
        if (event.msgType === 'FINISH') {
          if (chunkQueueRef.current.length > 0) {
            draftRef.current += chunkQueueRef.current.join('')
            updateLastAssistantMessage(currentSessionId, draftRef.current)
          }
          stopTyping()
          setIsResponding(false)
        }
      },
      () => {
        toast.warning('对话流已断开，正在等待重连。')
        setIsResponding(false)
        stopTyping()
      },
    )
    return () => {
      close()
      stopTyping()
      setIsResponding(false)
    }
  }, [currentSessionId, stopTyping, updateLastAssistantMessage])

  const submit = async () => {
    const content = input.trim()
    if (!content || isResponding) {
      return
    }

    let sessionId = currentSessionId
    if (!sessionId) {
      try {
        const session = await createSession(buildSessionTitle(content))
        sessionId = session.sessionId
      } catch (error) {
        toast.error((error as Error).message)
        return
      }
    } else if (!currentSession?.title || currentSession.title === '新会话') {
      updateSessionTitle(sessionId, buildSessionTitle(content))
    }

    setInput('')
    appendMessage(sessionId, {
      id: crypto.randomUUID(),
      role: 'user',
      content,
    })
    appendMessage(sessionId, {
      id: crypto.randomUUID(),
      role: 'assistant',
      content: '',
    })
    stopTyping()
    setIsResponding(true)
    try {
      await api.sendMessage(sessionId, content)
    } catch (error) {
      setIsResponding(false)
      stopTyping()
      toast.error((error as Error).message)
    }
  }

  return (
    <div className="flex h-full flex-col gap-4">
      <Card className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <div className="border-b border-zinc-100 px-5 py-4">
          <div className="flex items-center justify-between gap-3">
            <div>
              <div className="text-sm font-semibold text-zinc-900">晴景助手</div>
              <div className="mt-1 text-xs text-zinc-400">轻量对话模式，实时生成回复</div>
            </div>
            <div className="rounded-full border border-emerald-100 bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-600">
              Online
            </div>
          </div>
        </div>
        <div className="min-h-0 flex-1 overflow-auto bg-gradient-to-b from-white to-zinc-50/80 p-5">
          {!currentSessionId ? (
            <div className="flex h-full items-center justify-center">
              <div className="max-w-sm text-center">
                <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-zinc-100 text-zinc-500">
                  <Bot className="h-6 w-6" />
                </div>
                <div className="text-base font-semibold text-zinc-900">开始一个新会话</div>
                <div className="mt-2 text-sm leading-6 text-zinc-500">
                  输入问题后系统会自动创建会话，并在左侧列表中保留记录。
                </div>
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              {messages.map((message, index) => {
                const isUser = message.role === 'user'
                const isLastAssistant =
                  !isUser && index === messages.length - 1 && message.content.trim() === ''
                const showTypingCursor = !isUser && isResponding && index === messages.length - 1
                return (
                  <div
                    key={message.id}
                    className={`flex gap-3 ${isUser ? 'justify-end' : 'justify-start'}`}
                  >
                    {!isUser ? (
                      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-zinc-100 text-zinc-500">
                        <Bot className="h-5 w-5" />
                      </div>
                    ) : null}
                    <div
                      className={`max-w-[76%] rounded-2xl border px-4 py-3 text-sm leading-6 shadow-sm ${
                        isUser
                          ? 'border-zinc-200 bg-zinc-100 text-zinc-800'
                          : 'border-zinc-200 bg-white text-zinc-800'
                      }`}
                    >
                      {isLastAssistant && isResponding ? (
                        <div className="flex items-center gap-2 text-zinc-400">
                          <Loader2 className="h-4 w-4 animate-spin" />
                          <span>正在生成回复</span>
                        </div>
                      ) : (
                        <div className="min-w-0">
                          <MarkdownView content={message.content || ''} />
                          {showTypingCursor ? (
                            <span className="ml-1 inline-block animate-pulse text-indigo-400">
                              |
                            </span>
                          ) : null}
                        </div>
                      )}
                    </div>
                    {isUser ? (
                      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-zinc-100 text-zinc-500">
                        <UserRound className="h-5 w-5" />
                      </div>
                    ) : null}
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </Card>
      <Card className="p-3">
        <div className="flex items-end gap-3">
          <Textarea
            value={input}
            className="min-h-[72px] flex-1 resize-none border-0 bg-zinc-50 focus-visible:ring-indigo-50"
            onChange={(event) => setInput(event.target.value)}
            placeholder="输入你的问题，Enter 发送，Shift + Enter 换行"
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !event.shiftKey) {
                event.preventDefault()
                void submit()
              }
            }}
          />
          <Button className="h-10 shrink-0 gap-2" onClick={submit} disabled={isResponding}>
            <Send className="h-4 w-4" />
            发送
          </Button>
        </div>
      </Card>
    </div>
  )
}
