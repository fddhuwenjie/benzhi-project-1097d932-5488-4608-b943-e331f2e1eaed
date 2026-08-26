package audit

import "time"

const GenesisDigest = "0000000000000000000000000000000000000000000000000000000000000000"

type Event struct {
	EventID        string    `json:"event_id"`
	CaseID         string    `json:"case_id"`
	Sequence       int64     `json:"sequence"`
	EventType      string    `json:"event_type"`
	ActorID        string    `json:"actor_id"`
	Revision       int64     `json:"revision"`
	Payload        []byte    `json:"payload"`
	PayloadDigest  string    `json:"payload_digest"`
	PreviousDigest string    `json:"previous_digest"`
	EventDigest    string    `json:"event_digest"`
	OccurredAt     time.Time `json:"occurred_at"`
}

type Verification struct {
	Valid           bool   `json:"valid"`
	EventCount      int    `json:"event_count"`
	ChainHead       string `json:"chain_head"`
	Diagnostic      string `json:"diagnostic,omitempty"`
	FailureSequence int64  `json:"failure_sequence,omitempty"`
	ErrorCode       string `json:"error_code,omitempty"`
}

type Payload struct {
	Command  string `json:"command"`
	ActorID  string `json:"actor_id"`
	Revision int64  `json:"revision"`
	Result   any    `json:"result"`
}
