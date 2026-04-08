import { cn } from '@/lib/utils'
import type { TextareaHTMLAttributes } from 'react'

export function Textarea({
  className,
  ...props
}: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      className={cn(
        'flex min-h-[80px] w-full rounded-md border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 outline-none placeholder:text-zinc-500 focus-visible:border-zinc-500',
        className,
      )}
      {...props}
    />
  )
}
