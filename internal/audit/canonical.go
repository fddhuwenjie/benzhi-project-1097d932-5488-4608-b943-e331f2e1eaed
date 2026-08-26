package audit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func CanonicalPayload(command, actorID string, revision int64, result any) ([]byte, error) {
	var normalized any
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&normalized); err != nil {
		return nil, err
	}
	return json.Marshal(Payload{Command: command, ActorID: actorID, Revision: revision, Result: normalized})
}

type eventDigestInput struct {
	CaseID         string `json:"case_id"`
	Sequence       int64  `json:"sequence"`
	EventType      string `json:"event_type"`
	ActorID        string `json:"actor_id"`
	Revision       int64  `json:"revision"`
	PayloadDigest  string `json:"payload_digest"`
	PreviousDigest string `json:"previous_digest"`
	OccurredAt     string `json:"occurred_at"`
}

func calculateEventDigest(e Event) string {
	input := eventDigestInput{
		CaseID: e.CaseID, Sequence: e.Sequence, EventType: e.EventType,
		ActorID: e.ActorID, Revision: e.Revision, PayloadDigest: e.PayloadDigest,
		PreviousDigest: e.PreviousDigest, OccurredAt: e.OccurredAt.UTC().Format("2006-01-02T15:04:05.000000000Z"),
	}
	raw, _ := json.Marshal(input)
	return Digest(raw)
}
