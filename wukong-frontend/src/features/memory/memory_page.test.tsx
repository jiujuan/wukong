import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryPage } from '@/features/memory/memory_page'
import type { LongMemory, TaskItem, WorkingMemory } from '@/types/domain'

type StoreState = {
  tasks: TaskItem[]
  currentTaskId: string
  workingMemory: WorkingMemory[]
  longMemory: LongMemory[]
  setCurrentTask: ReturnType<typeof vi.fn>
  loadTasks: ReturnType<typeof vi.fn>
  loadMemory: ReturnType<typeof vi.fn>
}

const mockUseAppStore = vi.fn()
const mockToastError = vi.fn()

vi.mock('@/store/use_app_store', () => ({
  useAppStore: (selector: (state: StoreState) => unknown) => mockUseAppStore(selector),
}))

vi.mock('sonner', () => ({
  toast: {
    error: (...args: unknown[]) => mockToastError(...args),
  },
}))

function renderWithState(state: StoreState) {
  mockUseAppStore.mockImplementation((selector: (input: StoreState) => unknown) => selector(state))
  return render(<MemoryPage />)
}

describe('MemoryPage', () => {
  beforeEach(() => {
    mockUseAppStore.mockReset()
    mockToastError.mockReset()
  })

  it('loads tasks and current memory on mount', async () => {
    const loadTasks = vi.fn().mockResolvedValue(undefined)
    const loadMemory = vi.fn().mockResolvedValue(undefined)
    renderWithState({
      tasks: [
        {
          taskId: 'task-1',
          title: 'Task One',
          status: 'RUNNING',
          skillName: 'planner',
        },
      ],
      currentTaskId: 'task-1',
      workingMemory: [{ id: 'wm-1', content: 'short note', createdAt: 'today' }],
      longMemory: [{ id: 'lm-1', topic: 'topic-a', content: 'archived note' }],
      setCurrentTask: vi.fn(),
      loadTasks,
      loadMemory,
    })

    expect(screen.getAllByText('Task One').length).toBeGreaterThan(0)
    expect(screen.getByText('short note')).toBeInTheDocument()
    expect(screen.getByText('archived note')).toBeInTheDocument()

    await waitFor(() => {
      expect(loadTasks).toHaveBeenCalledTimes(1)
      expect(loadMemory).toHaveBeenCalledWith('task-1', 'planner')
    })
  })

  it('selects the first task when none is active', async () => {
    const setCurrentTask = vi.fn()
    const loadTasks = vi.fn().mockResolvedValue(undefined)
    const loadMemory = vi.fn().mockResolvedValue(undefined)

    renderWithState({
      tasks: [
        {
          taskId: 'task-1',
          title: 'Task One',
          status: 'PENDING',
          skillName: 'planner',
        },
      ],
      currentTaskId: '',
      workingMemory: [],
      longMemory: [],
      setCurrentTask,
      loadTasks,
      loadMemory,
    })

    await waitFor(() => {
      expect(setCurrentTask).toHaveBeenCalledWith('task-1')
    })
    expect(loadMemory).not.toHaveBeenCalled()
  })

  it('changes task when the user clicks another task row', async () => {
    const user = userEvent.setup()
    const setCurrentTask = vi.fn()

    renderWithState({
      tasks: [
        {
          taskId: 'task-1',
          title: 'Task One',
          status: 'RUNNING',
          skillName: 'planner',
        },
        {
          taskId: 'task-2',
          title: 'Task Two',
          status: 'COMPLETED',
          skillName: 'writer',
        },
      ],
      currentTaskId: 'task-1',
      workingMemory: [],
      longMemory: [],
      setCurrentTask,
      loadTasks: vi.fn().mockResolvedValue(undefined),
      loadMemory: vi.fn().mockResolvedValue(undefined),
    })

    await user.click(screen.getByRole('button', { name: /Task Two/i }))

    expect(setCurrentTask).toHaveBeenCalledWith('task-2')
  })
})
