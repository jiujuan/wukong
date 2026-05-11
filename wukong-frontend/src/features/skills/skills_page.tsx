import { useEffect } from 'react'
import type { ComponentType } from 'react'
import { Brain, CheckCircle2, Database, SlidersHorizontal } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { useAppStore } from '@/store/use_app_store'

export function SkillsPage() {
  const skills = useAppStore((state) => state.skills)
  const loadSkills = useAppStore((state) => state.loadSkills)

  useEffect(() => {
    loadSkills().catch((error: Error) => toast.error(error.message))
  }, [loadSkills])

  return (
    <div className="flex h-full flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold text-zinc-900">服务管理</h1>
        <p className="mt-1 text-sm text-zinc-500">查看当前可用技能、版本和记忆策略</p>
      </div>

      <div className="grid grid-cols-3 gap-4">
        <SummaryCard icon={Brain} label="技能数量" value={String(skills.length)} />
        <SummaryCard
          icon={CheckCircle2}
          label="已启用"
          value={String(skills.filter((skill) => skill.enabled).length)}
        />
        <SummaryCard
          icon={Database}
          label="记忆策略"
          value={String(new Set(skills.map((skill) => skill.memoryType ?? 'working')).size)}
        />
      </div>

      <Card className="min-h-0 flex-1 overflow-hidden">
        <div className="flex items-center justify-between border-b border-zinc-100 px-5 py-4">
          <div className="flex items-center gap-2 text-sm font-semibold text-zinc-900">
            <SlidersHorizontal className="h-4 w-4 text-indigo-500" />
            技能列表
          </div>
          <div className="text-xs text-zinc-400">{skills.length} records</div>
        </div>
        <div className="overflow-auto">
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr className="border-b border-zinc-100 bg-zinc-50 text-xs font-medium text-zinc-400">
                <th className="px-5 py-3 text-left">技能</th>
                <th className="px-5 py-3 text-left">版本</th>
                <th className="px-5 py-3 text-left">状态</th>
                <th className="px-5 py-3 text-left">记忆策略</th>
              </tr>
            </thead>
            <tbody>
              {skills.map((skill) => (
                <tr key={skill.name} className="border-b border-zinc-100 hover:bg-indigo-50/30">
                  <td className="px-5 py-4">
                    <div className="flex items-center gap-3">
                      <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-indigo-50 text-indigo-600">
                        <Brain className="h-4 w-4" />
                      </div>
                      <div>
                        <div className="font-medium text-zinc-900">{skill.name}</div>
                        <div className="text-xs text-zinc-400">Skill service</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-5 py-4 text-zinc-600">{skill.version}</td>
                  <td className="px-5 py-4">
                    <Badge variant={skill.enabled ? 'success' : 'outline'}>
                      {skill.enabled ? '启用' : '禁用'}
                    </Badge>
                  </td>
                  <td className="px-5 py-4 text-zinc-600">
                    {skill.memoryType ?? '-'}{' '}
                    {skill.windowSize ? (
                      <span className="text-xs text-zinc-400">window={skill.windowSize}</span>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  )
}

function SummaryCard({
  icon: Icon,
  label,
  value,
}: {
  icon: ComponentType<{ className?: string }>
  label: string
  value: string
}) {
  return (
    <Card className="p-4">
      <div className="flex items-center justify-between">
        <div>
          <div className="text-sm text-zinc-500">{label}</div>
          <div className="mt-2 text-2xl font-semibold text-zinc-900">{value}</div>
        </div>
        <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-indigo-50 text-indigo-600">
          <Icon className="h-5 w-5" />
        </div>
      </div>
    </Card>
  )
}
