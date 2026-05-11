import { useEffect, useMemo, useState } from 'react'
import type { ComponentType, ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ArrowRight, BookOpen, CheckCircle2, Clock3, Database, FolderKanban } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { api } from '@/lib/api'
import { useAppStore } from '@/store/use_app_store'
import type { TaskItem } from '@/types/domain'

const PAGE_SIZE = 20

export function MemoryPage() {
  const navigate = useNavigate()
  const [tasks, setTasks] = useState<TaskItem[]>([])
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [pages, setPages] = useState(1)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    let alive = true
    setLoading(true)
    api
      .listTasksPage({ page, size: PAGE_SIZE })
      .then((result) => {
        if (!alive) {
          return
        }
        setTasks(result.list)
        setTotal(result.total)
        setPages(result.pages || 1)
      })
      .catch((error: Error) => toast.error(error.message))
      .finally(() => {
        if (alive) {
          setLoading(false)
        }
      })
    return () => {
      alive = false
    }
  }, [page])

  return (
    <div className="flex h-full flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold text-zinc-900">记忆文件</h1>
        <p className="mt-1 text-sm text-zinc-500">查看任务记忆列表，并进入独立详情页</p>
      </div>

      <div className="grid grid-cols-3 gap-4">
        <SummaryCard icon={FolderKanban} label="任务数量" value={String(total)} />
        <SummaryCard icon={Clock3} label="短期记忆" value="-" />
        <SummaryCard icon={BookOpen} label="长期记忆" value="-" />
      </div>

      <Card className="min-h-0 flex-1 overflow-hidden">
        <SectionHeader
          icon={Database}
          title="记忆列表"
          meta={`${total} records`}
        />
        <div className="overflow-auto">
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr className="border-b border-zinc-100 bg-zinc-50 text-xs font-medium text-zinc-400">
                <th className="px-5 py-3 text-left">任务</th>
                <th className="px-5 py-3 text-left">技能</th>
                <th className="px-5 py-3 text-left">状态</th>
                <th className="px-5 py-3 text-left">操作</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <EmptyTable colSpan={4}>加载中...</EmptyTable>
              ) : tasks.length ? (
                tasks.map((task) => (
                  <tr key={task.taskId} className="border-b border-zinc-100 hover:bg-indigo-50/20">
                    <td className="px-5 py-4">
                      <div className="font-medium text-zinc-900">{task.title}</div>
                      <div className="mt-1 text-xs text-zinc-400">{task.taskId}</div>
                    </td>
                    <td className="px-5 py-4 text-zinc-600">{task.skillName ?? '-'}</td>
                    <td className="px-5 py-4">
                      <Badge variant={task.status === 'COMPLETED' ? 'success' : 'outline'}>
                        {task.status}
                      </Badge>
                    </td>
                    <td className="px-5 py-4">
                      <Button
                        variant="secondary"
                        size="sm"
                        className="h-8 gap-1 rounded-xl border-zinc-200 bg-zinc-100 px-3 text-zinc-700 shadow-none hover:bg-zinc-200"
                        onClick={() => navigate(`/memory/${task.taskId}`)}
                      >
                        详情
                        <ArrowRight className="h-3.5 w-3.5" />
                      </Button>
                    </td>
                  </tr>
                ))
              ) : (
                <EmptyTable colSpan={4}>暂无任务记录</EmptyTable>
              )}
            </tbody>
          </table>
        </div>
        <div className="flex items-center justify-between border-t border-zinc-100 px-5 py-4">
          <div className="text-xs text-zinc-400">
            第 {page} / {Math.max(pages, 1)} 页 · {total} 条
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="secondary"
              size="sm"
              className="rounded-xl border-zinc-200 bg-zinc-100 text-zinc-700 shadow-none hover:bg-zinc-200"
              disabled={page <= 1 || loading}
              onClick={() => setPage((current) => Math.max(1, current - 1))}
            >
              上一页
            </Button>
            <Button
              variant="secondary"
              size="sm"
              className="rounded-xl border-zinc-200 bg-zinc-100 text-zinc-700 shadow-none hover:bg-zinc-200"
              disabled={page >= pages || loading}
              onClick={() => setPage((current) => Math.min(pages, current + 1))}
            >
              下一页
            </Button>
          </div>
        </div>
      </Card>
    </div>
  )
}

