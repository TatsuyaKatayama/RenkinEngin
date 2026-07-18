package bot

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

const (
	DefaultDispatchMaxRetries = 3
	DefaultDispatchDelay      = 5 * time.Second
	DefaultDispatchTimeout    = 30 * time.Minute
)

type CommandRunner interface {
	Start(ctx context.Context, item BoardItem) (RunningCommand, error)
}

type RunningCommand interface {
	Done() <-chan error
	Kill() error
}

type ResolutionChecker interface {
	IsResolved(ctx context.Context, item BoardItem) (bool, error)
}

type Dispatcher struct {
	store           *StateStore
	runner          CommandRunner
	resolver        ResolutionChecker
	log             io.Writer
	maxRetries      int
	restartDelay    time.Duration
	dispatchTimeout time.Duration
	now             func() time.Time
	active          RunningCommand
	notifier        ExhaustedNotifier
}

type DispatcherOption func(*Dispatcher)

func NewDispatcher(store *StateStore, runner CommandRunner, log io.Writer, opts ...DispatcherOption) *Dispatcher {
	if log == nil {
		log = io.Discard
	}
	d := &Dispatcher{
		store:           store,
		runner:          runner,
		log:             log,
		maxRetries:      DefaultDispatchMaxRetries,
		restartDelay:    DefaultDispatchDelay,
		dispatchTimeout: DefaultDispatchTimeout,
		now:             time.Now,
	}
	for _, opt := range opts {
		opt(d)
	}
	if d.maxRetries <= 0 {
		d.maxRetries = 1
	}
	if d.restartDelay < 0 {
		d.restartDelay = 0
	}
	if d.dispatchTimeout <= 0 {
		d.dispatchTimeout = DefaultDispatchTimeout
	}
	if d.now == nil {
		d.now = time.Now
	}
	return d
}

func WithResolutionChecker(resolver ResolutionChecker) DispatcherOption {
	return func(d *Dispatcher) {
		d.resolver = resolver
	}
}

func WithDispatchPolicy(maxRetries int, restartDelay, dispatchTimeout time.Duration) DispatcherOption {
	return func(d *Dispatcher) {
		d.maxRetries = maxRetries
		d.restartDelay = restartDelay
		d.dispatchTimeout = dispatchTimeout
	}
}

func WithClock(now func() time.Time) DispatcherOption {
	return func(d *Dispatcher) {
		d.now = now
	}
}

func WithExhaustedNotifier(notifier ExhaustedNotifier) DispatcherOption {
	return func(d *Dispatcher) {
		d.notifier = notifier
	}
}

func (d *Dispatcher) DispatchNewItems(ctx context.Context, state State, items []BoardItem) (State, error) {
	return d.Tick(ctx, state, items)
}

func (d *Dispatcher) Tick(ctx context.Context, state State, items []BoardItem) (State, error) {
	var err error
	state, err = d.advanceActiveDispatch(ctx, state)
	if err != nil {
		return state, err
	}

	if state.Dispatch == nil || !isActiveDispatchState(state.Dispatch.State) {
		if len(items) == 0 {
			return state, nil
		}
		item := items[0]
		state.Dispatch = &DispatchRecord{
			BoardItemID:  item.ID,
			BoardItem:    item,
			MaxRetries:   d.maxRetries,
			RestartDelay: d.restartDelay,
			State:        DispatchStatePending,
		}
		if err := d.save(state); err != nil {
			return state, err
		}
	}

	if state.Dispatch != nil && state.Dispatch.State == DispatchStatePending {
		return d.startIfReady(ctx, state)
	}
	if state.Dispatch != nil && isActiveDispatchState(state.Dispatch.State) && len(items) > 0 {
		fmt.Fprintf(d.log, "dispatch already active: board_item_id=%s state=%s\n", state.Dispatch.BoardItemID, state.Dispatch.State)
	}
	return state, nil
}

func (d *Dispatcher) advanceActiveDispatch(ctx context.Context, state State) (State, error) {
	if state.Dispatch == nil || !isActiveDispatchState(state.Dispatch.State) {
		return state, nil
	}
	if state.Dispatch.BoardItem.ID == "" {
		state.Dispatch.BoardItem.ID = state.Dispatch.BoardItemID
	}

	if resolved, err := d.isResolved(ctx, state.Dispatch.BoardItem); err != nil {
		return state, err
	} else if resolved {
		state.Dispatch.State = DispatchStateConfirmed
		if err := d.save(state); err != nil {
			return state, err
		}
		fmt.Fprintf(d.log, "dispatch confirmed: board_item_id=%s\n", state.Dispatch.BoardItemID)
		return state, nil
	}

	if d.active == nil {
		if state.Dispatch.State == DispatchStateInFlight {
			return d.scheduleRetryOrExhaust(ctx, state)
		}
		return state, nil
	}

	select {
	case err := <-d.active.Done():
		d.active = nil
		if err != nil {
			fmt.Fprintf(d.log, "dispatch command exited with error: board_item_id=%s error=%v\n", state.Dispatch.BoardItemID, err)
		} else {
			state.Dispatch.State = DispatchStateConfirmed
			if err := d.save(state); err != nil {
				return state, err
			}
			fmt.Fprintf(d.log, "dispatch confirmed: board_item_id=%s\n", state.Dispatch.BoardItemID)
			return state, nil
		}
		if resolved, err := d.isResolved(ctx, state.Dispatch.BoardItem); err != nil {
			return state, err
		} else if resolved {
			state.Dispatch.State = DispatchStateConfirmed
			if err := d.save(state); err != nil {
				return state, err
			}
			fmt.Fprintf(d.log, "dispatch confirmed: board_item_id=%s\n", state.Dispatch.BoardItemID)
			return state, nil
		}
		return d.scheduleRetryOrExhaust(ctx, state)
	default:
		if !state.Dispatch.Deadline.IsZero() && !d.now().Before(state.Dispatch.Deadline) {
			if err := d.active.Kill(); err != nil {
				fmt.Fprintf(d.log, "dispatch kill failed: board_item_id=%s error=%v\n", state.Dispatch.BoardItemID, err)
			}
			d.active = nil
			fmt.Fprintf(d.log, "dispatch deadline reached: board_item_id=%s\n", state.Dispatch.BoardItemID)
			return d.scheduleRetryOrExhaust(ctx, state)
		}
		return state, nil
	}
}

