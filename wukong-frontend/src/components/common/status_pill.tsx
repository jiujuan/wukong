import {
  AlertCircle,
  Ban,
  CheckCircle2,
  Clock3,
  Loader2,
  type LucideIcon,
} from 'lucide-react'
import type { TaskStatus } from '@/types/domain'

type StatusPillProps = {
  status: TaskStatus | string
  labelOverride?: string
  completedStyle?: boolean
}

export function StatusPill({ status, labelOverride, completedStyle = false }: StatusPillProps) {
  const meta = STATUS_META[status] ?? STATUS_META.UNKNOWN
  const Icon = completedStyle ? CheckCircle2 : meta.icon
  const className = completedStyle
    ? 'border-emerald-300 bg-emerald-50 text-emerald-700'
    : meta.className
  const spin = completedStyle ? false : meta.spin
  const label = labelOverride ?? meta.label
  return (
    <div className={`inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs font-medium ${className}`}>
      <Icon className={`h-3.5 w-3.5 ${spin ? 'animate-spin' : ''}`} />
      <span>{label}</span>
    </div>
  )
}

const STATUS_META: Record<
  string,
  { label: string; className: string; icon: LucideIcon; spin?: boolean }
> = {
  PENDING: {
    label: '待执行',
    className: 'border-zinc-300 bg-zinc-100 text-zinc-700',
    icon: Clock3,
  },
  PLANNING: {
    label: '规划中',
    className: 'border-sky-300 bg-sky-50 text-sky-700',
    icon: Loader2,
    spin: true,
  },
  RUNNING: {
    label: '运行中',
    className: 'border-emerald-300 bg-emerald-50 text-emerald-700',
    icon: Loader2,
    spin: true,
  },
  WAITING: {
    label: '等待中',
    className: 'border-amber-300 bg-amber-50 text-amber-700',
    icon: Clock3,
  },
  COMPLETED: {
    label: '已完成',
    className: 'border-emerald-300 bg-emerald-50 text-emerald-700',
    icon: CheckCircle2,
  },
  FAILED: {
    label: '失败',
    className: 'border-red-300 bg-red-50 text-red-700',
    icon: AlertCircle,
  },
  CANCELLED: {
    label: '已取消',
    className: 'border-rose-300 bg-rose-50 text-rose-700',
    icon: Ban,
  },
  UNKNOWN: {
    label: '未知状态',
    className: 'border-zinc-300 bg-zinc-100 text-zinc-700',
    icon: AlertCircle,
  },
}
