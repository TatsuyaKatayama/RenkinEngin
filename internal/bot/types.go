package bot

import "time"

type Checkpoint struct {
	LastMessageID string    `json:"last_message_id"`
	LastCheckedAt time.Time `json:"last_checked_at,omitempty"`
}

type BoardItem struct {
	ID        string    `json:"id"`
	ChannelID string    `json:"channel_id,omitempty"`
	AuthorID  string    `json:"author_id,omitempty"`
	Content   string    `json:"content,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type DispatchState string

const (
	DispatchStatePending   DispatchState = "pending"
	DispatchStateInFlight  DispatchState = "in_flight"
	DispatchStateConfirmed DispatchState = "confirmed"
	DispatchStateExhausted DispatchState = "exhausted"
)

type DispatchRecord struct {
	BoardItemID  string        `json:"board_item_id"`
	BoardItem    BoardItem     `json:"board_item"`
	Attempt      int           `json:"attempt"`
	MaxRetries   int           `json:"max_retries"`
	RestartDelay time.Duration `json:"restart_delay"`
	Deadline     time.Time     `json:"deadline,omitempty"`
	NextRunAt    time.Time     `json:"next_run_at,omitempty"`
	State        DispatchState `json:"state"`
}

type State struct {
	Checkpoint Checkpoint      `json:"checkpoint"`
	Dispatch   *DispatchRecord `json:"dispatch,omitempty"`
}
