package academy

import "reflect"

func defaultAccessMode(value string) string {
	if value == "immediate" {
		return value
	}
	return "schedule"
}

func enrollmentSummary(program Program, progress []Progress, states map[string]bool, started, completed bool) Summary {
	s := SummaryFor(program, progress, started)
	if completed {
		s.Status = "completed"
		return s
	}
	for _, m := range learningProgram(program).Modules {
		for _, l := range m.Lessons {
			if !l.Required || CompletionBlock(l) != "" || requirementsReason(l, states) == "" {
				continue
			}
			for _, p := range progress {
				if p.LessonKey == l.Key && p.Status == "completed" {
					s.Completed--
					break
				}
			}
		}
	}
	if s.Required > 0 {
		s.Percent = s.Completed * 100 / s.Required
	}
	if s.Completed < s.Required && s.Status == "completed" {
		s.Status = "in_progress"
	}
	return s
}

// VersionDraft has its own concurrency token, also returned as Program.Revision
// for the existing editor. It never uses the legacy course-wide save counter.
type VersionDraft struct {
	VersionID int64                          `json:"version_id"`
	Revision  int64                          `json:"revision"`
	Program   Program                        `json:"program"`
	Tests     map[string]capturedAcademyTest `json:"-"`
}

type ProgramChanges struct {
	AddedModules        int  `json:"added_modules"`
	ArchivedModules     int  `json:"archived_modules"`
	ChangedModules      int  `json:"changed_modules"`
	AddedLessons        int  `json:"added_lessons"`
	ArchivedLessons     int  `json:"archived_lessons"`
	ChangedLessons      int  `json:"changed_lessons"`
	AddedTests          int  `json:"added_tests"`
	AddedAssignments    int  `json:"added_assignments"`
	RequirementsChanged bool `json:"requirements_changed"`
}

// independentProgram allocates logical identities only for a NEW version.
// Ordinary edits must retain keys. Media bytes and asset IDs are not copied;
// callers must copy the corresponding authorized asset links in their tx.
func independentProgram(source Program) (Program, map[string]string) {
	out := clone(source)
	out.Revision = 1
	out.VersionID, out.PublicationID = 0, 0
	keys := map[string]string{}
	for i := range out.Modules {
		m := &out.Modules[i]
		m.ID = 0
		keys[m.Key] = Key()
		m.Key = keys[m.Key]
		for j := range m.Lessons {
			l := &m.Lessons[j]
			l.ID = 0
			keys[l.Key] = Key()
			l.Key = keys[l.Key]
			for k := range l.Blocks {
				b := &l.Blocks[k]
				b.ID = 0
				keys[b.Key] = Key()
				b.Key = keys[b.Key]
			}
		}
	}
	return out, keys
}

func programChanges(before, after Program) ProgramChanges {
	oldLessons, newLessons := activeLessons(before), activeLessons(after)
	out := ProgramChanges{}
	oldModules, newModules := map[string]Module{}, map[string]Module{}
	for _, m := range before.Modules {
		if m.Status == "published" {
			oldModules[m.Key] = m
		}
	}
	for _, m := range after.Modules {
		if m.Status == "published" {
			newModules[m.Key] = m
		}
	}
	for key, m := range newModules {
		old, ok := oldModules[key]
		if !ok {
			out.AddedModules++
			continue
		}
		old.ID, m.ID = 0, 0
		old.Lessons, m.Lessons = nil, nil
		if !reflect.DeepEqual(old, m) {
			out.ChangedModules++
		}
	}
	for key := range oldModules {
		if _, ok := newModules[key]; !ok {
			out.ArchivedModules++
		}
	}
	for key, l := range newLessons {
		old, exists := oldLessons[key]
		if !exists {
			out.AddedLessons++
			out.RequirementsChanged = out.RequirementsChanged || l.Required
		} else {
			if !reflect.DeepEqual(lessonContent(old), lessonContent(l)) {
				out.ChangedLessons++
			}
			out.RequirementsChanged = out.RequirementsChanged || !reflect.DeepEqual(lessonRequirements(old), lessonRequirements(l))
		}
		blocks := map[string]string{}
		for _, b := range old.Blocks {
			blocks[b.Key] = b.Type
		}
		for _, b := range l.Blocks {
			if blocks[b.Key] != b.Type {
				switch b.Type {
				case "test":
					out.AddedTests++
				case "assignment":
					out.AddedAssignments++
				}
			}
		}
	}
	for key, l := range oldLessons {
		if _, ok := newLessons[key]; !ok {
			out.ArchivedLessons++
			out.RequirementsChanged = out.RequirementsChanged || l.Required
		}
	}
	return out
}

func activeLessons(p Program) map[string]Lesson {
	out := map[string]Lesson{}
	for _, m := range p.Modules {
		if m.Status == "published" {
			for _, l := range m.Lessons {
				if l.Status == "published" {
					out[l.Key] = l
				}
			}
		}
	}
	return out
}

func lessonContent(l Lesson) Lesson {
	l = clone(l)
	l.ID = 0
	for i := range l.Blocks {
		l.Blocks[i].ID = 0
	}
	return l
}

// Ignore wording, media, ordering and optional blocks when deciding whether a
// publication confirmation needs to warn about completion requirements.
func lessonRequirements(l Lesson) any {
	type requirement struct {
		Kind string
		Data AssessmentPolicy
		Test int64
	}
	blocks := map[string]requirement{}
	if l.Required {
		for _, b := range l.Blocks {
			if requiredBlock(l, b) {
				p := clone(policy(b))
				p.ShowCorrect = false
				p.ShowExplanations = false
				for i := range p.Criteria {
					p.Criteria[i].Title = ""
				}
				blocks[b.Key] = requirement{b.Type, p, b.Data.TestVersionID}
			}
		}
	}
	return struct {
		Required  bool
		SelfPaced bool
		Rule      string
		Blocks    map[string]requirement
	}{l.Required, l.SelfPaced, l.CompletionRule, blocks}
}
