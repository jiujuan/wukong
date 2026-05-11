import { useEffect, useMemo, useState } from 'react'
import type { ChangeEvent, ComponentType } from 'react'
import { Brain, Cpu, Eye, ListTree, PencilLine, Save } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'
import { useAppStore } from '@/store/use_app_store'
import type { SkillItem } from '@/types/domain'

type ViewMode = 'skills' | 'tools'

export function SkillsPage() {
  const [viewMode, setViewMode] = useState<ViewMode>('skills')
  const navigate = useNavigate()
  const skills = useAppStore((state) => state.skills)
  const tools = useAppStore((state) => state.tools)
  const loadSkills = useAppStore((state) => state.loadSkills)
  const loadTools = useAppStore((state) => state.loadTools)

  useEffect(() => {
    loadSkills().catch((error: Error) => toast.error(error.message))
    loadTools().catch((error: Error) => toast.error(error.message))
  }, [loadSkills, loadTools])

  const activeList = viewMode === 'skills' ? skills : tools
  const metrics = useMemo(
    () => [
      { icon: Brain, label: 'Skills', value: String(skills.length) },
      { icon: Cpu, label: 'Tools', value: String(tools.length) },
      { icon: ListTree, label: 'Active', value: String(skills.filter((skill) => skill.enabled).length) },
    ],
    [skills, tools],
  )

  return (
    <div className="flex h-full flex-col gap-5">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-zinc-900">服务管理</h1>
          <p className="mt-1 text-sm text-zinc-500">在 skills 和 tools 之间切换查看</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant={viewMode === 'skills' ? 'default' : 'secondary'} className="gap-2" onClick={() => setViewMode('skills')}>
            <ListTree className="h-4 w-4" />
            Skills
          </Button>
          <Button variant={viewMode === 'tools' ? 'default' : 'secondary'} className="gap-2" onClick={() => setViewMode('tools')}>
            <Cpu className="h-4 w-4" />
            Tools
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-4">
        {metrics.map((metric) => (
          <SummaryCard key={metric.label} icon={metric.icon} label={metric.label} value={metric.value} />
        ))}
      </div>

      <Card className="min-h-0 flex-1 overflow-hidden">
        <div className="flex items-center justify-between border-b border-zinc-100 px-5 py-4">
          <div className="flex items-center gap-2 text-sm font-semibold text-zinc-900">
            {viewMode === 'skills' ? <ListTree className="h-4 w-4 text-indigo-500" /> : <Cpu className="h-4 w-4 text-indigo-500" />}
            {viewMode === 'skills' ? 'Skills 列表' : 'Tools 列表'}
          </div>
          <div className="text-xs text-zinc-400">{activeList.length} records</div>
        </div>
        <div className="overflow-auto">
          {viewMode === 'skills' ? <SkillsTable skills={skills} onOpen={navigate} /> : <ToolsTable tools={tools} />}
        </div>
      </Card>
    </div>
  )
}

