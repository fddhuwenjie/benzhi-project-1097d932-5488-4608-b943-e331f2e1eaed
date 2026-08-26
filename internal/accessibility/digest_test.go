package accessibility

import "testing"

func TestProfileDigestIgnoresInputOrder(t *testing.T) {
	one := CalculateProfileDigest("v1", []string{"B", "A"}, []Severity{SeverityMajor, SeverityCritical})
	two := CalculateProfileDigest("v1", []string{"A", "B"}, []Severity{SeverityCritical, SeverityMajor})
	if one != two {
		t.Fatal("稳定摘要不应受输入顺序影响")
	}
}
func TestTransitionRejectsSkip(t *testing.T) {
	a := CaseAggregate{Case: PublicationCase{Status: StatusDraft}}
	if err := a.Transition(StatusApproved); err == nil {
		t.Fatal("不应允许跳过流程状态")
	}
}
