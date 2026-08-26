package audit

import (
	"fmt"
	"time"
)

func NewEvent(caseID, eventID, eventType, actorID string, revision int64, result any, previous Event, at time.Time) (Event, error) {
	payload, err := CanonicalPayload(eventType, actorID, revision, result)
	if err != nil {
		return Event{}, fmt.Errorf("规范化审计载荷: %w", err)
	}
	sequence := int64(1)
	previousDigest := GenesisDigest
	if previous.Sequence > 0 {
		sequence = previous.Sequence + 1
		previousDigest = previous.EventDigest
	}
	e := Event{
		EventID: eventID, CaseID: caseID, Sequence: sequence, EventType: eventType,
		ActorID: actorID, Revision: revision, Payload: payload, PayloadDigest: Digest(payload),
		PreviousDigest: previousDigest, OccurredAt: at.UTC(),
	}
	e.EventDigest = calculateEventDigest(e)
	return e, nil
}

func Verify(events []Event) Verification {
	result := Verification{Valid: true, EventCount: len(events), ChainHead: GenesisDigest}
	previous := GenesisDigest
	for i, e := range events {
		expectedSequence := int64(i + 1)
		if e.Sequence != expectedSequence {
			return broken(result, expectedSequence, "EVENT_SEQUENCE_MISMATCH", fmt.Sprintf("事件序号不连续：期望 %d，得到 %d", expectedSequence, e.Sequence))
		}
		if e.PreviousDigest != previous {
			return broken(result, expectedSequence, "PREVIOUS_DIGEST_MISMATCH", fmt.Sprintf("第 %d 个事件的前序摘要不匹配", expectedSequence))
		}
		if Digest(e.Payload) != e.PayloadDigest {
			return broken(result, expectedSequence, "PAYLOAD_DIGEST_MISMATCH", fmt.Sprintf("第 %d 个事件的载荷摘要损坏", expectedSequence))
		}
		if calculateEventDigest(e) != e.EventDigest {
			return broken(result, expectedSequence, "EVENT_DIGEST_MISMATCH", fmt.Sprintf("第 %d 个事件的事件摘要损坏", expectedSequence))
		}
		previous = e.EventDigest
		result.ChainHead = previous
	}
	return result
}

func broken(v Verification, sequence int64, code, diagnostic string) Verification {
	v.Valid = false
	v.Diagnostic = diagnostic
	v.FailureSequence = sequence
	v.ErrorCode = code
	return v
}

func ChainHead(events []Event) (string, error) {
	v := Verify(events)
	if !v.Valid {
		return "", fmt.Errorf("审计链校验失败: %s", v.Diagnostic)
	}
	return v.ChainHead, nil
}
