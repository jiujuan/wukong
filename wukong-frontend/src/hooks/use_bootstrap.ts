import { useEffect } from 'react'
import { toast } from 'sonner'
import { useAppStore } from '@/store/use_app_store'

export function useBootstrap(enabled: boolean) {
  const loadSessions = useAppStore((state) => state.loadSessions)
  const loadTasks = useAppStore((state) => state.loadTasks)

  useEffect(() => {
    if (!enabled) {
      return
    }
    Promise.all([loadSessions(), loadTasks()]).catch((error: Error) => {
      toast.error(error.message)
    })
  }, [enabled, loadSessions, loadTasks])
}
