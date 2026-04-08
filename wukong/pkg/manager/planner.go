package manager

import (
	"context"
)

type TaskPlanner interface {
	Name() string
	PlanSubTasks(ctx context.Context, task *Task) ([]SubTaskDef, error)
}

type PlannerReport func(msgType string, content string)

type plannerReportKey struct{}

func WithPlanReporter(ctx context.Context, reporter PlannerReport) context.Context {
	return context.WithValue(ctx, plannerReportKey{}, reporter)
}

func reportPlan(ctx context.Context, msgType string, content string) {
	if ctx == nil {
		return
	}
	reporter, ok := ctx.Value(plannerReportKey{}).(PlannerReport)
	if !ok || reporter == nil {
		return
	}
	reporter(msgType, content)
}

type TaskTemplate struct {
	Name  string
	Steps []TemplateStep
}

type TemplateStep struct {
	Action    string
	Params    map[string]any
	DependsOn []int
}

type SubTaskDef struct {
	SubTaskID string
	TaskID    string
	Action    string
	Params    map[string]any
	DependsOn []string
	Status    string
}
