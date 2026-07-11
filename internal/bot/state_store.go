package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const DefaultStateFileName = ".renkin_bot_state.json"

type StateStore struct {
	path string
}

func NewStateStore(path string) *StateStore {
	return &StateStore{path: path}
}

func DefaultStatePath(dir string) string {
	return filepath.Join(dir, DefaultStateFileName)
}

func (s *StateStore) Load() (State, error) {
	var state State
	data, err := os.ReadFile(s.path)
	if err == nil {
		if len(data) == 0 {
			return state, nil
		}
		if err := json.Unmarshal(data, &state); err != nil {
			return State{}, fmt.Errorf("load bot state %s: %w", s.path, err)
		}
		return state, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	return State{}, fmt.Errorf("load bot state %s: %w", s.path, err)
}

func (s *StateStore) Save(state State) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("create bot state dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode bot state: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".renkin_bot_state_*.tmp")
	if err != nil {
		return fmt.Errorf("create bot state temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write bot state temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close bot state temp file: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace bot state file: %w", err)
	}
	return nil
}
