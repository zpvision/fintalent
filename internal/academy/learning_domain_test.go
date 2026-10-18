package academy

import "testing"

func learningFixture() Program {
	m := NewModule()
	m.Status = "published"
	for i := 0; i < 3; i++ {
		l := NewLesson()
		l.Status = "published"
		l.Blocks = []Block{{Key: Key(), Type: "text", Data: BlockData{Rich: []RichBlock{{Type: "paragraph", Text: "Материалы"}}}}}
		if i == 1 {
			l.Required = false
		}
		m.Lessons = append(m.Lessons, l)
	}
	return Program{Modules: []Module{m}}
}
func TestLearningProgress(t *testing.T) {
	p := learningFixture()
	keys := []string{p.Modules[0].Lessons[0].Key, p.Modules[0].Lessons[1].Key, p.Modules[0].Lessons[2].Key}
	states := []Progress{{LessonKey: keys[0], Status: "completed"}, {LessonKey: keys[1], Status: "completed"}}
	s := SummaryFor(p, states, true)
	if s.Percent != 50 || s.Required != 2 || s.CompletedLessons != 2 {
		t.Fatal(s)
	}
	states = append(states, Progress{LessonKey: keys[2], Status: "completed"})
	s = SummaryFor(p, states, true)
	if s.Percent != 100 || s.Status != "completed" {
		t.Fatal(s)
	}
	p.Modules[0].Lessons[2].Blocks = append(p.Modules[0].Lessons[2].Blocks, Block{Key: Key(), Type: "test"})
	s = SummaryFor(p, states, true)
	if s.Percent != 50 || s.Status == "completed" {
		t.Fatal("unsupported interactive requirement counted", s)
	}
	for i := range p.Modules[0].Lessons {
		p.Modules[0].Lessons[i].Required = false
	}
	s = SummaryFor(p, states, true)
	if s.Percent != 0 || s.Status == "completed" {
		t.Fatal("empty denominator became complete", s)
	}
}
func TestCompletionRules(t *testing.T) {
	l := NewLesson()
	l.CompletionRule = "materials"
	l.Blocks = []Block{{Type: "text"}}
	if CompletionBlock(l) != "" {
		t.Fatal("text attestation blocked")
	}
	l.Blocks = append(l.Blocks, Block{Type: "video"})
	if CompletionBlock(l) == "" {
		t.Fatal("video opening counted as viewing")
	}
	l.CompletionRule = "manual"
	if CompletionBlock(l) != "" {
		t.Fatal("explicit manual video completion blocked")
	}
	for _, kind := range []string{"test", "assignment"} {
		l.CompletionRule = kind
		if CompletionBlock(l) == "" {
			t.Fatal(kind)
		}
	}
}
func TestReadinessAndResume(t *testing.T) {
	p := learningFixture()
	if len(CheckReadiness(p).Blockers) > 0 {
		t.Fatal(CheckReadiness(p))
	}
	p.Modules[0].Lessons[0].Status = "draft"
	if len(CheckReadiness(p).Blockers) == 0 {
		t.Fatal("required draft allowed")
	}
	p.Modules[0].Lessons[0].Status = "published"
	key := p.Modules[0].Lessons[0].Key
	next := p.Modules[0].Lessons[1].Key
	if resumeLesson(p, key, []Progress{{LessonKey: key, Status: "completed"}}) != next {
		t.Fatal("resume did not advance")
	}
	if _, ok := findLesson(p, "foreign-key"); ok {
		t.Fatal("foreign lesson found")
	}
	p.Modules[0].Lessons[0].SelfPaced = false
	if _, ok := findLesson(p, key); ok {
		t.Fatal("non-self-paced lesson exposed")
	}
}
