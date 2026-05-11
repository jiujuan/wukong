import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { MemoryPage } from '@/features/memory/memory_page'

const apiMocks = vi.hoisted(() => ({
  listTasksPage: vi.fn(),
}))

const mockToastError = vi.fn()
const mockNavigate = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

vi.mock('@/lib/api', () => ({
  api: {
    listTasksPage: apiMocks.listTasksPage,
  },
}))

vi.mock('sonner', () => ({
  toast: {
    error: (...args: unknown[]) => mockToastError(...args),
  },
}))

describe('MemoryPage', () => {
  beforeEach(() => {
    apiMocks.listTasksPage.mockReset()
    mockToastError.mockReset()
    mockNavigate.mockReset()
  })

  it('loads tasks and renders the memory task list on mount', async () => {
    apiMocks.listTasksPage.mockResolvedValue({
      list: [
        {
          taskId: 'task-1',
          title: 'Task One',
          status: 'RUNNING',
          skillName: 'planner',
        },
      ],
      total: 1,
      pages: 1,
    })

    render(
      <MemoryRouter>
        <MemoryPage />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(apiMocks.listTasksPage).toHaveBeenCalledWith({ page: 1, size: 20 })
    })

    expect(await screen.findByText('Task One')).toBeInTheDocument()
    expect(screen.getByText('planner')).toBeInTheDocument()
    expect(screen.getByText('RUNNING')).toBeInTheDocument()
  })

  it('renders an empty list when there are no tasks', async () => {
    apiMocks.listTasksPage.mockResolvedValue({
      list: [],
      total: 0,
      pages: 1,
    })

    render(
      <MemoryRouter>
        <MemoryPage />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(apiMocks.listTasksPage).toHaveBeenCalledTimes(1)
    })

    expect(await screen.findByText('0 records')).toBeInTheDocument()
    expect(screen.getByText('暂无任务记录')).toBeInTheDocument()
  })

  it('navigates to the memory detail page when the user clicks a task row action', async () => {
    const user = userEvent.setup()
    apiMocks.listTasksPage.mockResolvedValue({
      list: [
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
      total: 2,
      pages: 1,
    })

    render(
      <MemoryRouter initialEntries={['/memory']}>
        <MemoryPage />
      </MemoryRouter>,
    )

    const detailButtons = await screen.findAllByRole('button', { name: /详情/i })
    await user.click(detailButtons[0])

    expect(mockNavigate).toHaveBeenCalledWith('/memory/task-1')
  })
})
