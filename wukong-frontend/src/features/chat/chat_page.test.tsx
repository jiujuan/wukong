import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ChatPage } from './chat_page'
import { useAppStore } from '@/store/use_app_store'

const apiMocks = vi.hoisted(() => ({
  listMessages: vi.fn(),
  sendMessage: vi.fn(),
  createSession: vi.fn(),
}))

const sseMocks = vi.hoisted(() => ({
  createSSE: vi.fn(),
}))

vi.mock('@/lib/api', () => ({
  api: {
    listMessages: apiMocks.listMessages,
    sendMessage: apiMocks.sendMessage,
    createSession: apiMocks.createSession,
  },
  streamUrl: (path: string, params: Record<string, string>) =>
    `${path}?${new URLSearchParams(params).toString()}`,
}))

vi.mock('@/lib/sse', () => ({
  createSSE: sseMocks.createSSE,
}))

vi.mock('sonner', () => ({
  toast: {
    error: vi.fn(),
    warning: vi.fn(),
  },
}))

describe('ChatPage', () => {
  beforeEach(() => {
    apiMocks.listMessages.mockResolvedValue([])
    apiMocks.sendMessage.mockResolvedValue(undefined)
    apiMocks.createSession.mockResolvedValue({ sessionId: 'created-session', title: '首条问题' })
    sseMocks.createSSE.mockReturnValue(vi.fn())
    vi.stubGlobal('crypto', { randomUUID: vi.fn(() => 'test-id') })
    useAppStore.setState({
      sessions: [{ sessionId: 'session-1', title: '会话 1' }],
      currentSessionId: 'session-1',
      messagesBySession: { 'session-1': [] },
      tasks: [],
      currentTaskId: '',
      eventsByTask: {},
      workingMemory: [],
      longMemory: [],
      skills: [],
      memoryOpen: false,
      updateSessionTitle: vi.fn(),
    })
  })

  it('reuses the current session when sending follow-up messages', async () => {
    render(<ChatPage />)

    const textarea = screen.getAllByRole('textbox')[0]
    fireEvent.change(textarea, { target: { value: '我叫什么' } })
    fireEvent.keyDown(textarea, { key: 'Enter', code: 'Enter' })

    await waitFor(() => {
      expect(apiMocks.sendMessage).toHaveBeenCalledWith('session-1', '我叫什么')
    })
    expect(apiMocks.createSession).not.toHaveBeenCalled()
    expect(sseMocks.createSSE).toHaveBeenCalledWith(
      '/api/v1/stream/chat?sessionId=session-1',
      'chat:session-1',
      expect.any(Function),
      expect.any(Function),
    )
  })

  it('creates a session title from the first question', async () => {
    const updateSessionTitle = vi.fn()
    useAppStore.setState({
      sessions: [],
      currentSessionId: '',
      messagesBySession: {},
      tasks: [],
      currentTaskId: '',
      eventsByTask: {},
      workingMemory: [],
      longMemory: [],
      skills: [],
      memoryOpen: false,
      updateSessionTitle,
    })

    render(<ChatPage />)

    const textarea = screen.getAllByRole('textbox')[0]
    fireEvent.change(textarea, { target: { value: '请帮我总结一下这个项目的核心模块架构' } })
    fireEvent.keyDown(textarea, { key: 'Enter', code: 'Enter' })

    await waitFor(() => {
      expect(apiMocks.createSession).toHaveBeenCalledWith('请帮我总结一下这个项目的核心模块架构')
      expect(apiMocks.sendMessage).toHaveBeenCalledWith(
        'created-session',
        '请帮我总结一下这个项目的核心模块架构',
      )
    })
    expect(updateSessionTitle).not.toHaveBeenCalled()
  })
})
