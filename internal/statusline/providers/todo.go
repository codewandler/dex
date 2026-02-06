package providers

import (
	"context"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/models"
	"github.com/codewandler/dex/internal/todo"
)

// TodoProvider fetches local todo counts
type TodoProvider struct{}

func NewTodoProvider() *TodoProvider {
	return &TodoProvider{}
}

func (p *TodoProvider) Name() string {
	return "todo"
}

func (p *TodoProvider) IsConfigured(_ *config.Config) bool {
	return true // Local file, no credentials needed
}

func (p *TodoProvider) Fetch(_ context.Context) (map[string]any, error) {
	store, err := todo.Load()
	if err != nil {
		return nil, err
	}

	pending, inProgress, onHold := 0, 0, 0
	for _, t := range store.Todos {
		switch t.State {
		case models.TodoStatePending:
			pending++
		case models.TodoStateInProgress:
			inProgress++
		case models.TodoStateOnHold:
			onHold++
		}
	}

	total := pending + inProgress + onHold

	return map[string]any{
		"Total":      total,
		"Pending":    pending,
		"InProgress": inProgress,
		"OnHold":     onHold,
	}, nil
}
