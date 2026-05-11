import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { MemoryPage } from '@/features/memory/memory_page'
import type { TaskItem } from '@/types/domain'

type StoreState = {
  tasks: TaskItem[]
  loadTasks: ReturnType<typeof vi.fn>
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
  return render(
    <MemoryRouter>
      <MemoryPage />
    </MemoryRouter>,
  )
}

describe('MemoryPage', () => {
  beforeEach(() => {
    mockUseAppStore.mockReset()
    mockToastError.mockReset()
  })

  it('loads tasks and renders the memory task list on mount', async () => {
    const loadTasks = vi.fn().mockResolvedValue(undefined)
    renderWithState({
      tasks: [
        {
          taskId: 'task-1',
          title: 'Task One',
          status: 'RUNNING',
          skillName: 'planner',
        },
      ],
      loadTasks,
    })

    expect(screen.getAllByText('Task One').length).toBeGreaterThan(0)
    expect(screen.getByText('planner')).toBeInTheDocument()
    expect(screen.getByText('RUNNING')).toBeInTheDocument()

    await waitFor(() => {
      expect(loadTasks).toHaveBeenCalledTimes(1)
    })
  })

  it('renders an empty list when there are no tasks', async () => {
    const loadTasks = vi.fn().mockResolvedValue(undefined)

    renderWithState({
      tasks: [],
      loadTasks,
    })

    await waitFor(() => {
      expect(loadTasks).toHaveBeenCalledTimes(1)
    })
    expect(screen.getByText('0 records')).toBeInTheDocument()
  })

  it('navigates to the memory detail page when the user clicks a task row action', async () => {
    const user = userEvent.setup()
    const state = {
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
      loadTasks: vi.fn().mockResolvedValue(undefined),
    }
    mockUseAppStore.mockImplementation((selector: (input: StoreState) => unknown) => selector(state))

    render(
      <MemoryRouter initialEntries={['/memory']}>
        <Routes>
          <Route path="/memory" element={<MemoryPage />} />
          <Route path="/memory/:taskId" element={<div>detail route</div>} />
        </Routes>
      </MemoryRouter>,
    )

    await user.click(screen.getAllByRole('button')[1])

    expect(screen.getByText('detail route')).toBeInTheDocument()
  })
})
