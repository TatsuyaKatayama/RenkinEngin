package bot

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingRunner struct {
	items    []BoardItem
	commands []*fakeRunningCommand
}

func (r *recordingRunner) Start(_ context.Context, item BoardItem) (RunningCommand, error) {
	r.items = append(r.items, item)
	cmd := newFakeRunningCommand()
	r.commands = append(r.commands, cmd)
	return cmd, nil
}

type fakeRunningCommand struct {
	done   chan error
	killed bool
}

func newFakeRunningCommand() *fakeRunningCommand {
	return &fakeRunningCommand{done: make(chan error, 1)}
}

func (c *fakeRunningCommand) Done() <-chan error {
	return c.done
}

func (c *fakeRunningCommand) Kill() error {
	c.killed = true
	c.done <- context.DeadlineExceeded
	return nil
}

func (c *fakeRunningCommand) finish(err error) {
	c.done <- err
}

type fakeResolver struct {
	resolved bool
}

func (r *fakeResolver) IsResolved(_ context.Context, _ BoardItem) (bool, error) {
	return r.resolved, nil
}

type recordingNotifier struct {
	records []DispatchRecord
	err     error
}

func (n *recordingNotifier) NotifyExhausted(_ context.Context, record DispatchRecord) error {
	n.records = append(n.records, record)
	return n.err
}

func TestDispatcherTransitionsPendingInFlightConfirmed(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultStateFileName)
	store := NewStateStore(path)
	runner := &recordingRunner{}
	resolver := &fakeResolver{}
	var log bytes.Buffer
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	dispatcher := NewDispatcher(store, runner, &log, WithResolutionChecker(resolver), WithDispatchPolicy(3, time.Second, time.Minute), WithClock(func() time.Time {
		return now
	}))

	state, err := dispatcher.DispatchNewItems(context.Background(), State{}, []BoardItem{{ID: "101", ChannelID: "c1"}})

	require.NoError(t, err)
	require.NotNil(t, state.Dispatch)
	assert.Equal(t, "101", state.Dispatch.BoardItemID)
	assert.Equal(t, 1, state.Dispatch.Attempt)
	assert.Equal(t, DispatchStateInFlight, state.Dispatch.State)
	require.Len(t, runner.items, 1)
	assert.Equal(t, "101", runner.items[0].ID)
	assert.Contains(t, log.String(), "dispatch started: board_item_id=101")

	resolver.resolved = true
	runner.commands[0].finish(nil)
	state, err = dispatcher.Tick(context.Background(), state, nil)

	require.NoError(t, err)
	require.NotNil(t, state.Dispatch)
	assert.Equal(t, DispatchStateConfirmed, state.Dispatch.State)
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
	dispatcher := NewDispatcher(store, runner, &log, WithResolutionChecker(&fakeResolver{}))
	state, err := dispatcher.DispatchNewItems(context.Background(), State{}, []BoardItem{{ID: "100"}})
	require.NoError(t, err)

	next, err := dispatcher.DispatchNewItems(context.Background(), state, []BoardItem{{ID: "101"}})

	require.NoError(t, err)
	assert.Equal(t, state.Dispatch, next.Dispatch)
	assert.Len(t, runner.items, 1)
	assert.Contains(t, log.String(), "dispatch already active")
}

func TestDispatcherRetriesThenExhaustsWhenCommandFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultStateFileName)
	store := NewStateStore(path)
	runner := &recordingRunner{}
	var log bytes.Buffer
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	dispatcher := NewDispatcher(store, runner, &log, WithResolutionChecker(&fakeResolver{}), WithDispatchPolicy(2, time.Second, time.Minute), WithClock(func() time.Time {
		return now
	}))

	state, err := dispatcher.DispatchNewItems(context.Background(), State{}, []BoardItem{{ID: "101"}})
	require.NoError(t, err)
	runner.commands[0].finish(assert.AnError)
	state, err = dispatcher.Tick(context.Background(), state, nil)
	require.NoError(t, err)
	assert.Equal(t, DispatchStatePending, state.Dispatch.State)
	assert.Equal(t, 1, state.Dispatch.Attempt)

	now = now.Add(time.Second)
	state, err = dispatcher.Tick(context.Background(), state, nil)
	require.NoError(t, err)
	assert.Equal(t, DispatchStateInFlight, state.Dispatch.State)
	assert.Equal(t, 2, state.Dispatch.Attempt)

	runner.commands[1].finish(assert.AnError)
	state, err = dispatcher.Tick(context.Background(), state, nil)
	require.NoError(t, err)
	assert.Equal(t, DispatchStateExhausted, state.Dispatch.State)
	assert.Contains(t, log.String(), "dispatch exhausted: board_item_id=101 attempts=2")
}

