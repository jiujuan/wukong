import { useEffect } from 'react'
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
    <Card className="h-full p-4">
      <div className="mb-4 text-lg font-semibold">技能列表</div>
      <div className="rounded-md border border-zinc-800">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr className="border-b border-zinc-800 bg-zinc-900">
              <th className="px-3 py-2 text-left">技能</th>
              <th className="px-3 py-2 text-left">版本</th>
              <th className="px-3 py-2 text-left">状态</th>
              <th className="px-3 py-2 text-left">记忆策略</th>
            </tr>
          </thead>
          <tbody>
            {skills.map((skill) => (
              <tr key={skill.name} className="border-b border-zinc-900">
                <td className="px-3 py-2">{skill.name}</td>
                <td className="px-3 py-2">{skill.version}</td>
                <td className="px-3 py-2">
                  <Badge variant={skill.enabled ? 'success' : 'outline'}>
                    {skill.enabled ? '启用' : '禁用'}
                  </Badge>
                </td>
                <td className="px-3 py-2">
                  {skill.memoryType ?? '-'} {skill.windowSize ? `(window=${skill.windowSize})` : ''}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  )
}
