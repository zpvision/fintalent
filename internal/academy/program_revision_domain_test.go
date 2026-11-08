package academy

import (
	test "FinTalent/internal/testmodule/domain"
	"reflect"
	"testing"
)

func revisionTestProgram() Program {
	l := NewLesson()
	l.ID = 30
	l.Blocks = []Block{{ID: 40, Key: Key(), Type: "text", Data: BlockData{Text: "Original"}},
		{ID: 41, Key: Key(), Type: "test", Data: BlockData{TestVersionID: 5}}}
	m := NewModule()
	m.ID, m.CoverAssetID, m.Lessons = 20, 66, []Lesson{l}
	return Program{ID: 10, CourseID: 9, Revision: 8, Source: "empty", Modules: []Module{m}}
}

func TestIndependentProgramIdentitiesAndMedia(t *testing.T) {
	p := revisionTestProgram()
	p.VersionID, p.PublicationID = 7, 99
	before := clone(p)
	other, mapping := independentProgram(p)
	if other.VersionID != 0 || other.PublicationID != 0 {
		t.Fatal("copy inherited publication identity")
	}
	if !reflect.DeepEqual(p, before) {
		t.Fatal("source changed")
	}
	if other.ID != p.ID || other.CourseID != p.CourseID {
		t.Fatal("course/program scope changed")
	}
	m, l := other.Modules[0], other.Modules[0].Lessons[0]
	if m.ID != 0 || l.ID != 0 || m.CoverAssetID != 66 || len(mapping) != 4 {
		t.Fatal("copy metadata")
	}
	if m.Key == p.Modules[0].Key || l.Key == p.Modules[0].Lessons[0].Key {
		t.Fatal("shared logical identity")
	}
	for i, b := range l.Blocks {
		if b.ID != 0 || b.Key == p.Modules[0].Lessons[0].Blocks[i].Key {
			t.Fatal("shared block identity")
		}
	}
	other.Modules[0].Lessons[0].Blocks[0].Data.Text = "New version"
	other.Modules[0].Lessons[0].Blocks[1].Data.Assessment = &AssessmentPolicy{MaxAttempts: 10}
	if !reflect.DeepEqual(p, before) {
		t.Fatal("nested data shared")
	}
}

func TestProgramRevisionChanges(t *testing.T) {
	p := revisionTestProgram()
	t.Run("same", func(t *testing.T) {
		if c := programChanges(p, clone(p)); c != (ProgramChanges{}) {
			t.Fatalf("%+v", c)
		}
	})
	t.Run("wording preserves keys and requirements", func(t *testing.T) {
		next := clone(p)
		next.Modules[0].Lessons[0].Blocks[0].Data.Text = "Corrected wording"
		c := programChanges(p, next)
		if c.ChangedLessons != 1 || c.RequirementsChanged || c.AddedLessons != 0 {
			t.Fatalf("%+v", c)
		}
		if next.Modules[0].Lessons[0].Key != p.Modules[0].Lessons[0].Key {
			t.Fatal("lesson identity changed")
		}
	})
	t.Run("bonus", func(t *testing.T) {
		next := clone(p)
		bonus := NewLesson()
		bonus.Required = false
		next.Modules[0].Lessons = append(next.Modules[0].Lessons, bonus)
		c := programChanges(p, next)
		if c.AddedLessons != 1 || c.RequirementsChanged {
			t.Fatalf("%+v", c)
		}
	})
	t.Run("required addition", func(t *testing.T) {
		next := clone(p)
		next.Modules[0].Lessons = append(next.Modules[0].Lessons, NewLesson())
		c := programChanges(p, next)
		if c.AddedLessons != 1 || !c.RequirementsChanged {
			t.Fatalf("%+v", c)
		}
	})
	t.Run("archive keeps source", func(t *testing.T) {
		next := clone(p)
		next.Modules[0].Status = "draft"
		c := programChanges(p, next)
		if c.ArchivedLessons != 1 || !c.RequirementsChanged || p.Modules[0].Status != "published" {
			t.Fatalf("%+v", c)
		}
	})
	t.Run("new required test", func(t *testing.T) {
		next := clone(p)
		next.Modules[0].Lessons[0].Blocks = append(next.Modules[0].Lessons[0].Blocks, Block{Key: Key(), Type: "test", Data: BlockData{TestVersionID: 6}})
		c := programChanges(p, next)
		if c.AddedTests != 1 || !c.RequirementsChanged {
			t.Fatalf("%+v", c)
		}
	})
	t.Run("reorder", func(t *testing.T) {
		next := clone(p)
		next.Modules = append(next.Modules, NewModule())
		p2 := clone(next)
		next.Modules[0], next.Modules[1] = next.Modules[1], next.Modules[0]
		if c := programChanges(p2, next); c != (ProgramChanges{}) {
			t.Fatalf("%+v", c)
		}
	})
}

func TestIndependentVersionCannotInheritLessonProgress(t *testing.T) {
	p := revisionTestProgram()
	next, _ := independentProgram(p)
	progress := []Progress{{LessonKey: p.Modules[0].Lessons[0].Key, Status: "completed"}}
	if SummaryFor(p, progress, true).Completed != 1 {
		t.Fatal("original completion missing")
	}
	if SummaryFor(next, progress, true).Completed != 0 {
		t.Fatal("new version inherited progress")
	}
}

func TestRevisionSummaryNewRequiredTestDoesNotEraseAchievement(t *testing.T) {
	p := revisionTestProgram()
	key := p.Modules[0].Lessons[0].Blocks[1].Key
	progress := []Progress{{LessonKey: p.Modules[0].Lessons[0].Key, Status: "completed"}}
	if s := enrollmentSummary(p, progress, map[string]bool{}, true, false); s.Completed != 0 || s.Status == "completed" {
		t.Fatal("new required test ignored", s)
	}
	if s := enrollmentSummary(p, progress, map[string]bool{key: true}, true, false); s.Completed != 1 || s.Status != "completed" {
		t.Fatal("test success not counted", s)
	}
	if s := enrollmentSummary(p, progress, map[string]bool{}, true, true); s.Status != "completed" {
		t.Fatal("attained completion revoked", s)
	}
	if progress[0].Status != "completed" {
		t.Fatal("historical lesson progress mutated")
	}
}

func TestRevisionGradingChangeWarningIgnoresWording(t *testing.T) {
	q := []test.Question{{ID: 1, Question: "Original", Type: "single_choice", Points: 10, Answers: []test.Answer{{ID: 1, Answer: "Yes", IsCorrect: true}, {ID: 2, Answer: "No"}}}}
	next := clone(q)
	next[0].Question = "Corrected wording"
	next[0].Explanation = "Explanation"
	next[0].Answers[0].Answer = "Corrected answer wording"
	if !reflect.DeepEqual(gradingSignature(q), gradingSignature(next)) {
		t.Fatal("cosmetic change treated as new grading")
	}
	next[0].Answers[0].IsCorrect = false
	next[0].Answers[1].IsCorrect = true
	if reflect.DeepEqual(gradingSignature(q), gradingSignature(next)) {
		t.Fatal("answer correction not detected")
	}
}
