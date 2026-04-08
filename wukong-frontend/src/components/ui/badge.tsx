import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'
import type { HTMLAttributes } from 'react'

const badgeVariants = cva(
  'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium',
  {
    variants: {
      variant: {
        default: 'bg-zinc-200 text-zinc-900',
        outline: 'border border-zinc-700 text-zinc-300',
        success: 'bg-emerald-700/30 text-emerald-300 border border-emerald-700',
        warning: 'bg-yellow-700/30 text-yellow-300 border border-yellow-700',
        danger: 'bg-red-700/30 text-red-300 border border-red-700',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
)

type BadgeProps = HTMLAttributes<HTMLDivElement> & VariantProps<typeof badgeVariants>

export function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />
}
