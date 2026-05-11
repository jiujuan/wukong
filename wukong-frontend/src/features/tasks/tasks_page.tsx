import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { ComponentType, ReactNode } from 'react'
import { Activity, ArrowLeft, GitBranch, ListTodo, Plus, TerminalSquare } from 'lucide-react'
import { toast } from 'sonner'
import ReactFlow, {
  Background,
  Controls,
  MarkerType,
  type Edge,
  type Node,
} from 'reactflow'
import 'reactflow/dist/style.css'
import { useNavigate, useParams } from 'react-router-dom'
import { StatusPill } from '@/components/common/status_pill'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Sheet } from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import { api, streamUrl } from '@/lib/api'
import { createSSE } from '@/lib/sse'
import { useAppStore } from '@/store/use_app_store'
import type { StreamType, TaskStatus } from '@/types/domain'

const streamFilters: StreamType[] = ['THINK', 'TOOL', 'CHUNK', 'STATUS', 'FINISH']
type TraceItem = { status: string; reason?: string; seq: number }

function getTraceDisplay(item: TraceItem, next?: TraceItem) {
  if (item.status === 'PLANNING' && next && next.status !== 'PLANNING') {
    return { label: '规划完成', completed: true }
  }
  if (item.status === 'RUNNING' && next && next.status !== 'RUNNING') {
    return { label: '执行完成', completed: true }
  }
  return { label: undefined, completed: false }
}

