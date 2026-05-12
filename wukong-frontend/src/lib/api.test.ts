import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api, setAuthToken, streamUrl } from '@/lib/api'

describe('api memory requests', () => {
  beforeEach(() => {
    window.localStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('requests working memory with task_id query parameter', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          list: [{ memory_id: 'wm-1', summary: 'working summary', created_at: '2026-05-11' }],
        },
      }),
    } as Response)

    const result = await api.listWorkingMemory('task-123')

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost:8080/api/v1/memory/working/list?task_id=task-123',
      expect.objectContaining({
        headers: expect.objectContaining({
          'Content-Type': 'application/json',
        }),
      }),
    )
    expect(result).toEqual([
      {
        id: 'wm-1',
        content: 'working summary',
        createdAt: '2026-05-11',
      },
    ])
  })

  it('requests long memory with skill_name and authorization header', async () => {
    const fetchMock = vi.mocked(fetch)
    setAuthToken('token-1')
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          list: [{ memory_id: 'lm-1', topic: 'topic-a', content: 'long memory body' }],
        },
      }),
    } as Response)

    const result = await api.listLongMemory({ skillName: 'planner', keyword: 'latest' })

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost:8080/api/v1/memory/long/list?skill_name=planner&keyword=latest',
      expect.objectContaining({
        headers: expect.objectContaining({
          'Content-Type': 'application/json',
          Authorization: 'Bearer token-1',
        }),
      }),
    )
    expect(result).toEqual([
      {
        id: 'lm-1',
        topic: 'topic-a',
        content: 'long memory body',
        createdAt: undefined,
      },
    ])
  })

  it('builds stream urls from params', () => {
    expect(streamUrl('/api/v1/stream/task', { taskId: 'task-1', mode: 'live' })).toBe(
      'http://localhost:8080/api/v1/stream/task?taskId=task-1&mode=live',
    )
  })

  it('posts chat messages without changing the API contract', async () => {
    const fetchMock = vi.mocked(fetch)
    setAuthToken('token-chat')
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          msg_id: 'msg-1',
          session_id: 'session-1',
          content: 'reply',
          role: 'assistant',
        },
      }),
    } as Response)

    await api.sendMessage('session-1', 'hello context')

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost:8080/api/v1/chat/message/send',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ sessionId: 'session-1', content: 'hello context' }),
        headers: expect.objectContaining({
          'Content-Type': 'application/json',
          Authorization: 'Bearer token-chat',
        }),
      }),
    )
  })

  it('posts task creation without changing the API contract', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          task_id: 'task-1',
          skill_name: 'web_search',
          status: 'PENDING',
        },
      }),
    } as Response)

    await api.createTask({
      skillName: 'web_search',
      sessionId: 'session-1',
      params: { query: 'golang' },
      priority: 7,
    })

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost:8080/api/v1/task/create',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          skill_name: 'web_search',
          session_id: 'session-1',
          params: { query: 'golang' },
          priority: 7,
        }),
      }),
    )
  })

  it('requests paged task list with page and size query parameters', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          list: [],
          total: 0,
          page: 1,
          size: 10,
          pages: 0,
        },
      }),
    } as Response)

    await api.listTasksPage({ page: 1, size: 10 })

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost:8080/api/v1/task/list?page=1&size=10',
      expect.objectContaining({
        headers: expect.objectContaining({
          'Content-Type': 'application/json',
        }),
      }),
    )
  })

  it('parses task detail response without changing the API contract', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          task: {
            task_id: 'task-ctx-1',
            skill_name: 'web_search',
            status: 'RUNNING',
          },
          subtasks: [
            {
              sub_task_id: 'sub-1',
              action: 'web_search',
              status: 'PENDING',
            },
          ],
        },
      }),
    } as Response)

    const detail = await api.taskDetail('task-ctx-1')

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost:8080/api/v1/task/detail?task_id=task-ctx-1',
      expect.anything(),
    )
    expect(detail.task.taskId).toBe('task-ctx-1')
    expect(detail.subTasks[0]?.subTaskId).toBe('sub-1')
  })
})
