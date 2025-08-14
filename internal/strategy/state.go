package strategy

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// AutomationState represents the persisted automation status
type AutomationState struct {
	PID        int              `json:"pid"`
	StartedAt  time.Time        `json:"started_at"`
	Active     bool             `json:"active"`
	Strategies []StrategyStatus `json:"strategies"`
}

// StrategyStatus represents a single strategy's status for monitoring
type StrategyStatus struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Symbols []string `json:"symbols"`
	Active  bool     `json:"active"`
}

const stateDirName = ".alpaca-tui"
const stateFileName = "automation_state.json"

func getStateFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, stateDirName, stateFileName), nil
}

// WriteAutomationState writes the current state to disk
func WriteAutomationState(state *AutomationState) error {
	path, err := getStateFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	return enc.Encode(state)
}

// ReadAutomationState reads the current state from disk
func ReadAutomationState() (*AutomationState, error) {
	path, err := getStateFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}
	var state AutomationState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// RemoveAutomationState deletes the persisted state
func RemoveAutomationState() error {
	path, err := getStateFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}
