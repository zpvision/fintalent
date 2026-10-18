package academy

import (
	"math"
	"testing"
)

func TestAssessmentPoliciesAndReview(t *testing.T) {
	b := Block{Type: "assignment", Data: BlockData{AnswerFormat: "text"}}
	if err := validateAssessment(&b); err != nil {
		t.Fatal(err)
	}
	p := policy(b)
	if p.MaxAttempts != 3 || !yes(p.Required) || !yes(p.AllowRevision) {
		t.Fatal("legacy defaults")
	}
	c1, c2 := Key(), Key()
	p.Criteria = []Criterion{{Key: c1, Title: "Accuracy", MaxScore: 60}, {Key: c2, Title: "Quality", MaxScore: 40}}
	pass := 70.0
	p.PassingScore = &pass
	if _, err := reviewScore(p, nil, map[string]float64{c1: 61, c2: 40}, "accepted"); err == nil {
		t.Fatal("excess criterion score")
	}
	if score, err := reviewScore(p, nil, map[string]float64{c1: 50, c2: 30}, "accepted"); err != nil || *score != 80 {
		t.Fatal(score, err)
	}
	if _, err := reviewScore(p, nil, nil, "revision_requested"); err != nil {
		t.Fatal("revision should not require grade", err)
	}
	low := 40.0
	if _, err := reviewScore(AssessmentPolicy{MaxScore: 100, PassingScore: &pass}, &low, nil, "accepted"); err == nil {
		t.Fatal("accepted below threshold")
	}
	invalid := math.Inf(1)
	if _, err := reviewScore(p, &invalid, nil, "rejected"); err == nil {
		t.Fatal("nonfinite score")
	}
	p.ReviewMode = "automatic"
	b.Data.Assessment = &p
	if validateAssessment(&b) == nil {
		t.Fatal("automatic mode promised substantive grading")
	}
}
func TestSubmissionFormatsAndRequirements(t *testing.T) {
	p := AssessmentPolicy{Formats: []string{"text", "file", "link"}}
	answer := SubmissionAnswer{Text: "Answer", Links: []string{"https://example.invalid/work"}}
	if validateSubmission(&answer, p, true) == nil {
		t.Fatal("missing required attachment")
	}
	answer.AssetIDs = []int64{1}
	if err := validateSubmission(&answer, p, true); err != nil {
		t.Fatal(err)
	}
	answer.Links = []string{"javascript:alert(1)"}
	if validateSubmission(&answer, p, false) == nil {
		t.Fatal("unsafe link")
	}
	l := NewLesson()
	l.Blocks = []Block{{Key: Key(), Type: "test", Data: BlockData{TestVersionID: 1}}, {Key: Key(), Type: "assignment", Data: BlockData{AnswerFormat: "text"}}}
	states := map[string]bool{l.Blocks[0].Key: true}
	if requirementsReason(l, states) == "" {
		t.Fatal("test alone completed mixed lesson")
	}
	states[l.Blocks[1].Key] = true
	if reason := requirementsReason(l, states); reason != "" {
		t.Fatal(reason)
	}
	optional := false
	l.Blocks[1].Data.Assessment = &AssessmentPolicy{Required: &optional}
	delete(states, l.Blocks[1].Key)
	if reason := requirementsReason(l, states); reason != "" {
		t.Fatal("optional block blocked lesson", reason)
	}
}
func TestAssessmentAttemptAvailability(t *testing.T) {
	disabled := false
	p := AssessmentPolicy{MaxAttempts: 3, AllowRetry: &disabled}
	if remainingTestAttempts(p, 0) != 1 || remainingTestAttempts(p, 1) != 0 {
		t.Fatal("retry-disabled attempt counter")
	}
	if !canRequestRevision(p, 2) || canRequestRevision(p, 3) {
		t.Fatal("revision should require another available attempt")
	}
	p.AllowRevision = &disabled
	if canRequestRevision(p, 1) {
		t.Fatal("revision disabled")
	}
}
