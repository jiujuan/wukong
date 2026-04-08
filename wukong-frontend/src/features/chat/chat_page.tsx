import { useEffect, useMemo, useRef, useState } from 'react'
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
  const typingRef = useRef<number | null>(null)
  const chunkQueueRef = useRef<string[]>([])
  const draftRef = useRef('')

  const currentSessionId = useAppStore((state) => state.currentSessionId)
  const messagesBySession = useAppStore((state) => state.messagesBySession)
  const appendMessage = useAppStore((state) => state.appendMessage)
  const updateLastAssistantMessage = useAppStore((state) => state.updateLastAssistantMessage)
  const loadMessages = useAppStore((state) => state.loadMessages)
  const createSession = useAppStore((state) => state.createSession)

  const messages = useMemo(
    () => messagesBySession[currentSessionId] ?? [],
    [currentSessionId, messagesBySession],
  )

  useEffect(() => {
    if (!currentSessionId) {
      return
    }
    loadMessages(currentSessionId).catch((error: Error) => {
      toast.error(error.message)
    })
  }, [currentSessionId, loadMessages])

  useEffect(() => {
    if (!currentSessionId) {
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
          chunkQueueRef.current = []
          if (typingRef.current) {
            window.clearInterval(typingRef.current)
            typingRef.current = null
          }
          draftRef.current = ''
        }
      },
      () => {
        toast.warning('对话流已断开，正在等待重连')
      },
    )
    return () => {
      close()
      if (typingRef.current) {
        window.clearInterval(typingRef.current)
        typingRef.current = null
      }
    }
  }, [currentSessionId, updateLastAssistantMessage])

  const submit = async () => {
    if (!input.trim()) {
      return
    }
    let sessionId = currentSessionId
    if (!sessionId) {
      try {
        const session = await createSession()
        sessionId = session.sessionId
      } catch (error) {
        toast.error((error as Error).message)
        return
      }
    }
    const content = input.trim()
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
    draftRef.current = ''
    try {
      await api.sendMessage(sessionId, content)
    } catch (error) {
      toast.error((error as Error).message)
    }
  }

  return (
    <div className="flex h-full flex-col gap-4">
      <Card className="flex-1 overflow-auto p-4">
        {!currentSessionId ? (
          <div className="text-zinc-500">请输入内容并发送，系统会自动创建会话</div>
        ) : (
          <div className="space-y-4">
            {messages.map((message) => (
              <div
                key={message.id}
                className={`max-w-[85%] rounded-lg border p-3 ${
                  message.role === 'user'
                    ? 'ml-auto border-zinc-300 bg-zinc-100 text-zinc-900'
                    : 'border-zinc-300 bg-white text-zinc-900'
                }`}
              >
                <div className="mb-2 text-xs uppercase tracking-wide opacity-70">
                  {message.role === 'user' ? '你' : 'Wukong'}
                </div>
                <MarkdownView content={message.content || '...'} />
              </div>
            ))}
          </div>
        )}
      </Card>
      <Card className="p-3">
        <div className="flex flex-col gap-3">
          <Textarea
            value={input}
            onChange={(event) => setInput(event.target.value)}
            placeholder="输入你的问题，回车发送，Shift + 回车换行"
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !event.shiftKey) {
                event.preventDefault()
                void submit()
              }
            }}
          />
          <div className="flex justify-end">
            <Button onClick={submit}>发送</Button>
          </div>
        </div>
      </Card>
    </div>
  )
}
