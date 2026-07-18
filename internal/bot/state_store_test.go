package bot

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateStoreLoadMissingFileReturnsDefaultState(t *testing.T) {
	store := NewStateStore(filepath.Join(t.TempDir(), DefaultStateFileName))

	state, err := store.Load()

	require.NoError(t, err)
	assert.Empty(t, state.Checkpoint.LastMessageID)
	assert.Nil(t, state.Dispatch)
}

func TestStateStoreSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultStateFileName)
	store := NewStateStore(path)
	deadline := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	lastCheckedAt := time.Date(2026, 7, 18, 5, 0, 0, 0, time.UTC)

	expected := State{
		Checkpoint: Checkpoint{LastMessageID: "123", LastCheckedAt: lastCheckedAt},
		Dispatch: &DispatchRecord{
			BoardItemID:  "123",
			Attempt:      1,
			MaxRetries:   3,
			RestartDelay: 5 * time.Second,
			Deadline:     deadline,
			State:        DispatchStateInFlight,
		},
	}

	require.NoError(t, store.Save(expected))
	actual, err := store.Load()

	require.NoError(t, err)
	assert.Equal(t, expected.Checkpoint, actual.Checkpoint)
	require.NotNil(t, actual.Dispatch)
	assert.Equal(t, expected.Dispatch.BoardItemID, actual.Dispatch.BoardItemID)
	assert.Equal(t, expected.Dispatch.Attempt, actual.Dispatch.Attempt)
	assert.Equal(t, expected.Dispatch.MaxRetries, actual.Dispatch.MaxRetries)
	assert.Equal(t, expected.Dispatch.RestartDelay, actual.Dispatch.RestartDelay)
	assert.Equal(t, expected.Dispatch.Deadline, actual.Dispatch.Deadline)
	assert.Equal(t, expected.Dispatch.State, actual.Dispatch.State)
}

func TestStateStoreLoadCorruptFileReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultStateFileName)
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0644))
	store := NewStateStore(path)

	_, err := store.Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "load bot state")
}

func TestDefaultStatePath(t *testing.T) {
	assert.Equal(t, filepath.Join("/tmp/project", DefaultStateFileName), DefaultStatePath("/tmp/project"))
}