func TestDispatcherNotifiesWhenExhausted(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultStateFileName)
	store := NewStateStore(path)
	runner := &recordingRunner{}
	notifier := &recordingNotifier{}
	dispatcher := NewDispatcher(store, runner, nil, WithResolutionChecker(&fakeResolver{}), WithDispatchPolicy(1, 0, time.Minute), WithExhaustedNotifier(notifier))

	state, err := dispatcher.DispatchNewItems(context.Background(), State{}, []BoardItem{{ID: "101"}})
	require.NoError(t, err)
	runner.commands[0].finish(assert.AnError)
	state, err = dispatcher.Tick(context.Background(), state, nil)

	require.NoError(t, err)
	assert.Equal(t, DispatchStateExhausted, state.Dispatch.State)
	require.Len(t, notifier.records, 1)
	assert.Equal(t, "101", notifier.records[0].BoardItemID)
}

func TestDispatcherIgnoresExhaustedNotificationFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultStateFileName)
	store := NewStateStore(path)
	runner := &recordingRunner{}
	notifier := &recordingNotifier{err: assert.AnError}
	var log bytes.Buffer
	dispatcher := NewDispatcher(store, runner, &log, WithResolutionChecker(&fakeResolver{}), WithDispatchPolicy(1, 0, time.Minute), WithExhaustedNotifier(notifier))

	state, err := dispatcher.DispatchNewItems(context.Background(), State{}, []BoardItem{{ID: "101"}})
	require.NoError(t, err)
	runner.commands[0].finish(assert.AnError)
	state, err = dispatcher.Tick(context.Background(), state, nil)

	require.NoError(t, err)
	assert.Equal(t, DispatchStateExhausted, state.Dispatch.State)
	assert.Contains(t, log.String(), "dispatch exhausted notification failed")
}

func TestDispatcherKillsProcessAtDeadline(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultStateFileName)
	store := NewStateStore(path)
	runner := &recordingRunner{}
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	dispatcher := NewDispatcher(store, runner, nil, WithResolutionChecker(&fakeResolver{}), WithDispatchPolicy(1, 0, time.Second), WithClock(func() time.Time {
		return now
	}))

	state, err := dispatcher.DispatchNewItems(context.Background(), State{}, []BoardItem{{ID: "101"}})
	require.NoError(t, err)
	now = now.Add(time.Second)
	state, err = dispatcher.Tick(context.Background(), state, nil)

	require.NoError(t, err)
	assert.True(t, runner.commands[0].killed)
	assert.Equal(t, DispatchStateExhausted, state.Dispatch.State)
}

func TestPollerRunOnceDispatchesDetectedItemAndPersistsInFlightState(t *testing.T) {
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
	assert.Equal(t, DispatchStateInFlight, state.Dispatch.State)
}

func TestShellCommandRunnerExecutesCommand(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "dispatch.txt")
	runner := ShellCommandRunner{
		Command: "printf '%s:%s:%s:%s:%s' \"$RENKIN_BOARD_ITEM_ID\" \"$RENKIN_BOARD_CHANNEL_ID\" \"$RENKIN_BOARD_AUTHOR_ID\" \"$RENKIN_BOARD_CONTENT\" \"$RENKIN_BOARD_CREATED_AT\" > dispatch.txt",
		Dir:     dir,
	}

	createdAt := time.Date(2026, 7, 18, 5, 45, 0, 0, time.UTC)
	cmd, err := runner.Start(context.Background(), BoardItem{ID: "101", ChannelID: "c1", AuthorID: "u1", Content: "hello", CreatedAt: createdAt})

	require.NoError(t, err)
	require.NoError(t, <-cmd.Done())
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, "101:c1:u1:hello:2026-07-18T05:45:00Z", string(data))
}
