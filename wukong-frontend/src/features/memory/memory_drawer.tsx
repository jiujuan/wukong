import { useEffect } from 'react'
import type { ComponentType, ReactNode } from 'react'
import { BookOpen, Clock3 } from 'lucide-react'
import { toast } from 'sonner'
import { Card } from '@/components/ui/card'
import { Sheet } from '@/components/ui/sheet'
import { useAppStore } from '@/store/use_app_store'

export function MemoryDrawer() {
  const memoryOpen = useAppStore((state) => state.memoryOpen)
  const currentTaskId = useAppStore((state) => state.currentTaskId)
  const workingMemory = useAppStore((state) => state.workingMemory)
  const longMemory = useAppStore((state) => state.longMemory)
  const toggleMemory = useAppStore((state) => state.toggleMemory)
  const loadMemory = useAppStore((state) => state.loadMemory)

  useEffect(() => {
    if (!memoryOpen || !currentTaskId) {
      return
    }
    loadMemory(currentTaskId).catch((error: Error) => toast.error(error.message))
  }, [currentTaskId, loadMemory, memoryOpen])

  return (
    <Sheet open={memoryOpen} onClose={() => toggleMemory(false)} title="记忆文件">
      {!currentTaskId ? (
        <div className="rounded-xl border border-dashed border-zinc-200 bg-zinc-50 p-6 text-center text-sm text-zinc-500">
          请选择任务后查看记忆
        </div>
      ) : (
        <div className="space-y-6">
          <MemorySection
            icon={Clock3}
            title="短期记忆"
            subtitle={`${workingMemory.length} 条工作上下文`}
          >
            {workingMemory.length === 0 ? (
              <EmptyMemory />
            ) : (
              workingMemory.map((item) => (
                <Card key={item.id} className="p-3 shadow-none">
                  <div className="text-sm leading-6 whitespace-pre-wrap text-zinc-700">{item.content}</div>
                </Card>
              ))
            )}
          </MemorySection>

          <MemorySection
            icon={BookOpen}
            title="长期记忆"
            subtitle={`${longMemory.length} 条沉淀经验`}
          >
            {longMemory.length === 0 ? (
              <EmptyMemory />
            ) : (
              longMemory.map((item) => (
                <Card key={item.id} className="p-3 shadow-none">
                  <div className="mb-1 text-sm font-medium text-zinc-900">{item.topic}</div>
                  <div className="text-sm leading-6 whitespace-pre-wrap text-zinc-600">{item.content}</div>
                </Card>
              ))
            )}
          </MemorySection>
        </div>
      )}
    </Sheet>
  )
}

function MemorySection({
  icon: Icon,
  title,
  subtitle,
  children,
}: {
  icon: ComponentType<{ className?: string }>
  title: string
  subtitle: string
  children: ReactNode
}) {
  return (
    <section>
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-indigo-50 text-indigo-600">
            <Icon className="h-4 w-4" />
          </div>
          <div>
            <div className="text-sm font-semibold text-zinc-900">{title}</div>
            <div className="text-xs text-zinc-400">{subtitle}</div>
          </div>
        </div>
      </div>
      <div className="space-y-2">{children}</div>
    </section>
  )
}

function EmptyMemory() {
  return (
    <div className="rounded-xl border border-dashed border-zinc-200 bg-zinc-50 p-4 text-sm text-zinc-400">
      暂无记忆数据
    </div>
  )
}
