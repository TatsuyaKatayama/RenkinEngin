package bot

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingRunner struct {
	items []BoardItem
}

func (r *recordingRunner) Run(_ context.Context, item BoardItem) error {
	r.items = append(r.items, item)
	return nil
}

func TestDispatcherTransitionsPendingInFlightConfirmed(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultStateFileName)
	store := NewStateStore(path)
	runner := &recordingRunner{}
	var log bytes.Buffer
	dispatcher := NewDispatcher(store, runner, &log)

	state, err := dispatcher.DispatchNewItems(context.Background(), State{}, []BoardItem{{ID: "101", ChannelID: "c1"}})

	require.NoError(t, err)
	require.NotNil(t, state.Dispatch)
	assert.Equal(t, "101", state.Dispatch.BoardItemID)
	assert.Equal(t, 1, state.Dispatch.Attempt)
	assert.Equal(t, DispatchStateConfirmed, state.Dispatch.State)
	require.Len(t, runner.items, 1)
	assert.Equal(t, "101", runner.items[0].ID)
	assert.Contains(t, log.String(), "dispatch started: board_item_id=101")
	assert.Contains(t, log.String(), "dispatch confirmed: board_item_id=101")

	saved, err := store.Load()
	require.NoError(t, err)
	require.NotNil(t, saved.Dispatch)
	assert.Equal(t, DispatchStateConfirmed, saved.Dispatch.State)
}

func TestDispatcherSkipsNewItemsWhenDispatchActive(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultStateFileName)
	store := NewStateStore(path)
	runner := &recordingRunner{}
	var log bytes.Buffer
	dispatcher := NewDispatcher(store, runner, &log)
	state := State{
		Dispatch: &DispatchRecord{
			BoardItemID: "100",
			State:       DispatchStateInFlight,
		},
	}

	next, err := dispatcher.DispatchNewItems(context.Background(), state, []BoardItem{{ID: "101"}})

	require.NoError(t, err)
	assert.Equal(t, state.Dispatch, next.Dispatch)
	assert.Empty(t, runner.items)
	assert.Contains(t, log.String(), "dispatch already active")
}

func TestPollerRunOnceDispatchesDetectedItemAndPersistsConfirmedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultStateFileName)
	store := NewStateStore(path)
	adapter := &fakeBoardAdapter{
		items: []BoardItem{{ID: "101", ChannelID: "c1", AuthorID: "u1"}},
		next:  Checkpoint{LastMessageID: "101"},
	}
	runner := &recordingRunner{}
	var log bytes.Buffer
	poller := NewPoller(adapter, store, &log)
	poller.SetDispatcher(NewDispatcher(store, runner, &log))

	items, err := poller.RunOnce(context.Background())

	require.NoError(t, err)
	assert.Len(t, items, 1)
	require.Len(t, runner.items, 1)
	assert.Equal(t, "101", runner.items[0].ID)

	state, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, "101", state.Checkpoint.LastMessageID)
	require.NotNil(t, state.Dispatch)
	assert.Equal(t, "101", state.Dispatch.BoardItemID)
	assert.Equal(t, DispatchStateConfirmed, state.Dispatch.State)
}

func TestShellCommandRunnerExecutesCommand(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "dispatch.txt")
	runner := ShellCommandRunner{
		Command: "printf '%s:%s:%s' \"$RENKIN_BOARD_ITEM_ID\" \"$RENKIN_BOARD_CHANNEL_ID\" \"$RENKIN_BOARD_AUTHOR_ID\" > dispatch.txt",
		Dir:     dir,
	}

	err := runner.Run(context.Background(), BoardItem{ID: "101", ChannelID: "c1", AuthorID: "u1"})

	require.NoError(t, err)
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, "101:c1:u1", string(data))
}
