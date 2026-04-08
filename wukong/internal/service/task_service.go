package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jiujuan/wukong/internal/repository"
	"github.com/jiujuan/wukong/pkg/manager"
	"github.com/jiujuan/wukong/pkg/statemachine"
)

// TaskService 任务服务
type TaskService struct {
	mgr  *manager.Manager
	repo *repository.TaskRepository
}

// NewTaskService 创建任务服务
func NewTaskService(mgr *manager.Manager, repo *repository.TaskRepository) *TaskService {
	return &TaskService{mgr: mgr, repo: repo}
}

// CreateTask 创建任务
func (s *TaskService) CreateTask(ctx context.Context, userID, sessionID, skillName string, params map[string]any, priority int) (*manager.Task, error) {
	if priority < 1 {
		priority = 5
	}
	if priority > 10 {
		priority = 10
	}
	if s.mgr == nil {
		return nil, fmt.Errorf("task manager not initialized")
	}
	task, err := s.mgr.CreateTask(ctx, userID, sessionID, skillName, params, priority)
	if err != nil {
		return nil, err
	}
	if s.repo != nil {
		if err := s.persistTask(ctx, task); err != nil {
			return nil, err
		}
	}
	return task, nil
}

// GetTask 获取任务
func (s *TaskService) GetTask(ctx context.Context, taskID string) (*manager.Task, error) {
	if s.mgr != nil {
		task, err := s.mgr.GetTask(ctx, taskID)
		if err != nil {
			return nil, err
		}
		if task != nil {
			return task, nil
		}
	}
	if s.repo == nil {
		return nil, nil
	}
	return s.repo.GetTask(ctx, taskID)
}

// ListTasks 列出任务
func (s *TaskService) ListTasks(ctx context.Context, userID, status string, page, size int) ([]*manager.Task, int64, error) {
	if s.repo == nil {
		return []*manager.Task{}, 0, nil
	}
	return s.repo.ListTasks(ctx, userID, status, page, size)
}

// CancelTask 取消任务
func (s *TaskService) CancelTask(ctx context.Context, taskID string) error {
	if s.mgr == nil {
		return fmt.Errorf("task manager not initialized")
	}
	if err := s.mgr.CancelTask(ctx, taskID); err != nil {
		return err
	}
	if s.repo == nil {
		return nil
	}
	task, err := s.mgr.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		task, err = s.repo.GetTask(ctx, taskID)
		if err != nil {
			return err
		}
	}
	task.Status = statemachine.StatusCancelled
	task.UpdatedAt = time.Now()
	return s.repo.UpdateTask(ctx, task)
}

func (s *TaskService) InjectTaskInstruction(ctx context.Context, taskID string, content string) error {
	if s.mgr == nil {
		return fmt.Errorf("task manager not initialized")
	}
	return s.mgr.InjectTaskInstruction(ctx, taskID, content)
}

// GetTaskState 获取任务状态
func (s *TaskService) GetTaskState(taskID string) string {
	if s.mgr == nil {
		return ""
	}
	return s.mgr.GetTaskState(taskID)
}

// GetSubTasks 获取子任务列表
func (s *TaskService) GetSubTasks(ctx context.Context, taskID string) ([]*manager.SubTask, error) {
	if s.mgr != nil {
		subtasks, err := s.mgr.GetSubTasks(ctx, taskID)
		if err != nil {
			return nil, err
		}
		if len(subtasks) > 0 {
			return subtasks, nil
		}
	}
	if s.repo == nil {
		return []*manager.SubTask{}, nil
	}
	return s.repo.GetSubTasks(ctx, taskID)
}

// UpdateTaskResult 更新任务结果
func (s *TaskService) UpdateTaskResult(ctx context.Context, taskID string, result map[string]any) error {
	task, err := s.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Result = result
	task.Status = statemachine.StatusCompleted
	task.Error = ""
	task.UpdatedAt = time.Now()

	if s.repo != nil {
		return s.repo.UpdateTask(ctx, task)
	}
	return s.persistTask(ctx, task)
}

// UpdateTaskError 更新任务错误
func (s *TaskService) UpdateTaskError(ctx context.Context, taskID, errMsg string) error {
	task, err := s.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Error = errMsg
	task.Status = statemachine.StatusFailed
	task.UpdatedAt = time.Now()

	if s.repo != nil {
		return s.repo.UpdateTask(ctx, task)
	}
	return s.persistTask(ctx, task)
}

func (s *TaskService) persistTask(ctx context.Context, task *manager.Task) error {
	if s.repo == nil || task == nil {
		return nil
	}
	_, err := s.repo.GetTask(ctx, task.TaskID)
	if err == nil {
		return s.repo.UpdateTask(ctx, task)
	}
	if err == pgx.ErrNoRows {
		return s.repo.CreateTask(ctx, task)
	}
	return err
}
