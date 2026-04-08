package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	dbpkg "github.com/jiujuan/wukong/pkg/database"
	"github.com/jiujuan/wukong/pkg/manager"
)

type TaskRepository struct {
	db *dbpkg.DB
}

func NewTaskRepository(db *dbpkg.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

// func (r *TaskRepository) EnsureTaskSubColumns(ctx context.Context) error {
// 	if r == nil || r.db == nil {
// 		return nil
// 	}
// 	if _, err := r.db.Exec(ctx, `ALTER TABLE task_sub ADD COLUMN IF NOT EXISTS result JSONB NULL`); err != nil {
// 		return err
// 	}
// 	if _, err := r.db.Exec(ctx, `ALTER TABLE task_sub ADD COLUMN IF NOT EXISTS error TEXT NULL`); err != nil {
// 		return err
// 	}
// 	return nil
// }

func (r *TaskRepository) CreateTask(ctx context.Context, task *manager.Task) error {
	paramsJSON, err := json.Marshal(task.Params)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO task_info (task_id, user_id, session_id, skill_name, params, status, priority, retry_count, max_retry, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err = r.db.Exec(ctx, query,
		task.TaskID, task.UserID, task.SessionID, task.SkillName,
		paramsJSON, task.Status, task.Priority, task.RetryCount, task.MaxRetry,
		task.CreatedAt, task.UpdatedAt,
	)
	return err
}

func (r *TaskRepository) GetTask(ctx context.Context, taskID string) (*manager.Task, error) {
	query := `
		SELECT task_id, user_id, session_id, skill_name, params, status, priority, retry_count, max_retry, created_at, updated_at, result, error
		FROM task_info WHERE task_id = $1 AND is_deleted = false
	`
	row := r.db.Pool().QueryRow(ctx, query, taskID)

	task := &manager.Task{}
	var paramsJSON, resultJSON []byte
	var sessionID, errMsg pgtype.Text

	err := row.Scan(
		&task.TaskID, &task.UserID, &sessionID, &task.SkillName,
		&paramsJSON, &task.Status, &task.Priority, &task.RetryCount, &task.MaxRetry,
		&task.CreatedAt, &task.UpdatedAt, &resultJSON, &errMsg,
	)
	if err != nil {
		return nil, err
	}

	if sessionID.Valid {
		task.SessionID = sessionID.String
	}
	if errMsg.Valid {
		task.Error = errMsg.String
	}
	if len(paramsJSON) > 0 {
		if err := json.Unmarshal(paramsJSON, &task.Params); err != nil {
			return nil, err
		}
	}
	if len(resultJSON) > 0 {
		if err := json.Unmarshal(resultJSON, &task.Result); err != nil {
			return nil, err
		}
	}

	return task, nil
}

func (r *TaskRepository) UpdateTask(ctx context.Context, task *manager.Task) error {
	paramsJSON, err := json.Marshal(task.Params)
	if err != nil {
		return err
	}
	resultJSON, err := json.Marshal(task.Result)
	if err != nil {
		return err
	}

	query := `
		UPDATE task_info
		SET user_id = $1, session_id = $2, skill_name = $3, params = $4, status = $5, priority = $6, retry_count = $7, max_retry = $8, updated_at = $9, result = $10, error = $11
		WHERE task_id = $12
	`
	_, err = r.db.Exec(ctx, query,
		task.UserID, task.SessionID, task.SkillName, paramsJSON, task.Status,
		task.Priority, task.RetryCount, task.MaxRetry, task.UpdatedAt,
		resultJSON, task.Error, task.TaskID,
	)
	return err
}

func (r *TaskRepository) BatchUpsertTasks(ctx context.Context, tasks []*manager.Task) error {
	if len(tasks) == 0 {
		return nil
	}
	values := make([]string, 0, len(tasks))
	args := make([]any, 0, len(tasks)*13)
	for i, task := range tasks {
		if task == nil {
			continue
		}
		paramsJSON, err := json.Marshal(task.Params)
		if err != nil {
			return err
		}
		resultJSON, err := json.Marshal(task.Result)
		if err != nil {
			return err
		}
		args = append(args,
			task.TaskID, task.UserID, task.SessionID, task.SkillName,
			paramsJSON, task.Status, task.Priority, task.RetryCount, task.MaxRetry,
			task.CreatedAt, task.UpdatedAt, resultJSON, task.Error,
		)
		values = append(values, placeholderTuple(i*13+1, 13))
	}
	if len(values) == 0 {
		return nil
	}
	query := `
		INSERT INTO task_info (
			task_id, user_id, session_id, skill_name, params, status, priority, retry_count, max_retry, created_at, updated_at, result, error
		) VALUES ` + strings.Join(values, ",") + `
		ON CONFLICT (task_id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			session_id = EXCLUDED.session_id,
			skill_name = EXCLUDED.skill_name,
			params = EXCLUDED.params,
			status = EXCLUDED.status,
			priority = EXCLUDED.priority,
			retry_count = EXCLUDED.retry_count,
			max_retry = EXCLUDED.max_retry,
			updated_at = EXCLUDED.updated_at,
			result = EXCLUDED.result,
			error = EXCLUDED.error
	`
	_, err := r.db.Exec(ctx, query, args...)
	return err
}

func (r *TaskRepository) ListTasks(ctx context.Context, userID, status string, page, size int) ([]*manager.Task, int64, error) {
	offset := (page - 1) * size

	conditions := []string{"user_id = $1", "is_deleted = false"}
	args := []any{userID}
	if status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)+1))
		args = append(args, status)
	}
	whereClause := strings.Join(conditions, " AND ")

	countQuery := "SELECT COUNT(*) FROM task_info WHERE " + whereClause
	listQuery := `
		SELECT task_id, user_id, session_id, skill_name, params, status, priority, retry_count, max_retry, created_at, updated_at, result, error
		FROM task_info
		WHERE ` + whereClause + `
		ORDER BY created_at DESC
		LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2)

	args = append(args, size, offset)

	var total int64
	if err := r.db.Pool().QueryRow(ctx, countQuery, args[:len(args)-2]...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	tasks := make([]*manager.Task, 0)
	for rows.Next() {
		task := &manager.Task{}
		var sessionID, errMsg pgtype.Text
		var paramsJSON, resultJSON []byte
		if err := rows.Scan(&task.TaskID, &task.UserID, &sessionID, &task.SkillName, &paramsJSON, &task.Status, &task.Priority, &task.RetryCount, &task.MaxRetry, &task.CreatedAt, &task.UpdatedAt, &resultJSON, &errMsg); err != nil {
			return nil, 0, err
		}
		if sessionID.Valid {
			task.SessionID = sessionID.String
		}
		if errMsg.Valid {
			task.Error = errMsg.String
		}
		if len(paramsJSON) > 0 {
			if err := json.Unmarshal(paramsJSON, &task.Params); err != nil {
				return nil, 0, err
			}
		}
		if len(resultJSON) > 0 {
			if err := json.Unmarshal(resultJSON, &task.Result); err != nil {
				return nil, 0, err
			}
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

func (r *TaskRepository) LoadPendingTasks(ctx context.Context) ([]*manager.Task, error) {
	query := `
		SELECT task_id, user_id, session_id, skill_name, params, status, priority, retry_count, max_retry, created_at, updated_at
		FROM task_info WHERE status IN ('PENDING', 'PLANNING', 'RUNNING', 'WAITING') AND is_deleted = false
		ORDER BY priority DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]*manager.Task, 0)
	for rows.Next() {
		task := &manager.Task{}
		var paramsJSON []byte
		var sessionID pgtype.Text
		if err := rows.Scan(&task.TaskID, &task.UserID, &sessionID, &task.SkillName, &paramsJSON, &task.Status, &task.Priority, &task.RetryCount, &task.MaxRetry, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, err
		}
		if sessionID.Valid {
			task.SessionID = sessionID.String
		}
		if len(paramsJSON) > 0 {
			if err := json.Unmarshal(paramsJSON, &task.Params); err != nil {
				return nil, err
			}
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (r *TaskRepository) CreateSubTask(ctx context.Context, subtask *manager.SubTask) error {
	dependsOnJSON, err := json.Marshal(subtask.DependsOn)
	if err != nil {
		return err
	}
	paramsJSON, err := json.Marshal(subtask.Params)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO task_sub (sub_task_id, task_id, depends_on, action, params, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err = r.db.Exec(ctx, query,
		subtask.SubTaskID, subtask.TaskID, dependsOnJSON, subtask.Action,
		paramsJSON, subtask.Status, subtask.CreatedAt, subtask.UpdatedAt,
	)
	return err
}

func (r *TaskRepository) GetSubTasks(ctx context.Context, taskID string) ([]*manager.SubTask, error) {
	query := `
		SELECT sub_task_id, task_id, depends_on, action, params, status, worker_id, created_at, updated_at
		FROM task_sub WHERE task_id = $1
		ORDER BY created_at
	`
	rows, err := r.db.Query(ctx, query, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subtasks := make([]*manager.SubTask, 0)
	for rows.Next() {
		st := &manager.SubTask{}
		var dependsOnJSON, paramsJSON []byte
		var workerID pgtype.Text
		if err := rows.Scan(&st.SubTaskID, &st.TaskID, &dependsOnJSON, &st.Action, &paramsJSON, &st.Status, &workerID, &st.CreatedAt, &st.UpdatedAt); err != nil {
			return nil, err
		}
		if workerID.Valid {
			st.WorkerID = workerID.String
		}
		if len(dependsOnJSON) > 0 {
			if err := json.Unmarshal(dependsOnJSON, &st.DependsOn); err != nil {
				return nil, err
			}
		}
		if len(paramsJSON) > 0 {
			if err := json.Unmarshal(paramsJSON, &st.Params); err != nil {
				return nil, err
			}
		}
		subtasks = append(subtasks, st)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return subtasks, nil
}

func (r *TaskRepository) UpdateSubTask(ctx context.Context, subtask *manager.SubTask) error {
	dependsOnJSON, err := json.Marshal(subtask.DependsOn)
	if err != nil {
		return err
	}
	paramsJSON, err := json.Marshal(subtask.Params)
	if err != nil {
		return err
	}
	resultJSON, err := json.Marshal(subtask.Result)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO task_sub (
			sub_task_id, task_id, depends_on, action, params, status, worker_id, created_at, updated_at, result, error
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (sub_task_id) DO UPDATE SET
			task_id = EXCLUDED.task_id,
			depends_on = EXCLUDED.depends_on,
			action = EXCLUDED.action,
			params = EXCLUDED.params,
			status = EXCLUDED.status,
			worker_id = EXCLUDED.worker_id,
			updated_at = EXCLUDED.updated_at,
			result = EXCLUDED.result,
			error = EXCLUDED.error
	`
	_, err = r.db.Exec(ctx, query,
		subtask.SubTaskID, subtask.TaskID, dependsOnJSON, subtask.Action, paramsJSON,
		subtask.Status, subtask.WorkerID, subtask.CreatedAt, subtask.UpdatedAt, resultJSON, subtask.Error,
	)
	return err
}

func (r *TaskRepository) CreateTaskExecLog(ctx context.Context, item *manager.TaskExecLog) error {
	query := `
		INSERT INTO task_exec_log (task_id, sub_task_id, log_type, content, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(ctx, query, item.TaskID, item.SubTaskID, item.LogType, item.Content, item.CreatedAt)
	return err
}

func (r *TaskRepository) BatchUpsertSubTasks(ctx context.Context, subtasks []*manager.SubTask) error {
	if len(subtasks) == 0 {
		return nil
	}
	values := make([]string, 0, len(subtasks))
	args := make([]any, 0, len(subtasks)*11)
	for i, subtask := range subtasks {
		if subtask == nil {
			continue
		}
		dependsOnJSON, err := json.Marshal(subtask.DependsOn)
		if err != nil {
			return err
		}
		paramsJSON, err := json.Marshal(subtask.Params)
		if err != nil {
			return err
		}
		resultJSON, err := json.Marshal(subtask.Result)
		if err != nil {
			return err
		}
		args = append(args,
			subtask.SubTaskID, subtask.TaskID, dependsOnJSON, subtask.Action, paramsJSON,
			subtask.Status, subtask.WorkerID, subtask.CreatedAt, subtask.UpdatedAt, resultJSON, subtask.Error,
		)
		values = append(values, placeholderTuple(i*11+1, 11))
	}
	if len(values) == 0 {
		return nil
	}
	query := `
		INSERT INTO task_sub (
			sub_task_id, task_id, depends_on, action, params, status, worker_id, created_at, updated_at, result, error
		) VALUES ` + strings.Join(values, ",") + `
		ON CONFLICT (sub_task_id) DO UPDATE SET
			task_id = EXCLUDED.task_id,
			depends_on = EXCLUDED.depends_on,
			action = EXCLUDED.action,
			params = EXCLUDED.params,
			status = EXCLUDED.status,
			worker_id = EXCLUDED.worker_id,
			updated_at = EXCLUDED.updated_at,
			result = EXCLUDED.result,
			error = EXCLUDED.error
	`
	_, err := r.db.Exec(ctx, query, args...)
	return err
}

func placeholderTuple(start int, n int) string {
	holders := make([]string, n)
	for i := 0; i < n; i++ {
		holders[i] = fmt.Sprintf("$%d", start+i)
	}
	return "(" + strings.Join(holders, ", ") + ")"
}
