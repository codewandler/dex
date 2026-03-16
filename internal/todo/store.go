package todo

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

const idAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

func storeFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dex", "todos.json"), nil
}

func Load() (*TodoStore, error) {
	path, err := storeFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewTodoStore(), nil
		}
		return nil, err
	}

	var store TodoStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}

	store.BuildLookupMaps()
	return &store, nil
}

func Save(store *TodoStore) error {
	path, err := storeFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func CreateTodo(title, description string) Todo {
	id, _ := gonanoid.Generate(idAlphabet, 4)
	now := time.Now()
	return Todo{
		ID:          id,
		Title:       title,
		Description: description,
		State:       TodoStatePending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func CreateReference(refType, value string) Reference {
	id, _ := gonanoid.Generate(idAlphabet, 4)
	return Reference{
		ID:    id,
		Type:  refType,
		Value: value,
	}
}
