import { cn } from '@/lib/utils'
import type { ReactNode } from 'react'

type SheetProps = {
  open: boolean
  onClose: () => void
  title: string
  children: ReactNode
}

export function Sheet({ open, onClose, title, children }: SheetProps) {
  if (!open) {
    return null
  }

  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-black/20">
      <button className="h-full flex-1 cursor-default" onClick={onClose} />
      <div
        className={cn(
          'h-full w-full max-w-xl border-l border-zinc-200 bg-zinc-100 p-5 text-zinc-900',
        )}
      >
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-lg font-semibold">{title}</h3>
          <button
            className="rounded-md border border-zinc-300 px-2 py-1 text-sm hover:bg-zinc-200"
            onClick={onClose}
          >
            关闭
          </button>
        </div>
        <div className="h-[calc(100%-44px)] overflow-auto">{children}</div>
      </div>
    </div>
  )
}
