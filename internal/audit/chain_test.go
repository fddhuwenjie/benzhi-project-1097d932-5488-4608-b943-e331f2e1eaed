package audit

import (
	"testing"
	"time"
)

func TestChainDetectsTampering(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	one, err := NewEvent("c1", "e1", "CASE_CREATED", "owner", 1, map[string]string{"status": "DRAFT"}, Event{}, now)
	if err != nil {
		t.Fatal(err)
	}
	two, err := NewEvent("c1", "e2", "PROFILE_FROZEN", "owner", 2, nil, one, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if got := Verify([]Event{one, two}); !got.Valid {
		t.Fatalf("应有效: %+v", got)
	}
	two.Payload = []byte("tampered")
	if got := Verify([]Event{one, two}); got.Valid {
		t.Fatal("篡改后不应有效")
	}
}
