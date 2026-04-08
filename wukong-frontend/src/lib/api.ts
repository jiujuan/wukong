import type {
  ChatMessage,
  ChatSession,
  LongMemory,
  SkillItem,
  TaskDetail,
  TaskItem,
  WorkingMemory,
} from '@/types/domain'

const API_BASE = import.meta.env.VITE_API_BASE ?? 'http://localhost:8080'
const AUTH_TOKEN_KEY = 'wukong:token'

type ApiEnvelope<T> = {
  code?: number
  msg?: string
  message?: string
  data?: T
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getAuthToken()
  const response = await fetch(`${API_BASE}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init?.headers,
    },
    ...init,
  })
  const data = (await response.json().catch(() => ({}))) as ApiEnvelope<T> | T
  if (!response.ok) {
    const errorMessage =
      (data as ApiEnvelope<T>).msg || (data as ApiEnvelope<T>).message || '请求失败'
    throw new Error(errorMessage)
  }
  if ((data as ApiEnvelope<T>).data !== undefined) {
    return (data as ApiEnvelope<T>).data as T
  }
  return data as T
}

function pick<T>(row: Record<string, unknown>, keys: string[], fallback: T): T {
  for (const key of keys) {
    const value = row[key]
    if (value !== undefined && value !== null) {
      return value as T
    }
  }
  return fallback
}

export const api = {
  async login(username: string, password: string): Promise<string> {
    const raw = await request<Record<string, unknown>>('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    })
    const token = pick(raw, ['access_token', 'accessToken', 'token'], '')
    if (!token) {
      throw new Error('登录成功但未返回 token')
    }
    return token
  },
  async logout(): Promise<void> {
    await request('/api/v1/auth/logout', {
      method: 'POST',
    })
  },
  async listSessions(): Promise<ChatSession[]> {
    const raw = await request<unknown>('/api/v1/chat/session/list')
    const list = Array.isArray(raw)
      ? raw
      : Array.isArray((raw as { list?: unknown[] })?.list)
        ? ((raw as { list?: unknown[] }).list ?? [])
        : []
    return list.map((item, idx) => {
      const row = item as Record<string, unknown>
      return {
        sessionId: pick(row, ['sessionId', 'session_id', 'id'], `session-${idx}`),
        title: pick(row, ['title', 'name'], '未命名会话'),
        createdAt: pick<string | undefined>(row, ['createdAt', 'created_at'], undefined),
      }
    })
  },
  async createSession(title: string): Promise<ChatSession> {
    const raw = await request<Record<string, unknown>>('/api/v1/chat/session/create', {
      method: 'POST',
      body: JSON.stringify({ title }),
    })
    return {
      sessionId: pick(raw, ['sessionId', 'session_id', 'id'], crypto.randomUUID()),
      title: pick(raw, ['title', 'name'], title || '新会话'),
      createdAt: pick<string | undefined>(raw, ['createdAt', 'created_at'], undefined),
    }
  },
  async deleteSession(sessionId: string): Promise<void> {
    await request('/api/v1/chat/session/delete', {
      method: 'POST',
      body: JSON.stringify({ sessionId }),
    })
  },
  async listMessages(sessionId: string): Promise<ChatMessage[]> {
    const raw = await request<unknown>(`/api/v1/chat/message/list?sessionId=${sessionId}`)
    const list = Array.isArray(raw)
      ? raw
      : Array.isArray((raw as { list?: unknown[] })?.list)
        ? ((raw as { list?: unknown[] }).list ?? [])
        : []
    return list.map((item, idx) => {
      const row = item as Record<string, unknown>
      return {
        id: pick(row, ['id', 'messageId', 'message_id'], `message-${idx}`),
        role: pick(row, ['role'], 'assistant'),
        content: pick(row, ['content', 'message'], ''),
        createdAt: pick<string | undefined>(row, ['createdAt', 'created_at'], undefined),
      } as ChatMessage
    })
  },
  async sendMessage(sessionId: string, content: string) {
    await request('/api/v1/chat/message/send', {
      method: 'POST',
      body: JSON.stringify({ sessionId, content }),
    })
  },
  async listTasks(): Promise<TaskItem[]> {
    const raw = await request<unknown>('/api/v1/task/list')
    const list = Array.isArray(raw)
      ? raw
      : Array.isArray((raw as { list?: unknown[] })?.list)
        ? ((raw as { list?: unknown[] }).list ?? [])
        : []
    return list.map((item, idx) => {
      const row = item as Record<string, unknown>
      const params = pick<Record<string, unknown> | undefined>(row, ['params'], undefined)
      const title = resolveTaskTitle(row, params)
      return {
        taskId: pick(row, ['taskId', 'task_id', 'id'], `task-${idx}`),
        sessionId: pick<string | undefined>(row, ['sessionId', 'session_id'], undefined),
        title,
        status: pick(row, ['status'], 'PENDING'),
        params,
        skillName: pick<string | undefined>(row, ['skillName', 'skill_name'], undefined),
        priority: pick<number | undefined>(row, ['priority'], undefined),
        result: pick<Record<string, unknown> | undefined>(row, ['result'], undefined),
        error: pick<string | undefined>(row, ['error'], undefined),
        createdAt: pick<string | undefined>(row, ['createdAt', 'created_at'], undefined),
        updatedAt: pick<string | undefined>(row, ['updatedAt', 'updated_at'], undefined),
      } as TaskItem
    })
  },
  async createTask(input: {
    skillName: string
    params?: Record<string, unknown>
    priority?: number
    sessionId?: string
  }): Promise<TaskItem> {
    const raw = await request<Record<string, unknown>>('/api/v1/task/create', {
      method: 'POST',
      body: JSON.stringify({
        skill_name: input.skillName,
        session_id: input.sessionId,
        params: input.params ?? {},
        priority: input.priority ?? 5,
      }),
    })
    const skillName = pick(raw, ['skillName', 'skill_name'], input.skillName)
    const params = input.params ?? {}
    return {
      taskId: pick(raw, ['taskId', 'task_id', 'id'], crypto.randomUUID()),
      title: resolveTaskTitle(raw, params),
      params,
      skillName,
      status: pick(raw, ['status'], 'PENDING'),
      priority: pick<number | undefined>(raw, ['priority'], input.priority ?? 5),
      createdAt: pick<string | undefined>(raw, ['createdAt', 'created_at'], undefined),
    }
  },
  async taskDetail(taskId: string): Promise<TaskDetail> {
    const raw = await request<Record<string, unknown>>(`/api/v1/task/detail?task_id=${taskId}`)
    const taskRaw = pick<Record<string, unknown> | undefined>(raw, ['task'], undefined)
    const list = Array.isArray((raw as { subTasks?: unknown[] })?.subTasks)
      ? ((raw as { subTasks?: unknown[] }).subTasks ?? [])
      : Array.isArray((raw as { subtasks?: unknown[] })?.subtasks)
        ? ((raw as { subtasks?: unknown[] }).subtasks ?? [])
        : Array.isArray((raw as { sub_tasks?: unknown[] })?.sub_tasks)
          ? ((raw as { sub_tasks?: unknown[] }).sub_tasks ?? [])
          : []
    const subTasks = list.map((item, idx) => {
      const row = item as Record<string, unknown>
      const depends = pick<unknown>(row, ['dependsOn', 'depends_on'], [])
      return {
        subTaskId: pick(row, ['subTaskId', 'sub_task_id', 'id'], `sub-${idx}`),
        title: pick(row, ['title', 'name', 'action'], '子任务'),
        action: pick<string | undefined>(row, ['action'], undefined),
        status: pick(row, ['status'], 'PENDING'),
        result: pick<Record<string, unknown> | undefined>(row, ['result'], undefined),
        error: pick<string | undefined>(row, ['error'], undefined),
        dependsOn: Array.isArray(depends) ? depends.map(String) : [],
        updatedAt: pick<string | undefined>(row, ['updatedAt', 'updated_at'], undefined),
      }
    })
    const task = taskRaw
      ? ({
          params: pick<Record<string, unknown> | undefined>(taskRaw, ['params'], undefined),
          taskId: pick(taskRaw, ['taskId', 'task_id', 'id'], taskId),
          sessionId: pick<string | undefined>(taskRaw, ['sessionId', 'session_id'], undefined),
          title: resolveTaskTitle(
            taskRaw,
            pick<Record<string, unknown> | undefined>(taskRaw, ['params'], undefined),
          ),
          status: pick(taskRaw, ['status'], 'PENDING'),
          skillName: pick<string | undefined>(taskRaw, ['skillName', 'skill_name'], undefined),
          priority: pick<number | undefined>(taskRaw, ['priority'], undefined),
          result: pick<Record<string, unknown> | undefined>(taskRaw, ['result'], undefined),
          error: pick<string | undefined>(taskRaw, ['error'], undefined),
          createdAt: pick<string | undefined>(taskRaw, ['createdAt', 'created_at'], undefined),
          updatedAt: pick<string | undefined>(taskRaw, ['updatedAt', 'updated_at'], undefined),
        } as TaskItem)
      : null
    return { task, subTasks }
  },
  async cancelTask(taskId: string) {
    await request('/api/v1/task/cancel', {
      method: 'POST',
      body: JSON.stringify({ task_id: taskId }),
    })
  },
  async listWorkingMemory(taskId: string): Promise<WorkingMemory[]> {
    const raw = await request<unknown>(`/api/v1/memory/working/list?taskId=${taskId}`)
    const list = Array.isArray(raw)
      ? raw
      : Array.isArray((raw as { list?: unknown[] })?.list)
        ? ((raw as { list?: unknown[] }).list ?? [])
        : []
    return list.map((item, idx) => {
      const row = item as Record<string, unknown>
      return {
        id: pick(row, ['id', 'memoryId', 'memory_id'], `wm-${idx}`),
        content: pick(row, ['content', 'summary'], ''),
        createdAt: pick<string | undefined>(row, ['createdAt', 'created_at'], undefined),
      }
    })
  },
  async listLongMemory(taskId: string): Promise<LongMemory[]> {
    const raw = await request<unknown>(`/api/v1/memory/long/list?taskId=${taskId}`)
    const list = Array.isArray(raw)
      ? raw
      : Array.isArray((raw as { list?: unknown[] })?.list)
        ? ((raw as { list?: unknown[] }).list ?? [])
        : []
    return list.map((item, idx) => {
      const row = item as Record<string, unknown>
      return {
        id: pick(row, ['id', 'memoryId', 'memory_id'], `lm-${idx}`),
        topic: pick(row, ['topic', 'title'], '未命名主题'),
        content: pick(row, ['content', 'summary'], ''),
        createdAt: pick<string | undefined>(row, ['createdAt', 'created_at'], undefined),
      }
    })
  },
  async listSkills(): Promise<SkillItem[]> {
    const raw = await request<unknown>('/api/v1/skill/list')
    const list = Array.isArray(raw)
      ? raw
      : Array.isArray((raw as { list?: unknown[] })?.list)
        ? ((raw as { list?: unknown[] }).list ?? [])
        : []
    return list.map((item, idx) => {
      const row = item as Record<string, unknown>
      return {
        name: pick(row, ['name', 'skillName', 'skill_name'], `skill-${idx}`),
        version: pick(row, ['version'], 'v1'),
        enabled: Boolean(pick(row, ['enabled'], true)),
        memoryType: pick<string | undefined>(row, ['memoryType', 'memory_type'], undefined),
        windowSize: pick<number | undefined>(row, ['windowSize', 'window_size'], undefined),
      }
    })
  },
}

function resolveTaskTitle(row: Record<string, unknown>, params?: Record<string, unknown>): string {
  const direct = pick<string>(row, ['title', 'name'], '')
  if (typeof direct === 'string' && direct.trim()) {
    return direct.trim()
  }
  const prompt = params?.prompt
  if (typeof prompt === 'string' && prompt.trim()) {
    return prompt.trim()
  }
  return pick(row, ['skill_name', 'skillName'], '未命名任务')
}

export function getAuthToken() {
  return window.localStorage.getItem(AUTH_TOKEN_KEY) ?? ''
}

export function setAuthToken(token: string) {
  if (token) {
    window.localStorage.setItem(AUTH_TOKEN_KEY, token)
    return
  }
  window.localStorage.removeItem(AUTH_TOKEN_KEY)
}

export function streamUrl(path: string, params: Record<string, string>) {
  const query = new URLSearchParams(params).toString()
  return `${API_BASE}${path}?${query}`
}
