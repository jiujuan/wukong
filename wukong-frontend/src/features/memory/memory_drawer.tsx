import { useEffect } from 'react'
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
    <Sheet open={memoryOpen} onClose={() => toggleMemory(false)} title="记忆面板">
      {!currentTaskId ? (
        <div className="text-sm text-zinc-500">请选择任务后查看记忆</div>
      ) : (
        <div className="space-y-5">
          <div>
            <div className="mb-2 text-sm font-semibold">短期记忆</div>
            <div className="space-y-2">
              {workingMemory.map((item) => (
                <Card key={item.id} className="p-3">
                  <div className="text-sm text-zinc-200 whitespace-pre-wrap">{item.content}</div>
                </Card>
              ))}
            </div>
          </div>
          <div>
            <div className="mb-2 text-sm font-semibold">长期记忆</div>
            <div className="space-y-2">
              {longMemory.map((item) => (
                <Card key={item.id} className="p-3">
                  <div className="mb-1 text-sm font-medium text-zinc-100">{item.topic}</div>
                  <div className="text-sm text-zinc-300 whitespace-pre-wrap">{item.content}</div>
                </Card>
              ))}
            </div>
          </div>
        </div>
      )}
    </Sheet>
  )
}
