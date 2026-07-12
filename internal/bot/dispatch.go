package bot

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type CommandRunner interface {
	Run(ctx context.Context, item BoardItem) error
}

type Dispatcher struct {
	store  *StateStore
	runner CommandRunner
	log    io.Writer
}

func NewDispatcher(store *StateStore, runner CommandRunner, log io.Writer) *Dispatcher {
	if log == nil {
		log = io.Discard
	}
	return &Dispatcher{
		store:  store,
		runner: runner,
		log:    log,
	}
}

func (d *Dispatcher) DispatchNewItems(ctx context.Context, state State, items []BoardItem) (State, error) {
	if len(items) == 0 {
		return state, nil
	}
	if state.Dispatch != nil && isActiveDispatchState(state.Dispatch.State) {
		fmt.Fprintf(d.log, "dispatch already active: board_item_id=%s state=%s\n", state.Dispatch.BoardItemID, state.Dispatch.State)
		return state, nil
	}

	item := items[0]
	state.Dispatch = &DispatchRecord{
		BoardItemID: item.ID,
		Attempt:     1,
		State:       DispatchStatePending,
	}
	if err := d.save(state); err != nil {
		return state, err
	}

	state.Dispatch.State = DispatchStateInFlight
	if err := d.save(state); err != nil {
		return state, err
	}
	fmt.Fprintf(d.log, "dispatch started: board_item_id=%s\n", item.ID)

	if err := d.runner.Run(ctx, item); err != nil {
		return state, fmt.Errorf("dispatch command failed for board_item_id=%s: %w", item.ID, err)
	}

	state.Dispatch.State = DispatchStateConfirmed
	if err := d.save(state); err != nil {
		return state, err
	}
	fmt.Fprintf(d.log, "dispatch confirmed: board_item_id=%s\n", item.ID)
	return state, nil
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

func (r ShellCommandRunner) Run(ctx context.Context, item BoardItem) error {
	if r.Command == "" {
		return fmt.Errorf("dispatch command is empty")
	}
	cmd := exec.CommandContext(ctx, "sh", "-lc", r.Command)
	cmd.Dir = r.Dir
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	cmd.Env = append(os.Environ(),
		"RENKIN_BOARD_ITEM_ID="+item.ID,
		"RENKIN_BOARD_CHANNEL_ID="+item.ChannelID,
		"RENKIN_BOARD_AUTHOR_ID="+item.AuthorID,
	)
	return cmd.Run()
}
