import { cn } from '@/lib/utils'
import type { InputHTMLAttributes } from 'react'

export function Input({
  className,
  ...props
}: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={cn(
        'flex h-10 w-full rounded-lg border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-900 outline-none transition-colors placeholder:text-zinc-400 focus-visible:border-indigo-300 focus-visible:ring-4 focus-visible:ring-indigo-50',
        className,
      )}
      {...props}
    />
  )
}
