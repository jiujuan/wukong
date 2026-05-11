import { cn } from '@/lib/utils'
import type { HTMLAttributes } from 'react'

export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn('rounded-xl border border-zinc-200 bg-white shadow-[0_10px_30px_rgba(31,36,48,0.04)]', className)}
      {...props}
    />
  )
}
