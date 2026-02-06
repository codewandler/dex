package models

import "time"

type TodoState string

const (
	TodoStatePending    TodoState = "pending"
	TodoStateInProgress TodoState = "in_progress"
	TodoStateOnHold     TodoState = "on_hold"
	TodoStateDone       TodoState = "done"
)

func IsValidTodoState(s string) bool {
	switch TodoState(s) {
	case TodoStatePending, TodoStateInProgress, TodoStateOnHold, TodoStateDone:
		return true
	}
	return false
}

type Reference struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Todo struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description,omitempty"`
	State       TodoState   `json:"state"`
	References  []Reference `json:"references,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

func (t *Todo) RemoveReference(refID string) bool {
	for i, ref := range t.References {
		if ref.ID == refID {
			t.References = append(t.References[:i], t.References[i+1:]...)
			return true
		}
	}
	return false
}

type TodoStore struct {
	Version   int            `json:"version"`
	Todos     []Todo         `json:"todos"`
	TodosByID map[string]int `json:"-"`
}

func NewTodoStore() *TodoStore {
	return &TodoStore{
		Version:   1,
		Todos:     []Todo{},
		TodosByID: make(map[string]int),
	}
}

func (s *TodoStore) BuildLookupMaps() {
	s.TodosByID = make(map[string]int, len(s.Todos))
	for i, t := range s.Todos {
		s.TodosByID[t.ID] = i
	}
}

func (s *TodoStore) FindTodo(id string) *Todo {
	if idx, ok := s.TodosByID[id]; ok && idx < len(s.Todos) {
		return &s.Todos[idx]
	}
	return nil
}

func (s *TodoStore) AddTodo(t Todo) {
	s.Todos = append(s.Todos, t)
	s.TodosByID[t.ID] = len(s.Todos) - 1
}