func (d *Dispatcher) startIfReady(ctx context.Context, state State) (State, error) {
	if state.Dispatch == nil || state.Dispatch.State != DispatchStatePending {
		return state, nil
	}
	if !state.Dispatch.NextRunAt.IsZero() && d.now().Before(state.Dispatch.NextRunAt) {
		return state, nil
	}
	if state.Dispatch.Attempt >= state.Dispatch.MaxRetries {
		state.Dispatch.State = DispatchStateExhausted
		if err := d.save(state); err != nil {
			return state, err
		}
		fmt.Fprintf(d.log, "dispatch exhausted: board_item_id=%s attempts=%d\n", state.Dispatch.BoardItemID, state.Dispatch.Attempt)
		d.notifyExhausted(ctx, *state.Dispatch)
		return state, nil
	}

	cmd, err := d.runner.Start(ctx, state.Dispatch.BoardItem)
	if err != nil {
		return state, fmt.Errorf("dispatch command failed to start for board_item_id=%s: %w", state.Dispatch.BoardItemID, err)
	}
	d.active = cmd
	state.Dispatch.Attempt++
	state.Dispatch.MaxRetries = d.maxRetries
	state.Dispatch.RestartDelay = d.restartDelay
	state.Dispatch.Deadline = d.now().Add(d.dispatchTimeout)
	state.Dispatch.NextRunAt = time.Time{}
	state.Dispatch.State = DispatchStateInFlight
	if err := d.save(state); err != nil {
		return state, err
	}
	fmt.Fprintf(d.log, "dispatch started: board_item_id=%s attempt=%d\n", state.Dispatch.BoardItemID, state.Dispatch.Attempt)
	return state, nil
}

func (d *Dispatcher) scheduleRetryOrExhaust(ctx context.Context, state State) (State, error) {
	if state.Dispatch == nil {
		return state, nil
	}
	if state.Dispatch.Attempt >= state.Dispatch.MaxRetries {
		state.Dispatch.State = DispatchStateExhausted
		if err := d.save(state); err != nil {
			return state, err
		}
		fmt.Fprintf(d.log, "dispatch exhausted: board_item_id=%s attempts=%d\n", state.Dispatch.BoardItemID, state.Dispatch.Attempt)
		d.notifyExhausted(ctx, *state.Dispatch)
		return state, nil
	}
	state.Dispatch.State = DispatchStatePending
	state.Dispatch.NextRunAt = d.now().Add(state.Dispatch.RestartDelay)
	state.Dispatch.Deadline = time.Time{}
	if err := d.save(state); err != nil {
		return state, err
	}
	fmt.Fprintf(d.log, "dispatch retry scheduled: board_item_id=%s next_attempt=%d\n", state.Dispatch.BoardItemID, state.Dispatch.Attempt+1)
	return state, nil
}

func (d *Dispatcher) notifyExhausted(ctx context.Context, record DispatchRecord) {
	if d.notifier == nil {
		return
	}
	if err := d.notifier.NotifyExhausted(ctx, record); err != nil {
		fmt.Fprintf(d.log, "dispatch exhausted notification failed: board_item_id=%s error=%v\n", record.BoardItemID, err)
	}
}

func (d *Dispatcher) isResolved(ctx context.Context, item BoardItem) (bool, error) {
	if d.resolver == nil {
		return true, nil
	}
	return d.resolver.IsResolved(ctx, item)
}

func (d *Dispatcher) save(state State) error {
	if d.store == nil {
		return nil
	}
	return d.store.Save(state)
}

func isActiveDispatchState(state DispatchState) bool {
	return state == DispatchStatePending || state == DispatchStateInFlight
}

type ShellCommandRunner struct {
	Command string
	Dir     string
	Stdout  io.Writer
	Stderr  io.Writer
}

func (r ShellCommandRunner) Start(ctx context.Context, item BoardItem) (RunningCommand, error) {
	if r.Command == "" {
		return nil, fmt.Errorf("dispatch command is empty")
	}
	cmd := exec.CommandContext(ctx, "sh", "-lc", r.Command)
	cmd.Dir = r.Dir
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	cmd.Env = append(os.Environ(),
		"RENKIN_BOARD_ITEM_ID="+item.ID,
		"RENKIN_BOARD_CHANNEL_ID="+item.ChannelID,
		"RENKIN_BOARD_AUTHOR_ID="+item.AuthorID,
		"RENKIN_BOARD_CONTENT="+item.Content,
		"RENKIN_BOARD_CREATED_AT="+item.CreatedAt.Format(time.RFC3339Nano),
	)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	running := &shellRunningCommand{
		cmd:  cmd,
		done: make(chan error, 1),
	}
	go func() {
		running.done <- cmd.Wait()
	}()
	return running, nil
}

type shellRunningCommand struct {
	cmd  *exec.Cmd
	done chan error
}

func (c *shellRunningCommand) Done() <-chan error {
	return c.done
}

func (c *shellRunningCommand) Kill() error {
	if c.cmd.Process == nil {
		return nil
	}
	return c.cmd.Process.Kill()
}
