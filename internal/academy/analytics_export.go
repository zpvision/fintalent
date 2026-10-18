package academy

import (
	"archive/zip"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var reportKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,80}\.(csv|xlsx)$`)

func reportPath(root, key string) (string, error) {
	if !reportKeyPattern.MatchString(key) {
		return "", ErrInvalid
	}
	base, e := filepath.Abs(filepath.Join(root, "reports"))
	if e != nil {
		return "", e
	}
	p := filepath.Join(base, key)
	rel, e := filepath.Rel(base, p)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrInvalid
	}
	return p, nil
}
func safeExportCell(value string) string {
	value = strings.Map(func(r rune) rune {
		if r == 0 || r < 32 && r != '\n' && r != '\t' && r != '\r' {
			return -1
		}
		return r
	}, value)
	first := strings.TrimLeftFunc(value, func(r rune) bool { return unicode.IsSpace(r) || r == '\ufeff' })
	if len(first) > 0 && strings.ContainsRune("=+-@", rune(first[0])) {
		return "'" + value
	}
	return value
}
func excelColumn(n int) string {
	s := ""
	for n > 0 {
		n--
		s = string(rune('A'+n%26)) + s
		n /= 26
	}
	return s
}
func writeAnalyticsCSV(w io.Writer, rows [][]string) error {
	if _, e := io.WriteString(w, "\ufeff"); e != nil {
		return e
	}
	c := csv.NewWriter(w)
	c.Comma = ';'
	for _, row := range rows {
		out := make([]string, len(row))
		for i, v := range row {
			out[i] = safeExportCell(v)
		}
		if e := c.Write(out); e != nil {
			return e
		}
	}
	c.Flush()
	return c.Error()
}

// OOXML uses explicit inline text or allowlisted numeric columns; never formulas or external content.
func writeAnalyticsXLSX(w io.Writer, rows [][]string) error {
	z := zip.NewWriter(w)
	defer z.Close()
	files := map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`,
		"_rels/.rels":                `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="FinTalent Academy" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"xl/styles.xml":              `<?xml version="1.0" encoding="UTF-8"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><color rgb="FFFFFFFF"/><name val="Calibri"/></font></fonts><fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF3569E8"/><bgColor indexed="64"/></patternFill></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf/></cellStyleXfs><cellXfs count="2"><xf fontId="0" fillId="0" borderId="0" xfId="0"/><xf fontId="1" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1"/></cellXfs></styleSheet>`,
	}
	for name, body := range files {
		f, e := z.Create(name)
		if e != nil {
			return e
		}
		if _, e = io.WriteString(f, body); e != nil {
			return e
		}
	}
	sheet, e := z.Create("xl/worksheets/sheet1.xml")
	if e != nil {
		return e
	}
	if _, e = io.WriteString(sheet, `<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="30" width="24" customWidth="1"/></cols><sheetData>`); e != nil {
		return e
	}
	maxCols := 0
	for i, row := range rows {
		maxCols = max(maxCols, len(row))
		if _, e = fmt.Fprintf(sheet, `<row r="%d">`, i+1); e != nil {
			return e
		}
		for j, value := range row {
			style := ""
			if i == 0 {
				style = ` s="1"`
			}
			numeric := i > 0 && j < len(rows[0]) && analyticsNumericColumn(rows[0][j])
			if n, err := strconv.ParseFloat(value, 64); numeric && err == nil && !math.IsInf(n, 0) && !math.IsNaN(n) {
				if _, e = fmt.Fprintf(sheet, "<c r=\"%s%d\"><v>%s</v></c>", excelColumn(j+1), i+1, strconv.FormatFloat(n, 'g', -1, 64)); e != nil {
					return e
				}
				continue
			}
			if _, e = fmt.Fprintf(sheet, `<c r="%s%d" t="inlineStr"%s><is><t xml:space="preserve">`, excelColumn(j+1), i+1, style); e != nil {
				return e
			}
			if e = xml.EscapeText(sheet, []byte(safeExportCell(value))); e != nil {
				return e
			}
			if _, e = io.WriteString(sheet, "</t></is></c>"); e != nil {
				return e
			}
		}
		if _, e = io.WriteString(sheet, "</row>"); e != nil {
			return e
		}
	}
	_, e = fmt.Fprintf(sheet, `</sheetData><autoFilter ref="A1:%s%d"/></worksheet>`, excelColumn(max(1, maxCols)), max(1, len(rows)))
	if e != nil {
		return e
	}
	return z.Close()
}
func exportValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "Недостаточно данных"
	case *float64:
		if x == nil {
			return "Недостаточно данных"
		}
		return strconv.FormatFloat(*x, 'f', 2, 64)
	case *int:
		if x == nil {
			return "Недостаточно данных"
		}
		return strconv.Itoa(*x)
	default:
		return fmt.Sprint(v)
	}
}
func (d *analyticsDataset) exportRows(kind string) ([][]string, error) {
	rows := [][]string{}
	appendMap := func(data []analyticsRow, keys, labels []string) {
		rows = append(rows, labels)
		for _, r := range data {
			out := []string{}
			for _, k := range keys {
				out = append(out, exportValue(r[k]))
			}
			rows = append(rows, out)
		}
	}
	switch kind {
	case "students", "progress", "cohort", "risk":
		rows = append(rows, []string{"Зачисление", "Ученик", "ID пользователя", "Поток", "Версия", "Дата зачисления", "Прогресс, %", "Завершено уроков", "Обязательных уроков", "Ожидается по расписанию", "Средний результат тестов, %", "На проверке", "На доработке", "Последняя активность", "Риск", "Причины", "Доступ"})
		for _, s := range d.Students {
			if kind == "risk" && len(s.Risk.Reasons) == 0 {
				continue
			}
			activity := "Не начинал"
			if s.Activity != nil {
				activity = s.Activity.In(mustLocation(d.Filters.Timezone)).Format("2006-01-02 15:04:05 MST")
			}
			rows = append(rows, []string{strconv.FormatInt(s.ID, 10), s.Name, strconv.FormatInt(s.User, 10), s.CohortName, strconv.Itoa(s.VersionNumber), s.Enrolled.In(mustLocation(d.Filters.Timezone)).Format("2006-01-02 15:04:05 MST"), exportValue(s.Progress), strconv.Itoa(s.Summary.Completed), strconv.Itoa(s.Summary.Required), func() string {
				if !s.Scheduled {
					return "Без расписания"
				}
				return strconv.Itoa(s.Expected)
			}(), exportValue(s.TestAverage), strconv.Itoa(s.Pending), strconv.Itoa(s.Revisions), activity, s.Risk.Level, strings.Join(s.Risk.Reasons, "; "), s.Access})
		}
	case "tests", "assignments", "performance":
		rows = append(rows, []string{"Тип", "Зачисление", "Ученик", "Версия", "Проверка", "Название", "Попытка", "Статус", "Баллы", "Процент", "Результат", "Дата", "Проверяющий"})
		a := map[int64]analyticsAssessment{}
		for _, v := range d.Assessments {
			a[v.ID] = v
		}
		if kind != "assignments" {
			for _, t := range d.Tests {
				s := d.ByID[t.Enrollment]
				if s == nil || t.Finished == nil || !d.Filters.in(*t.Finished) {
					continue
				}
				rows = append(rows, []string{"Тест", fmt.Sprint(s.ID), s.Name, fmt.Sprint(s.VersionNumber), fmt.Sprint(t.Assessment), a[t.Assessment].Title, fmt.Sprint(t.Number), t.Status, fmt.Sprint(t.Score), fmt.Sprint(t.Percent), fmt.Sprint(t.Passed), t.Finished.In(mustLocation(d.Filters.Timezone)).Format("2006-01-02 15:04:05 MST"), "Автоматически"})
			}
		}
		if kind != "tests" {
			for _, sub := range d.Submissions {
				s := d.ByID[sub.Enrollment]
				if s == nil || !d.Filters.in(sub.Submitted) {
					continue
				}
				rows = append(rows, []string{"Задание", fmt.Sprint(s.ID), s.Name, fmt.Sprint(s.VersionNumber), fmt.Sprint(sub.Assessment), a[sub.Assessment].Title, fmt.Sprint(sub.Number), sub.Status, func() string {
					if sub.Score == nil {
						return "Не оценено"
					}
					return fmt.Sprint(*sub.Score)
				}(), "", "", sub.Submitted.In(mustLocation(d.Filters.Timezone)).Format("2006-01-02 15:04:05 MST"), fmt.Sprint(sub.Reviewer)})
			}
		}
	case "cohorts":
		appendMap(d.cohortRows(), []string{"title", "version_number", "enrollments", "active", "average_progress", "completed", "completion_rate", "test_average", "assignment_acceptance", "pending", "overdue"}, []string{"Поток", "Версия", "Зачисления", "Активные ученики", "Средний прогресс", "Завершили", "Доля завершивших", "Средний тест", "Принято работ, %", "На проверке сейчас", "Просрочено сейчас"})
	case "staff":
		appendMap(d.staffRows(), []string{"name", "role", "assigned", "reviewed", "review_hours", "pending", "revised", "attention"}, []string{"Сотрудник", "Роль", "Ученики", "Проверено", "Среднее ожидание, ч", "На проверке", "Доработка", "Нужна помощь"})
	case "communications":
		rows = append(rows, []string{"Сообщение", "Зачисление", "Получатель ID", "Рассылка/сценарий", "Канал", "Статус", "В очереди с", "Передано провайдеру", "Ошибка"})
		for _, m := range d.Deliveries {
			if d.ByID[m.Enrollment] == nil || !d.Filters.in(m.Queued) {
				continue
			}
			sent := ""
			if m.Sent != nil {
				sent = m.Sent.In(mustLocation(d.Filters.Timezone)).Format("2006-01-02 15:04:05 MST")
			}
			rows = append(rows, []string{fmt.Sprint(m.ID), fmt.Sprint(m.Enrollment), fmt.Sprint(m.User), m.Name, m.Channel, m.Status, m.Queued.In(mustLocation(d.Filters.Timezone)).Format("2006-01-02 15:04:05 MST"), sent, m.Error})
		}
	case "summary":
		o := d.overview()
		rows = append(rows, []string{"Показатель", "Значение"})
		for _, key := range []string{"enrollments", "unique_students", "active_students", "started", "completed", "average_progress", "completion_rate", "pending"} {
			rows = append(rows, []string{map[string]string{"enrollments": "Зачисления", "unique_students": "Уникальные ученики", "active_students": "Активные за период", "started": "Начали", "completed": "Завершили", "average_progress": "Средний прогресс, %", "completion_rate": "Доля завершивших, %", "pending": "Работы на проверке"}[key], exportValue(o[key])})
		}
		rows = append(rows, []string{"Период", "Дата зачисления аудитории; события за период, прогресс текущий"}, []string{"Временная зона", d.Filters.Timezone}, []string{"Сформировано", d.Now.Format("2006-01-02T15:04:05Z07:00")})
	default:
		return nil, ErrInvalid
	}
	if len(rows) > 100001 {
		return nil, inputError{"Отчёт превышает 100 000 строк: сузьте фильтры"}
	}
	return rows, nil
}
func writeReportFile(root, key, format string, rows [][]string) (int64, error) {
	p, e := reportPath(root, key)
	if e != nil {
		return 0, e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return 0, e
	}
	tmp, e := os.CreateTemp(filepath.Dir(p), ".academy-report-*")
	if e != nil {
		return 0, e
	}
	name := tmp.Name()
	defer os.Remove(name)
	_ = tmp.Chmod(0600)
	if format == "xlsx" {
		e = writeAnalyticsXLSX(tmp, rows)
	} else {
		e = writeAnalyticsCSV(tmp, rows)
	}
	if e == nil {
		e = tmp.Sync()
	}
	if closeErr := tmp.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		return 0, e
	}
	if e = os.Rename(name, p); e != nil {
		return 0, e
	}
	stat, e := os.Stat(p)
	if e != nil {
		return 0, e
	}
	return stat.Size(), nil
}

func analyticsNumericColumn(header string) bool {
	switch header {
	case "Зачисление", "ID пользователя", "Версия", "Прогресс, %", "Завершено уроков", "Обязательных уроков", "Ожидается по расписанию", "Средний результат тестов, %", "На проверке", "На доработке", "Проверка", "Попытка", "Баллы", "Процент", "Зачисления", "Активные ученики", "Средний прогресс", "Завершили", "Доля завершивших", "Средний тест", "Принято работ, %", "На проверке сейчас", "Просрочено сейчас", "Ученики", "Проверено", "Среднее ожидание, ч", "Доработка", "Нужна помощь", "Сообщение", "Получатель ID":
		return true
	}
	return false
}
