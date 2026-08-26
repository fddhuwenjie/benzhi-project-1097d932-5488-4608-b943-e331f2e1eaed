package accessibility

var allowedTransitions = map[CaseStatus]map[CaseStatus]bool{
	StatusDraft:         {StatusProfileFrozen: true},
	StatusProfileFrozen: {StatusAuditing: true, StatusRemediating: true},
	StatusAuditing:      {StatusAuditing: true, StatusRemediating: true},
	StatusRemediating:   {StatusRemediating: true, StatusReviewPending: true},
	StatusReviewPending: {StatusApproved: true, StatusRemediating: true},
	StatusApproved:      {StatusReleased: true},
	StatusReleased:      {StatusArchived: true},
}

func CanTransition(from, to CaseStatus) bool { return allowedTransitions[from][to] }

func (a *CaseAggregate) Transition(to CaseStatus) error {
	if !CanTransition(a.Case.Status, to) {
		return NewRuleError("INVALID_STATE", "不允许从 %s 转换到 %s", a.Case.Status, to)
	}
	a.Case.Status = to
	return nil
}

func IsMutable(status CaseStatus) bool {
	return status != StatusArchived && status != StatusReleased
}
