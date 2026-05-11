import { useEffect, useMemo } from 'react'
import { toast } from 'sonner'
import { Card } from '@/components/ui/card'
import { useAppStore } from '@/store/use_app_store'

export function MemoryPage() {
  const tasks = useAppStore((state) => state.tasks)
  const currentTaskId = useAppStore((state) => state.currentTaskId)
  const workingMemory = useAppStore((state) => state.workingMemory)
  const longMemory = useAppStore((state) => state.longMemory)
  const setCurrentTask = useAppStore((state) => state.setCurrentTask)
  const loadTasks = useAppStore((state) => state.loadTasks)
  const loadMemory = useAppStore((state) => state.loadMemory)

  useEffect(() => {
    loadTasks().catch((error: Error) => toast.error(error.message))
  }, [loadTasks])

  useEffect(() => {
    if (!tasks.length) {
      return
    }
    if (!currentTaskId) {
      setCurrentTask(tasks[0].taskId)
    }
  }, [currentTaskId, setCurrentTask, tasks])

  const currentTask = useMemo(
    () => tasks.find((task) => task.taskId === currentTaskId) ?? null,
    [currentTaskId, tasks],
  )

  useEffect(() => {
    if (!currentTaskId) {
      return
    }
    loadMemory(currentTaskId, currentTask?.skillName).catch((error: Error) =>
      toast.error(error.message),
    )
  }, [currentTask?.skillName, currentTaskId, loadMemory])

  return (
    <Card className="h-full p-4">
      <div className="mb-4 text-lg font-semibold">记忆文件</div>

      <div className="mb-4 rounded-md border border-zinc-800">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr className="border-b border-zinc-800 bg-zinc-900">
              <th className="px-3 py-2 text-left">当前任务</th>
              <th className="px-3 py-2 text-left">技能</th>
              <th className="px-3 py-2 text-left">状态</th>
            </tr>
          </thead>
          <tbody>
            {currentTask ? (
              <tr className="border-b border-zinc-900">
                <td className="px-3 py-2">
                  <div className="font-medium">{currentTask.title}</div>
                  <div className="mt-1 text-xs text-zinc-500">{currentTask.taskId}</div>
                </td>
                <td className="px-3 py-2">{currentTask.skillName ?? '-'}</td>
                <td className="px-3 py-2">{currentTask.status}</td>
              </tr>
            ) : (
              <tr>
                <td colSpan={3} className="px-3 py-6 text-zinc-500">
                  暂无任务，请先在任务中心选择或创建任务。
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="mb-4 rounded-md border border-zinc-800">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr className="border-b border-zinc-800 bg-zinc-900">
              <th className="px-3 py-2 text-left">任务列表</th>
              <th className="px-3 py-2 text-left">技能</th>
              <th className="px-3 py-2 text-left">状态</th>
            </tr>
          </thead>
          <tbody>
            {tasks.length ? (
              tasks.map((task) => (
                <tr
                  key={task.taskId}
                  className={`border-b border-zinc-900 transition-colors ${
                    task.taskId === currentTaskId ? 'bg-zinc-950/60' : ''
                  }`}
                >
                  <td className="px-3 py-2">
                    <button
                      className="w-full text-left"
                      onClick={() => setCurrentTask(task.taskId)}
                    >
                      <div className="font-medium">{task.title}</div>
                      <div className="mt-1 text-xs text-zinc-500">{task.taskId}</div>
                    </button>
                  </td>
                  <td className="px-3 py-2">{task.skillName ?? '-'}</td>
                  <td className="px-3 py-2">{task.status}</td>
                </tr>
              ))
            ) : (
              <tr>
                <td colSpan={3} className="px-3 py-6 text-zinc-500">
                  暂无任务记录。
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="grid h-[calc(100%-252px)] min-h-[320px] grid-cols-2 gap-4">
        <div className="rounded-md border border-zinc-800">
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr className="border-b border-zinc-800 bg-zinc-900">
                <th className="px-3 py-2 text-left">短期记忆</th>
                <th className="px-3 py-2 text-left">创建时间</th>
              </tr>
            </thead>
            <tbody>
              {workingMemory.length ? (
                workingMemory.map((item) => (
                  <tr key={item.id} className="border-b border-zinc-900 align-top">
                    <td className="px-3 py-2 whitespace-pre-wrap">{item.content || '-'}</td>
                    <td className="px-3 py-2 text-zinc-500">{item.createdAt ?? '-'}</td>
                  </tr>
                ))
              ) : (
                <tr>
                  <td colSpan={2} className="px-3 py-6 text-zinc-500">
                    当前任务暂无短期记忆。
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        <div className="rounded-md border border-zinc-800">
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr className="border-b border-zinc-800 bg-zinc-900">
                <th className="px-3 py-2 text-left">长期记忆</th>
                <th className="px-3 py-2 text-left">主题</th>
              </tr>
            </thead>
            <tbody>
              {longMemory.length ? (
                longMemory.map((item) => (
                  <tr key={item.id} className="border-b border-zinc-900 align-top">
                    <td className="px-3 py-2 whitespace-pre-wrap">{item.content || '-'}</td>
                    <td className="px-3 py-2 text-zinc-500">{item.topic || '-'}</td>
                  </tr>
                ))
              ) : (
                <tr>
                  <td colSpan={2} className="px-3 py-6 text-zinc-500">
                    当前任务暂无长期记忆。
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </Card>
  )
}
