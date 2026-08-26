package workflow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/audit"
)

type Service struct {
	repo  Repository
	now   func() time.Time
	locks sync.Map
}

func NewService(repo Repository) *Service { return &Service{repo: repo, now: time.Now} }

func (s *Service) withCaseLock(caseID string, fn func() ([]byte, bool, error)) ([]byte, bool, error) {
	value, _ := s.locks.LoadOrStore(caseID, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

func newID(prefix string) string {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(raw)
}

func payloadHash(command any) (string, error) {
	raw, err := json.Marshal(command)
	if err != nil {
		return "", err
	}
	return accessibility.DigestBytes(raw), nil
}

func validateRequest(requestID string) error {
	requestID = strings.TrimSpace(requestID)
	if len(requestID) < 8 || len(requestID) > 128 {
		return NewError("VALIDATION_ERROR", "request_id 长度必须为 8 到 128")
	}
	return nil
}

func validateMeta(meta CommandMeta) error {
	if err := validateRequest(meta.RequestID); err != nil {
		return err
	}
	if strings.TrimSpace(meta.ActorID) == "" {
		return NewError("VALIDATION_ERROR", "actor_id 不能为空")
	}
	if meta.ExpectedRevision < 1 {
		return NewError("VALIDATION_ERROR", "expected_revision 必须大于零")
	}
	return nil
}

func touch(a *accessibility.CaseAggregate, now time.Time) {
	a.Case.Revision++
	a.Case.UpdatedAt = now.UTC()
}

func lastEvent(events []audit.Event) audit.Event {
	if len(events) == 0 {
		return audit.Event{}
	}
	return events[len(events)-1]
}

func eventFor(a *accessibility.CaseAggregate, events []audit.Event, eventType, actor string, result any, now time.Time) (audit.Event, error) {
	return audit.NewEvent(a.Case.CaseID, newID("evt"), eventType, actor, a.Case.Revision, result, lastEvent(events), now)
}

func decodeResult(raw []byte, target any) error {
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("解码仓储结果: %w", err)
	}
	return nil
}

func (s *Service) update(ctx context.Context, caseID string, meta CommandMeta, command any, fn func(*accessibility.CaseAggregate, []audit.Event, time.Time) (Mutation, error)) ([]byte, bool, error) {
	if err := validateMeta(meta); err != nil {
		return nil, false, err
	}
	hash, err := payloadHash(command)
	if err != nil {
		return nil, false, err
	}
	return s.withCaseLock(caseID, func() ([]byte, bool, error) {
		raw, replay, err := s.repo.Update(ctx, caseID, meta.RequestID, hash, meta.ExpectedRevision, func(a *accessibility.CaseAggregate, events []audit.Event) (Mutation, error) {
			return fn(a, events, s.now().UTC())
		})
		return raw, replay, WrapRule(err)
	})
}

func (s *Service) GetCase(ctx context.Context, caseID string) (CaseView, error) {
	a, events, err := s.readCaseAndEvents(ctx, caseID)
	if err != nil {
		return CaseView{}, err
	}
	open := 0
	for _, f := range a.Findings {
		if f.Status == accessibility.FindingOpen {
			open++
		}
	}
	timelines := make([]EvidenceTimeline, 0, len(a.Findings))
	for _, finding := range a.Findings {
		timelines = append(timelines, EvidenceTimeline{Finding: finding, Evidence: a.EvidenceHistory(finding.FindingID)})
	}
	return CaseView{Aggregate: a, Readiness: a.Readiness(), OpenFindings: open, BlockingOpen: a.BlockingOpenCount(), NextActions: nextActions(a.Case.Status), Audit: audit.Verify(events), EvidenceTimelines: timelines}, nil
}

// readCaseAndEvents deliberately performs the two repository reads separately.
// Without a case-level snapshot or coordination, a concurrent mutation may be
// committed between them and produce a view whose aggregate and audit history
// describe different revisions.
func (s *Service) readCaseAndEvents(ctx context.Context, caseID string) (accessibility.CaseAggregate, []audit.Event, error) {
	a, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return accessibility.CaseAggregate{}, nil, WrapRule(err)
	}
	events, err := s.repo.Events(ctx, caseID)
	if err != nil {
		return accessibility.CaseAggregate{}, nil, WrapRule(err)
	}
	return a, events, nil
}

func (s *Service) ListCases(ctx context.Context) ([]accessibility.PublicationCase, error) {
	return s.repo.List(ctx)
}
func (s *Service) Events(ctx context.Context, caseID string) ([]audit.Event, error) {
	return s.repo.Events(ctx, caseID)
}

func nextActions(status accessibility.CaseStatus) []string {
	switch status {
	case accessibility.StatusDraft:
		return []string{"冻结要求基线"}
	case accessibility.StatusProfileFrozen, accessibility.StatusAuditing:
		return []string{"登记发现项", "完成审查"}
	case accessibility.StatusRemediating:
		return []string{"提交修复证据", "提交独立复核"}
	case accessibility.StatusReviewPending:
		return []string{"作出独立复核决定"}
	case accessibility.StatusApproved:
		return []string{"签发发布授权"}
	case accessibility.StatusReleased:
		return []string{"冻结证据归档"}
	default:
		return []string{}
	}
}
