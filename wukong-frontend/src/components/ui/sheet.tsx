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
    <div className="fixed inset-0 z-50 flex justify-end bg-slate-950/20 backdrop-blur-[2px]">
      <button className="h-full flex-1 cursor-default" onClick={onClose} />
      <div
        className={cn(
          'h-full w-full max-w-xl border-l border-zinc-200 bg-white p-5 text-zinc-900 shadow-2xl',
        )}
      >
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-base font-semibold text-zinc-900">{title}</h3>
          <button
            className="rounded-lg border border-zinc-200 px-3 py-1.5 text-sm text-zinc-600 hover:bg-zinc-50"
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
