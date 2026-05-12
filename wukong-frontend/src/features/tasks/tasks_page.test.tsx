import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { TasksPage } from './tasks_page'
import { useAppStore } from '@/store/use_app_store'

const apiMocks = vi.hoisted(() => ({
  listTasks: vi.fn(),
  listSkills: vi.fn(),
  taskDetail: vi.fn(),
}))

const sseMocks = vi.hoisted(() => ({
  createSSE: vi.fn(),
}))

vi.mock('@/lib/api', () => ({
  api: {
    listTasks: apiMocks.listTasks,
    listSkills: apiMocks.listSkills,
    taskDetail: apiMocks.taskDetail,
    cancelTask: vi.fn(),
  },
  streamUrl: (path: string, params: Record<string, string>) =>
    `${path}?${new URLSearchParams(params).toString()}`,
}))

vi.mock('@/lib/sse', () => ({
  createSSE: sseMocks.createSSE,
}))

vi.mock('sonner', () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
    warning: vi.fn(),
  },
}))

describe('TasksPage', () => {
  beforeEach(() => {
    apiMocks.listTasks.mockResolvedValue({ list: [], total: 0, pages: 1 })
    apiMocks.listSkills.mockResolvedValue([
      { name: 'echo_skill', version: '1.0.0', enabled: true },
    ])
    apiMocks.taskDetail.mockResolvedValue({
      task: {
        taskId: 'task-1',
        title: 'Echo skill task',
        skillName: 'echo_skill',
        status: 'COMPLETED',
        result: {
          _execution_type: 'third_party_skill',
          _execution: {
            execution_type: 'third_party_skill',
            skill_name: 'echo_skill',
            source_type: 'vendor',
            entry: 'run.sh',
            skill_root: '/skills/vendor/echo_skill',
            output_dir: 'storage/output_data/echo_skill',
            stdout: 'skill=echo_skill',
            stderr: '',
            output: 'skill=echo_skill',
            exit_code: 0,
            package: {
              manifest_path: '/skills/vendor/echo_skill/wukong.skill.json',
              source_type: 'vendor',
              entry: 'run.sh',
            },
          },
        },
      },
      subTasks: [
        {
          subTaskId: 'sub-1',
          title: 'echo_skill',
          action: 'echo_skill',
          status: 'SUCCESS',
          result: {
            execution_type: 'third_party_skill',
            skill_name: 'echo_skill',
            source_type: 'vendor',
            entry: 'run.sh',
            skill_root: '/skills/vendor/echo_skill',
            output_dir: 'storage/output_data/echo_skill',
            stdout: 'skill=echo_skill',
            stderr: '',
            output: 'skill=echo_skill',
            exit_code: 0,
          },
          dependsOn: [],
        },
      ],
    })
    sseMocks.createSSE.mockReturnValue(vi.fn())
    useAppStore.setState({
      sessions: [],
      currentSessionId: '',
      messagesBySession: {},
      tasks: [],
      currentTaskId: '',
      eventsByTask: {},
      workingMemory: [],
      longMemory: [],
      skills: [],
      tools: [],
      memoryOpen: false,
    })
  })

  it('renders a dedicated panel for third-party skill task results', async () => {
    render(
      <MemoryRouter initialEntries={['/tasks/task-1']}>
        <Routes>
          <Route path="/tasks/:taskId" element={<TasksPage />} />
        </Routes>
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(apiMocks.taskDetail).toHaveBeenCalledWith('task-1')
    })

    expect(await screen.findByText('第三方 Skill 执行结果')).toBeInTheDocument()
    expect(screen.getAllByText('echo_skill').length).toBeGreaterThan(0)
    expect(screen.getByText('vendor')).toBeInTheDocument()
    expect(screen.getByText('run.sh')).toBeInTheDocument()
    expect(screen.getAllByText('skill=echo_skill').length).toBeGreaterThanOrEqual(2)
  })
})