export function SkillEditorPage() {
  const { skillName = '' } = useParams<{ skillName: string }>()
  const navigate = useNavigate()
  const loadSkills = useAppStore((state) => state.loadSkills)
  const [skill, setSkill] = useState<SkillItem | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    api
      .skillDetail(skillName)
      .then(setSkill)
      .catch((error: Error) => toast.error(error.message))
  }, [skillName])

  const updateField = (key: keyof SkillItem, value: string | boolean | number) => {
    setSkill((current) => (current ? { ...current, [key]: value } : current))
  }

  const submit = async () => {
    if (!skill) {
      return
    }
    setSaving(true)
    try {
      await api.updateSkill({
        skillName: skill.name,
        description: skill.description,
        version: skill.version,
        enabled: skill.enabled,
        memoryType: skill.memoryType,
        memoryWindow: skill.windowSize,
        memoryCompress: skill.memoryCompress,
      })
      await loadSkills()
      toast.success('Skill updated')
      navigate('/skills')
    } catch (error) {
      toast.error((error as Error).message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex h-full flex-col gap-5">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-zinc-900">Skill 编辑</h1>
          <p className="mt-1 text-sm text-zinc-500">查看并编辑 Skill 配置</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="secondary" onClick={() => navigate('/skills')}>
            返回列表
          </Button>
          <Button className="gap-2" onClick={submit} disabled={!skill || saving}>
            <Save className="h-4 w-4" />
            保存
          </Button>
        </div>
      </div>

      {!skill ? (
        <Card className="p-6 text-sm text-zinc-500">加载中...</Card>
      ) : (
        <div className="grid gap-4">
          <Card className="p-5">
            <div className="grid grid-cols-2 gap-4">
              <Field label="Skill Name">
                <Input value={skill.name} disabled />
              </Field>
              <Field label="Version">
                <Input value={skill.version} onChange={(event) => updateField('version', event.target.value)} />
              </Field>
              <Field label="Memory Type">
                <Input value={skill.memoryType ?? ''} onChange={(event) => updateField('memoryType', event.target.value)} />
              </Field>
              <Field label="Memory Window">
                <Input
                  value={String(skill.windowSize ?? 0)}
                  onChange={(event) => updateField('windowSize', Number(event.target.value || 0))}
                />
              </Field>
            </div>
          </Card>

          <Card className="p-5">
            <Field label="Description">
              <Textarea value={skill.description ?? ''} onChange={(event) => updateField('description', event.target.value)} className="min-h-[140px]" />
            </Field>
          </Card>

          <Card className="p-5">
            <div className="flex items-center gap-6">
              <label className="flex items-center gap-2 text-sm text-zinc-700">
                <input type="checkbox" checked={skill.enabled} onChange={(event: ChangeEvent<HTMLInputElement>) => updateField('enabled', event.target.checked)} />
                Enabled
              </label>
              <label className="flex items-center gap-2 text-sm text-zinc-700">
                <input type="checkbox" checked={Boolean(skill.memoryCompress)} onChange={(event: ChangeEvent<HTMLInputElement>) => updateField('memoryCompress', event.target.checked)} />
                Memory Compress
              </label>
            </div>
          </Card>
        </div>
      )}
    </div>
  )
}

function SkillsTable({
  skills,
  onOpen,
}: {
  skills: ReturnType<typeof useAppStore.getState>['skills']
  onOpen: (to: string) => void
}) {
  return (
    <table className="w-full border-collapse text-sm">
      <thead>
        <tr className="border-b border-zinc-100 bg-zinc-50 text-xs font-medium text-zinc-400">
          <th className="px-5 py-3 text-left">名称</th>
          <th className="px-5 py-3 text-left">版本</th>
          <th className="px-5 py-3 text-left">状态</th>
          <th className="px-5 py-3 text-left">记忆策略</th>
          <th className="px-5 py-3 text-left">操作</th>
        </tr>
      </thead>
      <tbody>
        {skills.length ? (
          skills.map((skill) => (
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
                <Badge variant={skill.enabled ? 'success' : 'outline'}>{skill.enabled ? '启用' : '禁用'}</Badge>
              </td>
              <td className="px-5 py-4 text-zinc-600">
                {skill.memoryType ?? '-'} {skill.windowSize ? <span className="text-xs text-zinc-400">window={skill.windowSize}</span> : null}
              </td>
              <td className="px-5 py-4">
                <div className="flex items-center gap-2">
                  <Button variant="secondary" size="sm" className="gap-1" onClick={() => onOpen(`/skills/${skill.name}`)}>
                    <Eye className="h-3.5 w-3.5" />
                    查看
                  </Button>
                  <Button variant="secondary" size="sm" className="gap-1" onClick={() => onOpen(`/skills/${skill.name}`)}>
                    <PencilLine className="h-3.5 w-3.5" />
                    编辑
                  </Button>
                </div>
              </td>
            </tr>
          ))
        ) : (
          <EmptyRow colSpan={5}>暂无 Skills</EmptyRow>
        )}
      </tbody>
    </table>
  )
}

function ToolsTable({ tools }: { tools: ReturnType<typeof useAppStore.getState>['tools'] }) {
  return (
    <table className="w-full border-collapse text-sm">
      <thead>
        <tr className="border-b border-zinc-100 bg-zinc-50 text-xs font-medium text-zinc-400">
          <th className="px-5 py-3 text-left">名称</th>
          <th className="px-5 py-3 text-left">描述</th>
        </tr>
      </thead>
      <tbody>
        {tools.length ? (
          tools.map((tool) => (
            <tr key={tool.name} className="border-b border-zinc-100 hover:bg-indigo-50/30">
              <td className="px-5 py-4">
                <div className="flex items-center gap-3">
                  <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-indigo-50 text-indigo-600">
                    <Cpu className="h-4 w-4" />
                  </div>
                  <div className="font-medium text-zinc-900">{tool.name}</div>
                </div>
              </td>
              <td className="px-5 py-4 text-zinc-600">{tool.description || '-'}</td>
            </tr>
          ))
        ) : (
          <EmptyRow colSpan={2}>暂无 Tools</EmptyRow>
        )}
      </tbody>
    </table>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="mb-2 text-sm font-medium text-zinc-600">{label}</div>
      {children}
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

function EmptyRow({ colSpan, children }: { colSpan: number; children: string }) {
  return (
    <tr>
      <td colSpan={colSpan} className="px-5 py-8 text-sm text-zinc-500">
        {children}
      </td>
    </tr>
  )
}
