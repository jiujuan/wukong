export type StreamType = 'THINK' | 'TOOL' | 'CHUNK' | 'STATUS' | 'FINISH'

export type ChatSession = {
  sessionId: string
  title: string
  createdAt?: string
}

export type ChatMessage = {
  id: string
  role: 'user' | 'assistant'
  content: string
  createdAt?: string
}

export type TaskStatus =
  | 'PENDING'
  | 'PLANNING'
  | 'RUNNING'
  | 'WAITING'
  | 'COMPLETED'
  | 'FAILED'
  | 'CANCELLED'

export type TaskItem = {
  taskId: string
  sessionId?: string
  title: string
  status: TaskStatus
  params?: Record<string, unknown>
  skillName?: string
  priority?: number
  result?: Record<string, unknown>
  error?: string
  createdAt?: string
  updatedAt?: string
}

export type StreamEvent = {
  seq: number
  msgType: StreamType
  content: string
  createdAt?: string
}

export type SubTask = {
  subTaskId: string
  title: string
  action?: string
  status: string
  result?: Record<string, unknown>
  error?: string
  dependsOn: string[]
  updatedAt?: string
}

export type TaskDetail = {
  task: TaskItem | null
  subTasks: SubTask[]
}

export type WorkingMemory = {
  id: string
  content: string
  createdAt?: string
}

export type LongMemory = {
  id: string
  topic: string
  content: string
  createdAt?: string
}

export type SkillItem = {
  name: string
  version: string
  enabled: boolean
  memoryType?: string
  windowSize?: number
}
