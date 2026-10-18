package academy

import (
	"fmt"
	"sort"
	"strconv"
	"time"
)

type analyticsRow map[string]any

func (d *analyticsDataset) overview() map[string]any {
	users, active := map[int64]bool{}, map[int64]bool{}
	started, finished, pending := 0, 0, 0
	progress := []float64{}
	funnel := []int{len(d.Students), 0, 0, 0, 0, 0}
	for _, s := range d.Students {
		users[s.User] = true
		if s.Started != nil {
			started++
			funnel[1]++
		}
		if s.Summary.Status == "completed" {
			finished++
			funnel[5]++
		}
		if s.Progress != nil {
			progress = append(progress, *s.Progress)
			for i, t := range []float64{25, 50, 75} {
				if *s.Progress >= t {
					funnel[i+2]++
				}
			}
		}
		pending += s.Pending
	}
	for _, e := range d.Events {
		if s := d.ByID[e.Enrollment]; s != nil && e.User == s.User && d.Filters.in(e.At) {
			active[s.User] = true
		}
	}
	completion := ratio(finished, len(d.Students))
	if len(progress) == 0 {
		completion = nil
	}
	return map[string]any{"enrollments": len(d.Students), "unique_students": len(users), "active_students": len(active), "started": started, "completed": finished, "average_progress": mean(progress), "completion_rate": completion, "pending": pending, "funnel": funnel, "activity": d.activity(), "risk_students": d.riskRows(8)}
}
func (d *analyticsDataset) activity() []analyticsRow {
	out := []analyticsRow{}
	if len(d.Students) == 0 {
		return out
	}
	loc := mustLocation(d.Filters.Timezone)
	from := d.Filters.from
	if d.Filters.Period == "all" {
		from = d.Students[0].Enrolled
		for _, s := range d.Students {
			if s.Enrolled.Before(from) {
				from = s.Enrolled
			}
		}
	}
	until := d.Filters.until
	if until.After(d.Now) {
		until = d.Now
	}
	days := int(until.Sub(from).Hours() / 24)
	unit := "day"
	if days > 730 {
		unit = "month"
	} else if days > 90 {
		unit = "week"
	}
	bucket := func(t time.Time) time.Time {
		t = t.In(loc)
		day := t.Day()
		if unit == "month" {
			day = 1
		}
		b := time.Date(t.Year(), t.Month(), day, 0, 0, 0, 0, loc)
		if unit == "week" {
			n := (int(b.Weekday()) + 6) % 7
			b = b.AddDate(0, 0, -n)
		}
		return b
	}
	counts := map[string]map[int64]bool{}
	for _, e := range d.Events {
		if s := d.ByID[e.Enrollment]; s != nil && e.User == s.User && d.Filters.in(e.At) {
			key := bucket(e.At).Format("2006-01-02")
			if counts[key] == nil {
				counts[key] = map[int64]bool{}
			}
			counts[key][s.User] = true
		}
	}
	for t := bucket(from); !t.After(until.In(loc)) && len(out) < 400; {
		key := t.Format("2006-01-02")
		out = append(out, analyticsRow{"date": key, "active": len(counts[key]), "unit": unit})
		switch unit {
		case "week":
			t = t.AddDate(0, 0, 7)
		case "month":
			t = t.AddDate(0, 1, 0)
		default:
			t = t.AddDate(0, 0, 1)
		}
	}
	return out
}
func (d *analyticsDataset) riskRows(limit int) []*AnalyticsStudent {
	out := []*AnalyticsStudent{}
	for _, s := range d.Students {
		if len(s.Risk.Reasons) > 0 {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i].Risk.Reasons) != len(out[j].Risk.Reasons) {
			return len(out[i].Risk.Reasons) > len(out[j].Risk.Reasons)
		}
		return out[i].ID < out[j].ID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (d *analyticsDataset) lessonRows(detail string) []analyticsRow {
	out := []analyticsRow{}
	versions := []int64{}
	for v := range d.Programs {
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	for _, vid := range versions {
		all := publishedLessons(d.Programs[vid], true)
		self := publishedLessons(d.Programs[vid], false)
		nextKeys := [2]map[string][2]string{{}, {}}
		independent := map[string]bool{}
		for mode, lessons := range [2][]struct {
			Module, ModuleTitle string
			Lesson              Lesson
		}{self, all} {
			for i, l := range lessons {
				if mode == 0 {
					independent[l.Lesson.Key] = true
				}
				if i+1 < len(lessons) {
					n := lessons[i+1]
					nextKeys[mode][l.Lesson.Key] = [2]string{n.Module, n.Lesson.Key}
				}
			}
		}
		for index, l := range all {
			if detail != "" && l.Lesson.Key != detail {
				continue
			}
			available, started, completed, stopped, eligible, nextStarted := 0, 0, 0, 0, 0, 0
			elapsed := []float64{}
			students := []analyticsRow{}
			daily := map[string]int{}
			for _, s := range d.Students {
				if s.Version != vid || s.CohortID == 0 && !independent[l.Lesson.Key] {
					continue
				}
				access := availability(d.Cohorts[s.CohortID], l.Module, l.Lesson.Key, d.Now).Available && s.Access == "active"
				if access {
					available++
				}
				p, ok := d.progress[s.ID][l.Lesson.Key]
				if ok {
					started++
				}
				done := ok && p.Status == "completed" && CompletionBlock(l.Lesson) == ""
				if done {
					completed++
					if p.Completed != nil && d.Filters.in(*p.Completed) {
						daily[p.Completed.In(mustLocation(d.Filters.Timezone)).Format("2006-01-02")]++
					}
					if begin, has := d.opens[s.ID][l.Lesson.Key]; has && p.Completed != nil && d.Filters.in(*p.Completed) && !p.Completed.Before(begin) {
						elapsed = append(elapsed, p.Completed.Sub(begin).Hours())
					}
				}
				if access && ok && !done && s.LastLesson == l.Lesson.Key && s.Summary.Status != "completed" && s.Risk.InactiveDays >= d.Settings.InactiveDays && d.Now.Sub(p.Last) >= time.Duration(d.Settings.InactiveDays)*24*time.Hour {
					stopped++
					if detail != "" {
						students = append(students, analyticsRow{"enrollment_id": s.ID, "name": s.Name, "last_activity_at": s.Activity, "risk": s.Risk})
					}
				}
				mode := 0
				if s.CohortID != 0 {
					mode = 1
				}
				if next, ok := nextKeys[mode][l.Lesson.Key]; ok && done && s.Access == "active" && availability(d.Cohorts[s.CohortID], next[0], next[1], d.Now).Available {
					eligible++
					if _, has := d.progress[s.ID][next[1]]; has {
						nextStarted++
					}
				}
			}
			a := []analyticsRow{}
			for _, v := range d.Assessments {
				if v.Version == vid && v.Lesson == l.Lesson.Key {
					a = append(a, analyticsRow{"id": v.ID, "title": v.Title, "kind": v.Kind})
				}
			}
			dynamics := []analyticsRow{}
			for date, n := range daily {
				dynamics = append(dynamics, analyticsRow{"date": date, "completed": n})
			}
			sort.Slice(dynamics, func(i, j int) bool { return dynamics[i]["date"].(string) < dynamics[j]["date"].(string) })
			out = append(out, analyticsRow{"key": l.Lesson.Key, "module": l.Module, "module_title": l.ModuleTitle, "title": l.Lesson.Title, "version_id": vid, "version_number": d.VersionNumbers[vid], "order": index + 1, "available": available, "started": started, "completed": completed, "completion_rate": ratio(completed, started), "elapsed_hours": mean(elapsed), "elapsed_samples": len(elapsed), "stopped": stopped, "transition_eligible": eligible, "transitioned": nextStarted, "transition_rate": ratio(nextStarted, eligible), "students": students, "assessments": a, "dynamics": dynamics})
		}
	}
	return out
}
func (d *analyticsDataset) moduleRows() []analyticsRow {
	out := []analyticsRow{}
	for vid, p := range d.Programs {
		mods := []Module{}
		for _, m := range p.Modules {
			if m.Status == "published" && len(m.Lessons) > 0 {
				mods = append(mods, m)
			}
		}
		for index, m := range mods {
			started, completed, eligible, nextStarted := 0, 0, 0, 0
			for _, s := range d.Students {
				if s.Version != vid {
					continue
				}
				begin, done, has := false, true, false
				required := false
				for _, l := range m.Lessons {
					if l.Status == "published" && l.Required {
						required = true
					}
				}
				for _, l := range m.Lessons {
					if l.Status != "published" || s.CohortID == 0 && !l.SelfPaced {
						continue
					}
					has = true
					p, ok := d.progress[s.ID][l.Key]
					begin = begin || ok
					if s.CohortID != 0 {
						l.SelfPaced = true
					}
					if (!required || l.Required) && (!ok || p.Status != "completed" || CompletionBlock(l) != "") {
						done = false
					}
				}
				if begin {
					started++
				}
				if done && has {
					completed++
				}
				if done && has && index+1 < len(mods) {
					next := mods[index+1]
					open, nextBegin := false, false
					for _, l := range next.Lessons {
						if l.Status != "published" || s.CohortID == 0 && !l.SelfPaced {
							continue
						}
						open = open || availability(d.Cohorts[s.CohortID], next.Key, l.Key, d.Now).Available
						_, has := d.progress[s.ID][l.Key]
						nextBegin = nextBegin || has
					}
					if open {
						eligible++
						if nextBegin {
							nextStarted++
						}
					}
				}
			}
			out = append(out, analyticsRow{"key": m.Key, "title": m.Title, "version_id": vid, "version_number": d.VersionNumbers[vid], "order": index + 1, "started": started, "completed": completed, "completion_rate": ratio(completed, started), "transition_eligible": eligible, "transitioned": nextStarted, "transition_rate": ratio(nextStarted, eligible)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i]["version_id"] != out[j]["version_id"] {
			return out[i]["version_id"].(int64) < out[j]["version_id"].(int64)
		}
		return out[i]["order"].(int) < out[j]["order"].(int)
	})
	return out
}

func (d *analyticsDataset) testRows(detail int64) []analyticsRow {
	out := []analyticsRow{}
	for _, a := range d.Assessments {
		if a.Kind != "test" || detail > 0 && a.ID != detail {
			continue
		}
		scores := []float64{}
		users, enrolls := map[int64]bool{}, map[int64]bool{}
		passed, started := 0, 0
		bins := [5]int{}
		firstSuccess := map[int64]analyticsTest{}
		questionStats := map[int64][3]int{}
		for _, t := range d.Tests {
			student := d.ByID[t.Enrollment]
			if student == nil || t.Assessment != a.ID {
				continue
			}
			if d.Filters.in(t.Started) {
				started++
			}
			if t.Status != "finished" || t.Finished == nil {
				continue
			}
			if t.Passed {
				old, ok := firstSuccess[t.Enrollment]
				if !ok || t.Number < old.Number {
					firstSuccess[t.Enrollment] = t
				}
			}
			if !d.Filters.in(*t.Finished) {
				continue
			}
			users[student.User] = true
			enrolls[student.ID] = true
			scores = append(scores, t.Percent)
			if t.Passed {
				passed++
			}
			bin := min(4, int(t.Percent/20))
			bins[bin]++
			answered := map[int64]bool{}
			for _, id := range t.Answered {
				answered[id] = true
			}
			for _, g := range t.Grades {
				v := questionStats[g.Question]
				v[0]++
				if answered[g.Question] {
					v[1]++
				}
				if g.Correct != nil && *g.Correct {
					v[2]++
				}
				questionStats[g.Question] = v
			}
		}
		untilSuccess := []float64{}
		for _, t := range firstSuccess {
			if t.Finished != nil && d.Filters.in(*t.Finished) {
				untilSuccess = append(untilSuccess, float64(t.Number))
			}
		}
		questions := []analyticsRow{}
		if detail > 0 {
			for i, q := range a.Questions {
				v := questionStats[q.ID]
				questions = append(questions, analyticsRow{"id": q.ID, "number": i + 1, "text": q.Text, "graded": v[0], "answers": v[1], "unanswered": v[0] - v[1], "correct": v[2], "correct_rate": ratio(v[2], v[0]), "error_rate": ratio(v[0]-v[2], v[0])})
			}
		}
		out = append(out, analyticsRow{"id": a.ID, "title": a.Title, "lesson_key": a.Lesson, "version_id": a.Version, "version_number": d.VersionNumbers[a.Version], "attempts": len(scores), "started_attempts": started, "participants": len(users), "enrollments": len(enrolls), "average": mean(scores), "median": median(scores), "passed": passed, "success_rate": ratio(passed, len(scores)), "attempts_to_success": mean(untilSuccess), "distribution": bins, "questions": questions})
	}
	return out
}
func (d *analyticsDataset) assignmentRows(detail int64) []analyticsRow {
	out := []analyticsRow{}
	for _, a := range d.Assessments {
		if a.Kind != "assignment" || detail > 0 && a.ID != detail {
			continue
		}
		received, overdue := 0, 0
		submitted, accepted, revision := map[int64]bool{}, map[int64]bool{}, map[int64]bool{}
		attempts := 0
		scores, waits := []float64{}, []float64{}
		byID := map[int64]analyticsSubmission{}
		latest := map[int64]analyticsSubmission{}
		daily := map[string]int{}
		bins := [5]int{}
		criteria := map[string][]float64{}
		for _, s := range d.Submissions {
			if d.ByID[s.Enrollment] == nil || s.Assessment != a.ID {
				continue
			}
			byID[s.ID] = s
			if s.Number > latest[s.Enrollment].Number {
				latest[s.Enrollment] = s
			}
			if d.Filters.in(s.Submitted) {
				submitted[s.Enrollment] = true
				attempts++
				daily[s.Submitted.In(mustLocation(d.Filters.Timezone)).Format("2006-01-02")]++
			}
			if s.Status == "accepted" && s.Reviewed != nil && d.Filters.in(*s.Reviewed) {
				accepted[s.Enrollment] = true
			}
			if s.Score != nil && s.Reviewed != nil && d.Filters.in(*s.Reviewed) {
				scores = append(scores, *s.Score)
				maxScore := policy(Block{Type: "assignment", Data: a.Config}).MaxScore
				if maxScore > 0 {
					bins[min(4, int(*s.Score/maxScore*5))]++
				}
			}
		}
		revisionCount, reviewed := 0, 0
		for _, r := range d.Reviews {
			if sub, ok := byID[r.Submission]; ok && d.Filters.in(r.At) && r.Action != "in_review" {
				reviewed++
				if !r.At.Before(sub.Submitted) {
					waits = append(waits, r.At.Sub(sub.Submitted).Hours())
				}
				if r.Action == "revision_requested" {
					revision[sub.Enrollment] = true
					revisionCount++
				}
				for key, v := range r.Criteria {
					criteria[key] = append(criteria[key], v)
				}
			}
		}
		for _, s := range d.Students {
			if s.Version != a.Version {
				continue
			}
			module := ""
			selfPaced := false
			for _, l := range publishedLessons(d.Programs[s.Version], s.CohortID != 0) {
				if l.Lesson.Key == a.Lesson {
					module = l.Module
					selfPaced = l.Lesson.SelfPaced
					break
				}
			}
			if module != "" && selfPaced && s.Access == "active" && availability(d.Cohorts[s.CohortID], module, a.Lesson, d.Now).Available {
				received++
			}
			if at, _ := assignmentDeadline(d.Cohorts[s.CohortID], a.Block); module != "" && availability(d.Cohorts[s.CohortID], module, a.Lesson, d.Now).Available && at != nil && at.Before(d.Now) && !oneOf(latest[s.ID].Status, "submitted", "in_review", "accepted") {
				overdue++
			}
		}
		crows := []analyticsRow{}
		for _, c := range policy(Block{Type: "assignment", Data: a.Config}).Criteria {
			values := criteria[c.Key]
			sumShort := 0
			for _, v := range values {
				if v < c.MaxScore {
					sumShort++
				}
			}
			crows = append(crows, analyticsRow{"key": c.Key, "title": c.Title, "max_score": c.MaxScore, "average": mean(values), "graded": len(values), "below_max": sumShort, "below_max_rate": ratio(sumShort, len(values))})
		}
		dynamics := []analyticsRow{}
		for date, n := range daily {
			dynamics = append(dynamics, analyticsRow{"date": date, "submitted": n})
		}
		sort.Slice(dynamics, func(i, j int) bool { return dynamics[i]["date"].(string) < dynamics[j]["date"].(string) })
		out = append(out, analyticsRow{"id": a.ID, "title": a.Title, "lesson_key": a.Lesson, "version_id": a.Version, "version_number": d.VersionNumbers[a.Version], "available": received, "submitted": len(submitted), "accepted": len(accepted), "revised": len(revision), "revision_decisions": revisionCount, "reviewed": reviewed, "revision_rate": ratio(revisionCount, reviewed), "overdue": overdue, "average_score": mean(scores), "average_attempts": func() *float64 {
			if len(submitted) == 0 {
				return nil
			}
			v := float64(attempts) / float64(len(submitted))
			return &v
		}(), "review_hours": mean(waits), "review_samples": len(waits), "distribution": bins, "criteria": crows, "dynamics": dynamics})
	}
	return out
}

func (d *analyticsDataset) cohortRows() []analyticsRow {
	out := []analyticsRow{}
	// Decisions are append-only; this permits reconstructing the queue at the comparison boundary.
	reviews := map[int64][]analyticsReview{}
	for _, r := range d.Reviews {
		reviews[r.Submission] = append(reviews[r.Submission], r)
	}
	for id, c := range d.Cohorts {
		begin, end := d.Filters.from, d.Filters.until
		relative := d.Filters.Compare == "relative"
		mature := true
		if relative {
			begin = c.StartsAt
			end = begin.In(mustLocation(c.Timezone)).AddDate(0, 0, d.Filters.Days)
			mature = !d.Now.Before(end)
		}
		boundary := end
		if boundary.After(d.Now) {
			boundary = d.Now
		}
		total, finished, pending, overdue, eligible := 0, 0, 0, 0, 0
		values, testScores := []float64{}, []float64{}
		included, seenActive := map[int64]bool{}, map[int64]bool{}
		for _, student := range d.Students {
			if student.CohortID != id || relative && !student.Enrolled.Before(boundary) {
				continue
			}
			s := student
			total++
			included[s.ID] = true
			if !relative {
				pending += s.Pending
				overdue += s.Risk.Overdue
				if s.Progress != nil {
					values = append(values, *s.Progress)
					eligible++
				}
				if s.Summary.Status == "completed" {
					finished++
				}
			} else {
				required, done := 0, 0
				for _, l := range publishedLessons(d.Programs[s.Version], true) {
					if !l.Lesson.Required {
						continue
					}
					required++
					if at, ok := d.completions[s.ID][l.Lesson.Key]; ok && !at.Before(begin) && at.Before(boundary) {
						done++
					}
				}
				if required > 0 {
					eligible++
					values = append(values, float64(done)/float64(required)*100)
					if required == done {
						finished++
					}
				}
			}
		}
		for _, e := range d.Events {
			if s := d.ByID[e.Enrollment]; s != nil && included[s.ID] && e.User == s.User && !e.At.Before(begin) && e.At.Before(boundary) {
				seenActive[s.User] = true
			}
		}
		for _, t := range d.Tests {
			if included[t.Enrollment] && t.Finished != nil && !t.Finished.Before(begin) && t.Finished.Before(boundary) {
				testScores = append(testScores, t.Percent)
			}
		}
		submitted, accepted := 0, 0
		latest := map[[2]int64]analyticsSubmission{}
		for _, sub := range d.Submissions {
			if !included[sub.Enrollment] || !sub.Submitted.Before(boundary) {
				continue
			}
			state := "submitted"
			var latestDecision time.Time
			for _, r := range reviews[sub.ID] {
				if r.At.Before(boundary) && !r.At.Before(latestDecision) {
					state = r.Action
					latestDecision = r.At
				}
			}
			// Legacy rows without an audit still have an explicit review timestamp.
			if len(reviews[sub.ID]) == 0 && sub.Reviewed != nil && sub.Reviewed.Before(boundary) {
				state = sub.Status
			}
			if !sub.Submitted.Before(begin) {
				submitted++
				if state == "accepted" {
					accepted++
				}
			}
			if relative {
				sub.Status = state
				key := [2]int64{sub.Enrollment, sub.Assessment}
				if sub.Number > latest[key].Number {
					latest[key] = sub
				}
			}
		}
		if relative {
			for _, sub := range latest {
				if oneOf(sub.Status, "submitted", "in_review") {
					pending++
				}
			}
		}
		warnings := []string{}
		var overdueValue any = overdue
		if relative {
			overdueValue = nil
			warnings = append(warnings, "Просрочки на историческую дату не рассчитываются: расписание и дедлайны могли изменяться")
			if !mature {
				warnings = append(warnings, "Поток ещё не достиг конца выбранного интервала")
			}
		}
		if c.Status == "draft" || c.Status == "scheduled" {
			warnings = append(warnings, "Поток не запущен")
		}
		out = append(out, analyticsRow{"id": id, "title": c.Name, "status": c.Status, "starts_at": c.StartsAt, "version_id": c.VersionID, "version_number": c.VersionNumber, "enrollments": total, "active": len(seenActive), "completed": finished, "average_progress": mean(values), "completion_rate": ratio(finished, eligible), "test_average": mean(testScores), "assignment_acceptance": ratio(accepted, submitted), "pending": pending, "overdue": overdueValue, "window_start": begin, "window_end": end, "mature": mature, "warnings": warnings})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["id"].(int64) < out[j]["id"].(int64) })
	return out
}
func (d *analyticsDataset) staffRows() []analyticsRow {
	out := []analyticsRow{}
	names := map[int64]string{}
	roles := map[int64]string{}
	for _, s := range d.Staff {
		if d.Owner || s.ID == d.Actor {
			names[s.ID] = s.Name
			roles[s.ID] = s.Role
		}
	}
	for _, r := range d.Reviews {
		if d.Owner || r.Reviewer == d.Actor {
			names[r.Reviewer] = r.Name
		}
	}
	for uid, name := range names {
		assigned, pending, attention, checked, revised := 0, 0, 0, 0, 0
		waits := []float64{}
		responsible := func(s *AnalyticsStudent, review bool) bool {
			if s.CuratorID == uid && !review {
				return true
			}
			for _, member := range d.Staff {
				if member.ID == uid && (member.Cohort == s.CohortID || member.Role == "owner") && (!review || member.CanReview) && (member.Role != "curator" || s.CuratorID == uid) {
					return true
				}
			}
			return false
		}
		for _, s := range d.Students {
			if responsible(s, false) {
				assigned++
				if len(s.Risk.Reasons) > 0 {
					attention++
				}
			}
		}
		subs := map[int64]analyticsSubmission{}
		for _, s := range d.Submissions {
			if d.ByID[s.Enrollment] == nil {
				continue
			}
			subs[s.ID] = s
			if oneOf(s.Status, "submitted", "in_review") && (s.Assigned == uid || s.Assigned == 0 && responsible(d.ByID[s.Enrollment], true)) {
				pending++
			}
		}
		for _, r := range d.Reviews {
			if r.Reviewer != uid || r.Action == "in_review" || !d.Filters.in(r.At) {
				continue
			}
			s, ok := subs[r.Submission]
			if !ok {
				continue
			}
			checked++
			if r.Action == "revision_requested" {
				revised++
			}
			if !r.At.Before(s.Submitted) {
				waits = append(waits, r.At.Sub(s.Submitted).Hours())
			}
		}
		out = append(out, analyticsRow{"id": uid, "name": name, "role": roles[uid], "assigned": assigned, "reviewed": checked, "review_hours": mean(waits), "pending": pending, "revised": revised, "attention": attention})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["id"].(int64) < out[j]["id"].(int64) })
	return out
}
func (d *analyticsDataset) communications() map[string]any {
	rows := []analyticsRow{}
	counts := map[string]int{"queued": 0, "sending": 0, "sent": 0, "delivered": 0, "failed": 0, "cancelled": 0}
	campaigns := map[int64]bool{}
	groups := map[string][]analyticsDelivery{}
	allSent := map[int64][]analyticsDelivery{}
	events := map[int64][]time.Time{}
	enqueued, smtpSent, inAppSent, confirmed := 0, 0, 0, 0
	for _, ev := range d.Events {
		if student := d.ByID[ev.Enrollment]; student != nil && ev.User == student.User {
			events[student.User] = append(events[student.User], ev.At)
		}
	}
	for uid := range events {
		sort.Slice(events[uid], func(i, j int) bool { return events[uid][i].Before(events[uid][j]) })
	}
	for _, m := range d.Deliveries {
		student := d.ByID[m.Enrollment]
		if student == nil {
			continue
		}
		sent := m.Sent != nil && oneOf(m.Status, "sent", "delivered")
		if sent && m.Channel == "email" && student.User == m.User {
			allSent[m.User] = append(allSent[m.User], m)
		}
		queuedIn := d.Filters.in(m.Queued)
		sentIn := sent && d.Filters.in(*m.Sent)
		if queuedIn {
			enqueued++
			counts[m.Status]++
		}
		if sentIn {
			if m.Channel == "email" {
				smtpSent++
			} else {
				inAppSent++
			}
			if m.Status == "delivered" {
				confirmed++
			}
		}
		if !queuedIn && !sentIn {
			continue
		}
		if m.Campaign > 0 {
			campaigns[m.Campaign] = true
		}
		key := "campaign:" + strconv.FormatInt(m.Campaign, 10)
		if m.Campaign == 0 {
			key = "automation:" + strconv.FormatInt(m.Automation, 10)
		}
		groups[key] = append(groups[key], m)
	}
	for uid := range allSent {
		sort.Slice(allSent[uid], func(i, j int) bool { return allSent[uid][i].Sent.Before(*allSent[uid][j].Sent) })
	}
	for key, messages := range groups {
		eligible, observed, returned := map[int64]bool{}, map[int64]bool{}, map[int64]bool{}
		sent, failed, truncated := 0, 0, 0
		for _, m := range messages {
			if m.Status == "failed" {
				failed++
			}
			if m.Sent == nil || !oneOf(m.Status, "sent", "delivered") || m.Channel != "email" || !d.Filters.in(*m.Sent) || d.ByID[m.Enrollment].User != m.User {
				continue
			}
			sent++
			eligible[m.User] = true
			end := m.Sent.Add(time.Duration(d.Settings.ReturnDays) * 24 * time.Hour)
			next := allSent[m.User]
			n := sort.Search(len(next), func(i int) bool { return next[i].Sent.After(*m.Sent) })
			if n < len(next) && next[n].Sent.Before(end) {
				end = *next[n].Sent
				truncated++
			}
			if end.After(d.Now) {
				continue
			}
			observed[m.User] = true
			activity := events[m.User]
			n = sort.Search(len(activity), func(i int) bool { return activity[i].After(*m.Sent) })
			if n < len(activity) && activity[n].Before(end) {
				returned[m.User] = true
			}
		}
		rows = append(rows, analyticsRow{"key": key, "name": messages[0].Name, "messages": len(messages), "smtp_sent": sent, "failed": failed, "recipients": len(eligible), "observed": len(observed), "returned": len(returned), "return_rate": ratio(len(returned), len(observed)), "overlap_windows": truncated})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["key"].(string) < rows[j]["key"].(string) })
	var delivered *int
	if confirmed > 0 {
		delivered = &confirmed
	}
	return map[string]any{"campaigns": len(campaigns), "enqueued": enqueued, "smtp_sent": smtpSent, "in_app_sent": inAppSent, "counts": counts, "confirmed_deliveries": delivered, "automation_runs": d.Runs, "return_days": d.Settings.ReturnDays, "rows": rows, "attribution": "Активность после рассылки — наблюдаемая связь, не доказательство причинности. Повторное письмо завершает окно предыдущего; незавершённые окна исключены из доли."}
}
func (d *analyticsDataset) studentContext(id int64) (map[string]any, error) {
	s := d.ByID[id]
	if s == nil {
		return nil, fmt.Errorf("%w", ErrAccess)
	}
	deadlines := []analyticsRow{}
	unfinished := []analyticsRow{}
	for _, a := range d.Assessments {
		if a.Version != s.Version || a.Kind != "assignment" {
			continue
		}
		done := false
		for _, sub := range d.Submissions {
			if sub.Enrollment == id && sub.Assessment == a.ID && sub.Status == "accepted" {
				done = true
			}
		}
		if !done {
			unfinished = append(unfinished, analyticsRow{"id": a.ID, "title": a.Title, "lesson_key": a.Lesson})
			if at, _ := assignmentDeadline(d.Cohorts[s.CohortID], a.Block); at != nil {
				deadlines = append(deadlines, analyticsRow{"title": a.Title, "at": at, "overdue": at.Before(d.Now)})
			}
		}
	}
	tests := []analyticsRow{}
	for _, t := range d.Tests {
		if t.Enrollment == id {
			var percent, passed any
			if t.Status == "finished" {
				percent, passed = t.Percent, t.Passed
			}
			tests = append(tests, analyticsRow{"id": t.ID, "assessment_id": t.Assessment, "number": t.Number, "percent": percent, "passed": passed, "finished_at": t.Finished, "status": t.Status})
		}
	}
	history := []analyticsRow{}
	for _, m := range d.Deliveries {
		if m.Enrollment == id {
			history = append(history, analyticsRow{"id": m.ID, "channel": m.Channel, "status": m.Status, "sent_at": m.Sent})
		}
	}
	sort.Slice(deadlines, func(i, j int) bool { return deadlines[i]["at"].(*time.Time).Before(*deadlines[j]["at"].(*time.Time)) })
	testTotal, communicationTotal := len(tests), len(history)
	if len(tests) > 100 {
		tests = tests[len(tests)-100:]
	}
	if len(history) > 100 {
		history = history[len(history)-100:]
	}
	return map[string]any{"student": s, "tests": tests, "test_total": testTotal, "unfinished_assignments": unfinished, "deadlines": deadlines, "communications": history, "communication_total": communicationTotal, "communications_available": d.Communications, "generated_at": d.Now}, nil
}
