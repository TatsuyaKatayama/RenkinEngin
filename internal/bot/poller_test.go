package bot

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeBoardAdapter struct {
	gotCheckpoint Checkpoint
	items         []BoardItem
	next          Checkpoint
}

func (f *fakeBoardAdapter) PollSince(_ context.Context, cp Checkpoint) ([]BoardItem, Checkpoint, error) {
	f.gotCheckpoint = cp
	return f.items, f.next, nil
}

func TestPollerRunOnceLogsItemsAndPersistsCheckpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultStateFileName)
	store := NewStateStore(path)
	require.NoError(t, store.Save(State{Checkpoint: Checkpoint{LastMessageID: "100"}}))
	adapter := &fakeBoardAdapter{
		items: []BoardItem{
			{ID: "101", ChannelID: "c1", AuthorID: "u1"},
		},
		next: Checkpoint{LastMessageID: "101"},
	}
	var log bytes.Buffer
	poller := NewPoller(adapter, store, &log)

	items, err := poller.RunOnce(context.Background())

	require.NoError(t, err)
	assert.Equal(t, Checkpoint{LastMessageID: "100"}, adapter.gotCheckpoint)
	assert.Len(t, items, 1)
	assert.Contains(t, log.String(), "new board item: id=101 channel=c1 author=u1")

	state, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, "101", state.Checkpoint.LastMessageID)
}
