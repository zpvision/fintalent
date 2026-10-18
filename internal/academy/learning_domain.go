package academy

import (
	"fmt"
	"time"
)

type Readiness struct {
	Blockers []string `json:"blockers"`
	Warnings []string `json:"warnings"`
	Lessons  int      `json:"lessons"`
	Required int      `json:"required"`
}

func CompletionBlock(l Lesson) string {
	if !l.SelfPaced {
		return "Урок не предназначен для самостоятельного прохождения"
	}
	if l.CompletionRule == "test" || l.CompletionRule == "assignment" {
		return "Завершение по тесту или принятому заданию появится на Этапе C"
	}
	for _, b := range l.Blocks {
		if b.Type == "test" || b.Type == "assignment" {
			return "Интерактивный тест или задание пока нельзя выполнить. Урок не будет засчитан фиктивно"
		}
		if l.CompletionRule == "materials" && b.Type == "video" {
			return "Автоматическая проверка просмотра видео пока не поддерживается. Урок не будет завершён по факту открытия"
		}
	}
	return ""
}
func CheckReadiness(p Program) Readiness {
	r := Readiness{Blockers: []string{}, Warnings: []string{}}
	for _, m := range p.Modules {
		if m.Status != "published" {
			if len(m.Lessons) > 0 {
				r.Warnings = append(r.Warnings, fmt.Sprintf("Модуль «%s» — черновик, он не войдёт в обучение", m.Title))
			}
			continue
		}
		for _, l := range m.Lessons {
			if l.Status != "published" {
				if l.Required {
					r.Blockers = append(r.Blockers, fmt.Sprintf("Обязательный урок «%s» не опубликован", l.Title))
				}
				continue
			}
			r.Lessons++
			if l.Required {
				r.Required++
				if len(l.Blocks) == 0 && l.Description == "" {
					r.Blockers = append(r.Blockers, fmt.Sprintf("Добавьте содержимое обязательного урока «%s»", l.Title))
				}
				for _, b := range l.Blocks {
					if (oneOf(b.Type, "image", "file") && b.Data.AssetID == 0) || (oneOf(b.Type, "video", "link") && b.Data.URL == "") {
						r.Blockers = append(r.Blockers, fmt.Sprintf("В уроке «%s» не заполнен блок материалов", l.Title))
					}
				}
				if !l.SelfPaced {
					r.Blockers = append(r.Blockers, fmt.Sprintf("Обязательный урок «%s» недоступен для самостоятельного обучения", l.Title))
				}
				if reason := CompletionBlock(l); reason != "" {
					r.Warnings = append(r.Warnings, fmt.Sprintf("%s: %s", l.Title, reason))
				}
			}
		}
	}
	if r.Lessons == 0 {
		r.Blockers = append(r.Blockers, "Опубликуйте хотя бы один модуль и урок в редакторе, затем зафиксируйте новую версию")
	}
	if r.Required == 0 {
		r.Warnings = append(r.Warnings, "Нет обязательных уроков: процент будет 0, автоматического завершения курса не будет")
	}
	return r
}

type Progress struct {
	LessonKey   string     `json:"lesson_key"`
	Status      string     `json:"status"`
	CompletedAt *time.Time `json:"completed_at"`
}
type Summary struct {
	Required         int    `json:"required"`
	Completed        int    `json:"completed"`
	TotalLessons     int    `json:"total_lessons"`
	CompletedLessons int    `json:"completed_lessons"`
	Percent          int    `json:"percent"`
	Status           string `json:"status"`
}

func SummaryFor(p Program, progress []Progress, started bool) Summary {
	states := map[string]bool{}
	for _, v := range progress {
		states[v.LessonKey] = v.Status == "completed"
	}
	s := Summary{Status: "not_started"}
	if started {
		s.Status = "in_progress"
	}
	for _, m := range p.Modules {
		if m.Status != "published" {
			continue
		}
		for _, l := range m.Lessons {
			if l.Status != "published" {
				continue
			}
			s.TotalLessons++
			done := states[l.Key] && CompletionBlock(l) == ""
			if done {
				s.CompletedLessons++
			}
			if l.Required {
				s.Required++
				if done {
					s.Completed++
				}
			}
		}
	}
	if s.Required > 0 {
		s.Percent = s.Completed * 100 / s.Required
		if s.Completed == s.Required {
			s.Status = "completed"
		}
	}
	return s
}
func learningProgram(p Program) Program {
	out := p
	out.Modules = []Module{}
	for _, m := range p.Modules {
		if m.Status != "published" {
			continue
		}
		copy := m
		copy.Lessons = []Lesson{}
		for _, l := range m.Lessons {
			if l.Status == "published" {
				copy.Lessons = append(copy.Lessons, l)
			}
		}
		out.Modules = append(out.Modules, copy)
	}
	return out
}
func findLesson(p Program, key string) (Lesson, bool) {
	for _, m := range p.Modules {
		if m.Status != "published" {
			continue
		}
		for _, l := range m.Lessons {
			if l.Key == key && l.Status == "published" && l.SelfPaced {
				return l, true
			}
		}
	}
	return Lesson{}, false
}
func lessonAssets(l Lesson) map[int64]bool {
	ids := map[int64]bool{}
	if l.CoverAssetID > 0 {
		ids[l.CoverAssetID] = true
	}
	for _, b := range l.Blocks {
		for _, id := range []int64{b.Data.AssetID, b.Data.PosterID} {
			if id > 0 {
				ids[id] = true
			}
		}
		for _, id := range b.Data.MaterialIDs {
			if id > 0 {
				ids[id] = true
			}
		}
	}
	return ids
}