export function TasksPage() {
  const navigate = useNavigate()
  const { taskId: routeTaskId } = useParams()
  const isDetailPage = Boolean(routeTaskId)
  const [activeFilter, setActiveFilter] = useState<StreamType | 'ALL'>('ALL')
  const [nodes, setNodes] = useState<Node[]>([])
  const [edges, setEdges] = useState<Edge[]>([])
  const [sheetOpen, setSheetOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [taskPrompt, setTaskPrompt] = useState('')
  const [taskSkill, setTaskSkill] = useState('general')
  const [taskPriority, setTaskPriority] = useState(5)
  const [taskResult, setTaskResult] = useState<string>('')
  const [statusTrace, setStatusTrace] = useState<TraceItem[]>([])
  const replayedTaskRef = useRef<Set<string>>(new Set())

  const tasks = useAppStore((state) => state.tasks)
  const currentTaskId = useAppStore((state) => state.currentTaskId)
  const eventsByTask = useAppStore((state) => state.eventsByTask)
  const setCurrentTask = useAppStore((state) => state.setCurrentTask)
  const appendTaskEvent = useAppStore((state) => state.appendTaskEvent)
  const upsertTask = useAppStore((state) => state.upsertTask)
  const updateTaskStatus = useAppStore((state) => state.updateTaskStatus)
  const loadTasks = useAppStore((state) => state.loadTasks)
  const createTask = useAppStore((state) => state.createTask)

  const selectedTaskId = routeTaskId ?? currentTaskId
  const currentTask = tasks.find((item) => item.taskId === selectedTaskId)
  const currentEvents = useMemo(
    () => eventsByTask[selectedTaskId] ?? [],
    [eventsByTask, selectedTaskId],
  )
  const filteredEvents = useMemo(
    () =>
      activeFilter === 'ALL'
        ? currentEvents
        : currentEvents.filter((event) => event.msgType === activeFilter),
    [activeFilter, currentEvents],
  )

  useEffect(() => {
    loadTasks().catch((error: Error) => toast.error(error.message))
  }, [loadTasks])

  useEffect(() => {
    if (!routeTaskId) {
      return
    }
    setCurrentTask(routeTaskId)
  }, [routeTaskId, setCurrentTask])

  useEffect(() => {
    if (!selectedTaskId) {
      return
    }
    const streamKey = `task:${selectedTaskId}`
    if (!replayedTaskRef.current.has(selectedTaskId)) {
      window.localStorage.setItem(streamKey, '0')
      replayedTaskRef.current.add(selectedTaskId)
    }
    const close = createSSE(
      streamUrl('/api/v1/stream/task', { taskId: selectedTaskId }),
      streamKey,
      (event) => {
        appendTaskEvent(selectedTaskId, event)
        if (event.msgType === 'STATUS') {
          const parsed = parseStatusPayload(event.content)
          const status = parsed.status
          const reason = parsed.reason
          if (status) {
            updateTaskStatus(selectedTaskId, status)
            setStatusTrace((prev) => {
              const last = prev[prev.length - 1]
              if (last?.status === status && last?.reason === reason) {
                return prev
              }
              return [...prev, { status, reason, seq: event.seq }]
            })
          }
        }
        if (event.msgType === 'CHUNK' && event.content) {
          setTaskResult((prev) => (prev ? `${prev}\n${event.content}` : event.content))
        }
        if (event.msgType === 'FINISH') {
          toast.success('任务执行完成')
          loadTasks().catch(() => {})
          api
            .taskDetail(selectedTaskId)
            .then((detail) => {
              if (detail.task?.result) {
                setTaskResult(formatTaskResult(detail.task.result))
              }
            })
            .catch(() => {})
        }
      },
      () => toast.warning('任务流已断开，正在等待重连'),
    )
    return () => close()
  }, [appendTaskEvent, loadTasks, selectedTaskId, updateTaskStatus])

  const loadTaskDetail = useCallback(
    async (taskId: string) => {
      const detail = await api.taskDetail(taskId)
      if (detail.task) {
        upsertTask(detail.task)
        setStatusTrace((prev) => {
          const status = normalizeTaskStatus(detail.task?.status ?? '')
          if (!status) {
            return prev
          }
          if (prev.some((item) => item.status === status)) {
            return prev
          }
          return [{ status, reason: '任务详情加载', seq: 0 }, ...prev]
        })
        if (detail.task.result) {
          setTaskResult(formatTaskResult(detail.task.result))
        } else if (detail.task.error) {
          setTaskResult(detail.task.error)
        } else {
          setTaskResult('')
        }
      }
      const mapLevel = new Map<string, number>()
      const byId = new Map(detail.subTasks.map((item) => [item.subTaskId, item]))
      const levelOf = (id: string): number => {
        if (mapLevel.has(id)) {
          return mapLevel.get(id)!
        }
        const item = byId.get(id)
        if (!item || item.dependsOn.length === 0) {
          mapLevel.set(id, 0)
          return 0
        }
        const level = Math.max(...item.dependsOn.map(levelOf)) + 1
        mapLevel.set(id, level)
        return level
      }
      detail.subTasks.forEach((item) => levelOf(item.subTaskId))
      const levelCount = new Map<number, number>()
      setNodes(
        detail.subTasks.map((item) => {
          const level = mapLevel.get(item.subTaskId) ?? 0
          const count = levelCount.get(level) ?? 0
          levelCount.set(level, count + 1)
          return {
            id: item.subTaskId,
            position: { x: level * 260, y: count * 130 },
            data: { label: `${item.title} (${item.status})` },
            type: 'default',
            style: {
              border: '1px solid #e4e4e7',
              borderRadius: 12,
              background: '#ffffff',
              color: '#27272a',
              boxShadow: '0 10px 24px rgba(31,36,48,0.06)',
            },
          }
        }),
      )
      setEdges(
        detail.subTasks.flatMap((item) =>
          item.dependsOn.map((dep) => ({
            id: `${dep}-${item.subTaskId}`,
            source: dep,
            target: item.subTaskId,
            markerEnd: { type: MarkerType.ArrowClosed },
            style: { stroke: '#a5b4fc' },
          })),
        ),
      )
    },
    [upsertTask],
  )

  useEffect(() => {
    if (!selectedTaskId) {
      return
    }
    setTaskResult('')
    setStatusTrace([])
    loadTaskDetail(selectedTaskId).catch((error: Error) => toast.error(error.message))
  }, [loadTaskDetail, selectedTaskId])

  const cancelTask = async () => {
    if (!selectedTaskId) {
      return
    }
    try {
      await api.cancelTask(selectedTaskId)
      toast.success('任务已取消')
      updateTaskStatus(selectedTaskId, 'CANCELLED')
      await loadTasks()
    } catch (error) {
      toast.error((error as Error).message)
    }
  }

  const submitTask = async () => {
    if (!taskPrompt.trim()) {
      toast.warning('请输入任务描述')
      return
    }
    if (!taskSkill.trim()) {
      toast.warning('请输入技能名')
      return
    }
    setCreating(true)
    try {
      const created = await createTask({
        skillName: taskSkill.trim(),
        priority: taskPriority,
        params: { prompt: taskPrompt.trim() },
      })
      toast.success('任务已提交')
      setSheetOpen(false)
      setTaskPrompt('')
      navigate(`/tasks/${created.taskId}`)
    } catch (error) {
      toast.error((error as Error).message)
    } finally {
      setCreating(false)
    }
  }

  const cancelDisabled =
    !selectedTaskId ||
    currentTask?.status === 'COMPLETED' ||
    currentTask?.status === 'FAILED' ||
    currentTask?.status === 'CANCELLED'

  if (!isDetailPage) {
    return (
      <div className="flex h-full flex-col gap-5">
        <PageHeader
          title="任务中心"
          description="提交可追踪任务，并查看实时执行过程与最终结果"
          action={
            <Button className="gap-2" onClick={() => setSheetOpen(true)}>
              <Plus className="h-4 w-4" />
              提交任务
            </Button>
          }
        />
        <Card className="min-h-0 flex-1 overflow-hidden">
          <SectionTitle icon={ListTodo} title="任务列表" description={`${tasks.length} 个任务`} />
          <div className="min-h-0 space-y-2 overflow-auto p-4 pt-0">
            {tasks.length === 0 ? (
              <EmptyState text="暂无任务，请先提交一个任务" />
            ) : (
              tasks.map((task) => (
                <TaskRow key={task.taskId} task={task} onClick={() => navigate(`/tasks/${task.taskId}`)} />
              ))
            )}
          </div>
        </Card>
        <TaskSheet
          open={sheetOpen}
          creating={creating}
          taskSkill={taskSkill}
          taskPriority={taskPriority}
          taskPrompt={taskPrompt}
          setTaskSkill={setTaskSkill}
          setTaskPriority={setTaskPriority}
          setTaskPrompt={setTaskPrompt}
          submitTask={submitTask}
          onClose={() => setSheetOpen(false)}
        />
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col gap-5">
      <PageHeader
        title="任务详情"
        description={selectedTaskId ?? '未选择任务'}
        action={
          <div className="flex gap-2">
            <Button variant="secondary" className="gap-2" onClick={() => navigate('/tasks')}>
              <ArrowLeft className="h-4 w-4" />
              返回
            </Button>
            <Button
              variant="destructive"
              onClick={cancelTask}
              disabled={cancelDisabled}
              className="disabled:pointer-events-auto disabled:cursor-not-allowed"
            >
              取消任务
            </Button>
          </div>
        }
      />

      <div className="grid min-h-0 grid-cols-[320px_1fr] gap-5">
        <div className="min-h-0 space-y-5">
          <Card className="overflow-hidden">
            <SectionTitle icon={ListTodo} title="任务列表" description="选择任务查看详情" />
            <div className="max-h-[360px] space-y-2 overflow-auto p-4 pt-0">
              {tasks.map((item) => (
                <TaskRow
                  key={item.taskId}
                  task={item}
                  active={item.taskId === selectedTaskId}
                  dense
                  onClick={() => navigate(`/tasks/${item.taskId}`)}
                />
              ))}
            </div>
          </Card>
          <Card className="p-4">
            <div className="mb-3 text-sm font-semibold text-zinc-900">当前任务</div>
            {selectedTaskId ? (
              <div className="space-y-3 rounded-xl border border-zinc-100 bg-zinc-50 p-3">
                <div className="break-all text-xs text-zinc-400">{selectedTaskId}</div>
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0 flex-1 text-sm font-medium text-zinc-900">
                    {currentTask?.title ?? '未命名任务'}
                  </div>
                  <StatusPill status={currentTask?.status ?? 'PENDING'} />
                </div>
              </div>
            ) : (
              <EmptyState text="暂无任务" />
            )}
          </Card>
          <Card className="p-4">
            <div className="mb-3 text-sm font-semibold text-zinc-900">状态变化</div>
            <div className="max-h-[210px] space-y-2 overflow-auto">
              {statusTrace.length === 0 ? (
                <div className="text-sm text-zinc-400">暂无状态变化</div>
              ) : (
                statusTrace.map((item, index) => {
                  const next = statusTrace[index + 1]
                  const display = getTraceDisplay(item, next)
                  return (
                    <div key={`${item.seq}-${item.status}`} className="rounded-xl border border-zinc-100 bg-zinc-50 p-3">
                      <div className="mb-1 flex items-center gap-2">
                        <StatusPill
                          status={item.status}
                          labelOverride={display.label}
                          completedStyle={display.completed}
                        />
                        <span className="text-xs text-zinc-400">seq {item.seq}</span>
                      </div>
                      {item.reason ? <div className="text-xs text-zinc-500">{item.reason}</div> : null}
                    </div>
                  )
                })
              )}
            </div>
          </Card>
        </div>

        <div className="min-h-0 space-y-5 overflow-auto">
          <Card className="overflow-hidden">
            <div className="flex items-center justify-between border-b border-zinc-100 px-4 py-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-zinc-900">
                <Activity className="h-4 w-4 text-indigo-500" />
                实时执行面板
              </div>
              <div className="flex flex-wrap gap-2">
                <FilterBadge active={activeFilter === 'ALL'} onClick={() => setActiveFilter('ALL')}>
                  ALL
                </FilterBadge>
                {streamFilters.map((filter) => (
                  <FilterBadge
                    key={filter}
                    active={activeFilter === filter}
                    onClick={() => setActiveFilter(filter)}
                  >
                    {filter}
                  </FilterBadge>
                ))}
              </div>
            </div>
            <div className="h-[300px] space-y-2 overflow-auto bg-zinc-50/60 p-4">
              {filteredEvents.length === 0 ? (
                <EmptyState text="等待实时事件" />
              ) : (
                filteredEvents.map((event, index) => (
                  <div key={`${event.seq}-${index}`} className="rounded-xl border border-zinc-100 bg-white p-3">
                    <div className="mb-2 flex items-center gap-2">
                      <Badge variant="default">{event.msgType}</Badge>
                      <span className="text-xs text-zinc-400">seq {event.seq}</span>
                    </div>
                    <div className="whitespace-pre-wrap text-sm leading-6 text-zinc-700">{event.content}</div>
                  </div>
                ))
              )}
            </div>
          </Card>

          <Card className="overflow-hidden">
            <SectionTitle icon={GitBranch} title="子任务 DAG" description="规划与依赖关系" />
            <div className="h-[320px] border-t border-zinc-100 bg-white">
              <ReactFlow nodes={nodes} edges={edges} fitView>
                <Background color="#e4e4e7" gap={16} />
                <Controls showInteractive={false} />
              </ReactFlow>
            </div>
          </Card>

          <Card className="overflow-hidden">
            <SectionTitle icon={TerminalSquare} title="最终结果" description="聚合后的任务输出" />
            <div className="max-h-[260px] overflow-auto border-t border-zinc-100 bg-zinc-50 p-4 text-sm leading-6 whitespace-pre-wrap text-zinc-700">
              {taskResult || '等待执行结果...'}
            </div>
          </Card>
        </div>
      </div>
    </div>
  )
}

function PageHeader({
  title,
  description,
  action,
}: {
  title: string
  description: string
  action?: ReactNode
}) {
  return (
    <div className="flex items-center justify-between gap-4">
      <div className="min-w-0">
        <h1 className="text-xl font-semibold text-zinc-900">{title}</h1>
        <p className="mt-1 truncate text-sm text-zinc-500">{description}</p>
      </div>
      {action}
    </div>
  )
}

function SectionTitle({
  icon: Icon,
  title,
  description,
}: {
  icon: ComponentType<{ className?: string }>
  title: string
  description: string
}) {
  return (
    <div className="flex items-center justify-between px-4 py-3">
      <div className="flex items-center gap-2 text-sm font-semibold text-zinc-900">
        <Icon className="h-4 w-4 text-indigo-500" />
        {title}
      </div>
      <div className="text-xs text-zinc-400">{description}</div>
    </div>
  )
}

function TaskRow({
  task,
  active = false,
  dense = false,
  onClick,
}: {
  task: { taskId: string; title: string; status: string; skillName?: string; createdAt?: string }
  active?: boolean
  dense?: boolean
  onClick: () => void
}) {
  return (
    <button
      className={`w-full rounded-xl border p-3 text-left transition-colors ${
        active
          ? 'border-indigo-200 bg-indigo-50'
          : 'border-zinc-100 bg-white hover:border-indigo-100 hover:bg-indigo-50/40'
      }`}
      onClick={onClick}
    >
      <div className="mb-2 flex items-center justify-between gap-3">
        <div className="min-w-0 flex-1 truncate text-sm font-medium text-zinc-900">{task.title}</div>
        <StatusPill status={task.status} />
      </div>
      <div className="flex items-center justify-between gap-3 text-xs text-zinc-400">
        <span className="min-w-0 truncate">{task.taskId}</span>
        {!dense ? <span>{task.skillName ?? 'general'}</span> : null}
      </div>
    </button>
  )
}

function FilterBadge({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: ReactNode
}) {
  return (
    <Badge
      variant={active ? 'default' : 'outline'}
      className="cursor-pointer transition-colors hover:border-indigo-200 hover:bg-indigo-50 hover:text-indigo-700"
      onClick={onClick}
    >
      {children}
    </Badge>
  )
}

function EmptyState({ text }: { text: string }) {
  return (
    <div className="rounded-xl border border-dashed border-zinc-200 bg-white px-4 py-8 text-center text-sm text-zinc-400">
      {text}
    </div>
  )
}

function TaskSheet({
  open,
  creating,
  taskSkill,
  taskPriority,
  taskPrompt,
  setTaskSkill,
  setTaskPriority,
  setTaskPrompt,
  submitTask,
  onClose,
}: {
  open: boolean
  creating: boolean
  taskSkill: string
  taskPriority: number
  taskPrompt: string
  setTaskSkill: (value: string) => void
  setTaskPriority: (value: number) => void
  setTaskPrompt: (value: string) => void
  submitTask: () => Promise<void>
  onClose: () => void
}) {
  return (
    <Sheet open={open} onClose={onClose} title="提交任务执行">
      <div className="space-y-4">
        <div className="space-y-2">
          <div className="text-sm font-medium text-zinc-700">技能名</div>
          <Input value={taskSkill} onChange={(event) => setTaskSkill(event.target.value)} />
        </div>
        <div className="space-y-2">
          <div className="text-sm font-medium text-zinc-700">优先级 (1-10)</div>
          <Input
            type="number"
            min={1}
            max={10}
            value={taskPriority}
            onChange={(event) => {
              const value = Number(event.target.value)
              if (!Number.isFinite(value)) {
                return
              }
              setTaskPriority(Math.max(1, Math.min(10, value)))
            }}
          />
        </div>
        <div className="space-y-2">
          <div className="text-sm font-medium text-zinc-700">任务描述</div>
          <Textarea
            value={taskPrompt}
            onChange={(event) => setTaskPrompt(event.target.value)}
            placeholder="请输入要执行的任务目标和约束"
          />
        </div>
        <div className="flex justify-end">
          <Button onClick={() => void submitTask()} disabled={creating}>
            {creating ? '提交中...' : '提交执行'}
          </Button>
        </div>
      </div>
    </Sheet>
  )
}

function parseStatusPayload(content: string): { status: TaskStatus | null; reason?: string } {
  const text = content.trim()
  const direct = normalizeTaskStatus(text)
  if (direct) {
    return { status: direct }
  }
  try {
    const parsed = JSON.parse(content) as unknown
    if (typeof parsed === 'string') {
      return { status: normalizeTaskStatus(parsed.trim()) }
    }
    if (parsed && typeof parsed === 'object') {
      const obj = parsed as Record<string, unknown>
      const status = normalizeTaskStatus(
        String(obj.status ?? obj.state ?? obj.task_status ?? obj.taskStatus ?? ''),
      )
      const reason = typeof obj.reason === 'string' ? obj.reason : undefined
      return { status, reason }
    }
  } catch {
    return { status: null }
  }
  return { status: null }
}

function normalizeTaskStatus(value: string): TaskStatus | null {
  if (
    value === 'PENDING' ||
    value === 'PLANNING' ||
    value === 'RUNNING' ||
    value === 'WAITING' ||
    value === 'COMPLETED' ||
    value === 'FAILED' ||
    value === 'CANCELLED'
  ) {
    return value
  }
  return null
}

function formatTaskResult(result: unknown): string {
  const extracted = extractReadableText(result)
  if (extracted) {
    return extracted
  }
  if (typeof result === 'string') {
    return result
  }
  try {
    return JSON.stringify(result, null, 2)
  } catch {
    return String(result ?? '')
  }
}

function extractReadableText(value: unknown): string | null {
  const direct = readCandidateText(value)
  if (direct) {
    return direct
  }
  if (Array.isArray(value)) {
    const joined = value
      .map((item) => extractReadableText(item))
      .filter((item): item is string => Boolean(item))
      .join('\n\n')
      .trim()
    return joined || null
  }
  if (value && typeof value === 'object') {
    for (const nested of Object.values(value as Record<string, unknown>)) {
      const text = extractReadableText(nested)
      if (text) {
        return text
      }
    }
  }
  return null
}

function readCandidateText(value: unknown): string | null {
  if (!value || typeof value !== 'object') {
    return null
  }
  const record = value as Record<string, unknown>
  const keys = ['final_answer', 'answer', 'output', 'content', 'summary', 'text', 'message']
  for (const key of keys) {
    const raw = record[key]
    if (typeof raw === 'string' && raw.trim()) {
      return raw.trim()
    }
  }
  return null
}
