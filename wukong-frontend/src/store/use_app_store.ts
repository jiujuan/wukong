import { create } from 'zustand'
import { api } from '@/lib/api'
import type {
  ChatMessage,
  ChatSession,
  LongMemory,
  SkillItem,
  StreamEvent,
  TaskItem,
  WorkingMemory,
} from '@/types/domain'

type AppState = {
  sessions: ChatSession[]
  currentSessionId: string
  messagesBySession: Record<string, ChatMessage[]>
  tasks: TaskItem[]
  currentTaskId: string
  eventsByTask: Record<string, StreamEvent[]>
  workingMemory: WorkingMemory[]
  longMemory: LongMemory[]
  skills: SkillItem[]
  memoryOpen: boolean
  setCurrentSession: (sessionId: string) => void
  setCurrentTask: (taskId: string) => void
  toggleMemory: (open?: boolean) => void
  appendMessage: (sessionId: string, message: ChatMessage) => void
  updateLastAssistantMessage: (sessionId: string, content: string) => void
  appendTaskEvent: (taskId: string, event: StreamEvent) => void
  upsertTask: (task: TaskItem) => void
  updateTaskStatus: (taskId: string, status: TaskItem['status']) => void
  loadSessions: () => Promise<void>
  loadMessages: (sessionId: string) => Promise<void>
  createSession: () => Promise<ChatSession>
  deleteSession: (sessionId: string) => Promise<void>
  createTask: (input: {
    skillName: string
    params?: Record<string, unknown>
    priority?: number
    sessionId?: string
  }) => Promise<TaskItem>
  loadTasks: () => Promise<void>
  loadMemory: (taskId: string) => Promise<void>
  loadSkills: () => Promise<void>
}

export const useAppStore = create<AppState>((set) => ({
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
  setCurrentSession: (sessionId) => set({ currentSessionId: sessionId }),
  setCurrentTask: (taskId) => set({ currentTaskId: taskId }),
  toggleMemory: (open) => set((state) => ({ memoryOpen: open ?? !state.memoryOpen })),
  appendMessage: (sessionId, message) =>
    set((state) => ({
      messagesBySession: {
        ...state.messagesBySession,
        [sessionId]: [...(state.messagesBySession[sessionId] ?? []), message],
      },
    })),
  updateLastAssistantMessage: (sessionId, content) =>
    set((state) => {
      const list = [...(state.messagesBySession[sessionId] ?? [])]
      for (let idx = list.length - 1; idx >= 0; idx -= 1) {
        if (list[idx].role === 'assistant') {
          list[idx] = { ...list[idx], content }
          return {
            messagesBySession: {
              ...state.messagesBySession,
              [sessionId]: list,
            },
          }
        }
      }
      return {
        messagesBySession: {
          ...state.messagesBySession,
          [sessionId]: [
            ...list,
            { id: crypto.randomUUID(), role: 'assistant', content },
          ],
        },
      }
    }),
  appendTaskEvent: (taskId, event) =>
    set((state) => ({
      eventsByTask: {
        ...state.eventsByTask,
        [taskId]: [...(state.eventsByTask[taskId] ?? []), event].slice(-300),
      },
    })),
  upsertTask: (task) =>
    set((state) => {
      const index = state.tasks.findIndex((item) => item.taskId === task.taskId)
      if (index >= 0) {
        const tasks = [...state.tasks]
        tasks[index] = { ...tasks[index], ...task }
        return { tasks }
      }
      return { tasks: [task, ...state.tasks] }
    }),
  updateTaskStatus: (taskId, status) =>
    set((state) => ({
      tasks: state.tasks.map((task) =>
        task.taskId === taskId ? { ...task, status, updatedAt: new Date().toISOString() } : task,
      ),
    })),
  async loadSessions() {
    const sessions = await api.listSessions()
    set((state) => ({
      sessions,
      currentSessionId: state.currentSessionId || sessions[0]?.sessionId || '',
    }))
  },
  async loadMessages(sessionId) {
    const messages = await api.listMessages(sessionId)
    set((state) => ({
      messagesBySession: {
        ...state.messagesBySession,
        [sessionId]: messages,
      },
    }))
  },
  async createSession() {
    const session = await api.createSession('新会话')
    set((state) => ({
      sessions: [session, ...state.sessions],
      currentSessionId: session.sessionId,
      messagesBySession: { ...state.messagesBySession, [session.sessionId]: [] },
    }))
    return session
  },
  async deleteSession(sessionId) {
    await api.deleteSession(sessionId)
    set((state) => {
      const sessions = state.sessions.filter((item) => item.sessionId !== sessionId)
      const currentSessionId =
        state.currentSessionId === sessionId ? (sessions[0]?.sessionId ?? '') : state.currentSessionId
      return { sessions, currentSessionId }
    })
  },
  async createTask(input) {
    const task = await api.createTask(input)
    set((state) => ({
      tasks: [task, ...state.tasks.filter((item) => item.taskId !== task.taskId)],
      currentTaskId: task.taskId,
    }))
    return task
  },
  async loadTasks() {
    const tasks = await api.listTasks()
    set((state) => {
      const mergedMap = new Map(state.tasks.map((item) => [item.taskId, item]))
      for (const task of tasks) {
        mergedMap.set(task.taskId, { ...mergedMap.get(task.taskId), ...task })
      }
      const merged = Array.from(mergedMap.values()).sort((a, b) =>
        (b.createdAt ?? '').localeCompare(a.createdAt ?? ''),
      )
      const currentExists = merged.some((task) => task.taskId === state.currentTaskId)
      return {
        tasks: merged,
        currentTaskId: currentExists ? state.currentTaskId : (merged[0]?.taskId ?? ''),
      }
    })
  },
  async loadMemory(taskId) {
    const [workingMemory, longMemory] = await Promise.all([
      api.listWorkingMemory(taskId),
      api.listLongMemory(taskId),
    ])
    set({ workingMemory, longMemory })
  },
  async loadSkills() {
    const skills = await api.listSkills()
    set({ skills })
  },
}))

export function selectCurrentMessages() {
  const state = useAppStore.getState()
  return state.messagesBySession[state.currentSessionId] ?? []
}

export function selectCurrentTaskEvents() {
  const state = useAppStore.getState()
  return state.eventsByTask[state.currentTaskId] ?? []
}