export function MemoryDetailPage() {
  const { taskId } = useParams<{ taskId: string }>()
  const navigate = useNavigate()

  const tasks = useAppStore((state) => state.tasks)
  const workingMemory = useAppStore((state) => state.workingMemory)
  const longMemory = useAppStore((state) => state.longMemory)
  const loadTasks = useAppStore((state) => state.loadTasks)
  const loadMemory = useAppStore((state) => state.loadMemory)

  useEffect(() => {
    loadTasks().catch((error: Error) => toast.error(error.message))
  }, [loadTasks])

  const currentTask = useMemo(
    () => tasks.find((task) => task.taskId === taskId) ?? null,
    [taskId, tasks],
  )

  useEffect(() => {
    if (!taskId) {
      return
    }
    loadMemory(taskId, currentTask?.skillName).catch((error: Error) => toast.error(error.message))
  }, [currentTask?.skillName, loadMemory, taskId])

  return (
    <div className="flex h-full flex-col gap-5">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-zinc-900">记忆详情</h1>
          <p className="mt-1 text-sm text-zinc-500">当前任务的上下文、短期记忆和长期记忆</p>
        </div>
        <Button
          variant="secondary"
          className="rounded-xl border-zinc-200 bg-zinc-100 text-zinc-700 shadow-none hover:bg-zinc-200"
          onClick={() => navigate('/memory')}
        >
          返回列表
        </Button>
      </div>

      <Card>
        <SectionHeader
          icon={CheckCircle2}
          title="当前任务"
          meta={currentTask ? currentTask.taskId : taskId ?? '-'}
        />
        {currentTask ? (
          <div className="grid grid-cols-3 gap-4 px-5 pb-5">
            <InfoBlock label="任务标题" value={currentTask.title} />
            <InfoBlock label="技能" value={currentTask.skillName ?? '-'} />
            <InfoBlock label="状态" value={currentTask.status} />
          </div>
        ) : (
          <div className="px-5 pb-5 text-sm text-zinc-500">未找到该任务对应的记忆记录。</div>
        )}
      </Card>

      <div className="grid min-h-0 flex-1 grid-cols-2 gap-4">
        <MemoryTableCard
          icon={Clock3}
          title="短期记忆"
          meta={`${workingMemory.length} records`}
          columns={['内容', '创建时间']}
          emptyText="当前任务暂无短期记忆"
        >
          {workingMemory.map((item) => (
            <tr key={item.id} className="border-b border-zinc-100 align-top hover:bg-indigo-50/20">
              <td className="px-5 py-4 whitespace-pre-wrap text-zinc-700">{item.content || '-'}</td>
              <td className="px-5 py-4 text-zinc-500">{item.createdAt ?? '-'}</td>
            </tr>
          ))}
        </MemoryTableCard>

        <MemoryTableCard
          icon={BookOpen}
          title="长期记忆"
          meta={`${longMemory.length} records`}
          columns={['内容', '主题']}
          emptyText="当前任务暂无长期记忆"
        >
          {longMemory.map((item) => (
            <tr key={item.id} className="border-b border-zinc-100 align-top hover:bg-indigo-50/20">
              <td className="px-5 py-4 whitespace-pre-wrap text-zinc-700">{item.content || '-'}</td>
              <td className="px-5 py-4 text-zinc-500">{item.topic || '-'}</td>
            </tr>
          ))}
        </MemoryTableCard>
      </div>
    </div>
  )
}

function SummaryCard({
  icon: Icon,
  label,
  value,
}: {
  icon: ComponentType<{ className?: string }>
  label: string
  value: string
}) {
  return (
    <Card className="p-4">
      <div className="flex items-center justify-between">
        <div>
          <div className="text-sm text-zinc-500">{label}</div>
          <div className="mt-2 text-2xl font-semibold text-zinc-900">{value}</div>
        </div>
        <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-indigo-50 text-indigo-600">
          <Icon className="h-5 w-5" />
        </div>
      </div>
    </Card>
  )
}

function SectionHeader({
  icon: Icon,
  title,
  meta,
}: {
  icon: ComponentType<{ className?: string }>
  title: string
  meta: string
}) {
  return (
    <div className="flex items-center justify-between border-b border-zinc-100 px-5 py-4">
      <div className="flex items-center gap-2 text-sm font-semibold text-zinc-900">
        <Icon className="h-4 w-4 text-indigo-500" />
        {title}
      </div>
      <div className="text-xs text-zinc-400">{meta}</div>
    </div>
  )
}

function InfoBlock({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-zinc-100 bg-zinc-50 px-4 py-3">
      <div className="text-xs font-medium text-zinc-400">{label}</div>
      <div className="mt-2 text-sm font-medium text-zinc-900">{value}</div>
    </div>
  )
}

function MemoryTableCard({
  icon,
  title,
  meta,
  columns,
  emptyText,
  children,
}: {
  icon: ComponentType<{ className?: string }>
  title: string
  meta: string
  columns: [string, string]
  emptyText: string
  children: ReactNode
}) {
  const hasRows = Array.isArray(children) ? children.length > 0 : Boolean(children)

  return (
    <Card className="min-h-0 overflow-hidden">
      <SectionHeader icon={icon} title={title} meta={meta} />
      <div className="overflow-auto">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr className="border-b border-zinc-100 bg-zinc-50 text-xs font-medium text-zinc-400">
              <th className="px-5 py-3 text-left">{columns[0]}</th>
              <th className="px-5 py-3 text-left">{columns[1]}</th>
            </tr>
          </thead>
          <tbody>{hasRows ? children : <EmptyTable colSpan={2}>{emptyText}</EmptyTable>}</tbody>
        </table>
      </div>
    </Card>
  )
}

function EmptyTable({ colSpan, children }: { colSpan: number; children: ReactNode }) {
  return (
    <tr>
      <td colSpan={colSpan} className="px-5 py-8 text-sm text-zinc-500">
        {children}
      </td>
    </tr>
  )
}
