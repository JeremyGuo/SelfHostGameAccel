package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type persistentState struct {
	Users      map[string]userRecord  `json:"users"`
	DeviceBags map[string]string      `json:"device_bags"`
	Rooms      map[string]*roomRecord `json:"rooms"`
}

func loadState(path string) (persistentState, error) {
	state := persistentState{
		Users:      map[string]userRecord{},
		DeviceBags: map[string]string{},
		Rooms:      map[string]*roomRecord{},
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("read state: %w", err)
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("decode state: %w", err)
	}
	if state.Users == nil {
		state.Users = map[string]userRecord{}
	}
	if state.DeviceBags == nil {
		state.DeviceBags = map[string]string{}
	}
	if state.Rooms == nil {
		state.Rooms = map[string]*roomRecord{}
	}
	return state, nil
}

func saveState(path string, state persistentState) error {
	tmp := path + ".tmp"
	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	if err := os.WriteFile(tmp, encoded, 0o600); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("move state into place: %w", err)
	}
	return nil
}
