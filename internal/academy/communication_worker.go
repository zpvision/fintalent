package academy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type CommunicationResult struct {
	ErrorCode  string
	Retryable  bool
	Uncertain  bool
	ProviderID string
}
type CommunicationTransport func(context.Context, string, string, string, string, string) CommunicationResult
type queuedCommunication struct {
	ID, Course, Actor, Recipient      int64
	Campaign, Run, Enrollment         *int64
	Channel, Category, Target, Status string
	Message                           Message
	Attempt                           int
	Test                              bool
	Lease, MessageID                  string
}

// The transport is the application's SMTP adapter in production and an explicit mock in tests.
func ProcessCommunications(ctx context.Context, db *sql.DB, base string, send CommunicationTransport) error {
	if _, err := db.ExecContext(ctx, `WITH stale AS(UPDATE academy_message_deliveries SET status='failed',uncertain=true,error_code='smtp_result_unknown',lease_key=NULL WHERE status='sending' AND claimed_at<now()-INTERVAL '5 minutes' RETURNING id,attempts) UPDATE academy_delivery_attempts a SET outcome='uncertain',error_code='smtp_result_unknown',finished_at=now() FROM stale s WHERE a.delivery_id=s.id AND a.number=s.attempts AND a.finished_at IS NULL`); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var locked bool
	if err = tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(hashtext('academy:communication-claim'))`).Scan(&locked); err != nil || !locked {
		return err
	}
	var id int64
	var campaign, run *int64
	err = tx.QueryRowContext(ctx, `SELECT id,campaign_id,run_id FROM academy_message_deliveries WHERE status='queued' AND next_attempt_at<=now() ORDER BY next_attempt_at,id LIMIT 1`).Scan(&id, &campaign, &run)
	if err == sql.ErrNoRows {
		if er := refreshCampaigns(ctx, tx); er != nil {
			return er
		}
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	var c *Campaign
	var a *Automation
	var generation int64
	var source, eventType string
	var sourceData []byte
	var eventAt time.Time
	if campaign != nil {
		var raw []byte
		err = tx.QueryRowContext(ctx, `SELECT to_jsonb(c) FROM academy_campaigns c WHERE id=$1 FOR SHARE`, *campaign).Scan(&raw)
		if err != nil {
			return err
		}
		c = &Campaign{}
		if err = json.Unmarshal(raw, c); err != nil {
			return err
		}
	}
	if run != nil {
		var raw []byte
		err = tx.QueryRowContext(ctx, `SELECT to_jsonb(a),r.generation,r.source_key,r.event_type,r.source_data,r.event_at FROM academy_automations a JOIN academy_automation_runs r ON r.automation_id=a.id WHERE r.id=$1 FOR SHARE OF a`, *run).Scan(&raw, &generation, &source, &eventType, &sourceData, &eventAt)
		if err != nil {
			return err
		}
		a = &Automation{}
		if err = json.Unmarshal(raw, a); err != nil {
			return err
		}
	}
	var d queuedCommunication
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT id,course_id,actor_id,recipient_id,campaign_id,run_id,enrollment_id,channel,category,target,message,attempts,test,status FROM academy_message_deliveries WHERE id=$1 AND status='queued' AND next_attempt_at<=now() FOR UPDATE`, id).Scan(&d.ID, &d.Course, &d.Actor, &d.Recipient, &d.Campaign, &d.Run, &d.Enrollment, &d.Channel, &d.Category, &d.Target, &raw, &d.Attempt, &d.Test, &d.Status)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if err = json.Unmarshal(raw, &d.Message); err != nil {
		return err
	}
	cancel := func(code string) error {
		if _, er := tx.ExecContext(ctx, `UPDATE academy_message_deliveries SET status='cancelled',error_code=$2 WHERE id=$1`, id, code); er != nil {
			return er
		}
		if er := refreshCampaigns(ctx, tx); er != nil {
			return er
		}
		return tx.Commit()
	}
	postpone := func(seconds int, code string) error {
		_, er := tx.ExecContext(ctx, `UPDATE academy_message_deliveries SET next_attempt_at=now()+$2*INTERVAL '1 second',error_code=$3 WHERE id=$1`, id, seconds, code)
		if er != nil {
			return er
		}
		return tx.Commit()
	}
	if c != nil && c.Status == "cancelled" {
		return cancel("campaign_cancelled")
	}
	if a != nil {
		if a.Generation != generation || a.Status == "archived" {
			return cancel("automation_changed")
		}
		if a.Status != "active" {
			return postpone(60, "automation_paused")
		}
	}
	vars := map[string]string{}
	var e *Enrollment
	if d.Enrollment != nil {
		e, err = loadEnrollment(ctx, tx, *d.Enrollment, d.Actor, true, false)
		if err == sql.ErrNoRows {
			return cancel("permission_revoked")
		}
		if err != nil {
			return err
		}
		if e.EffectiveStatus != "active" {
			return cancel("enrollment_unavailable")
		}
		s := Segment{}
		if c != nil {
			s = c.Audience
		}
		if a != nil {
			s = a.Config.Audience
		}
		ok, er := matchesSegment(ctx, tx, s, d.Course, d.Actor, e.ID)
		if er != nil {
			return er
		}
		if !ok {
			return cancel("conditions_changed")
		}
		if a != nil {
			if eventType == "inactive" {
				anchor := e.EnrolledAt
				if e.LastActivityAt != nil {
					anchor = *e.LastActivityAt
				}
				if anchor.UTC().Format("2006-01-02T15:04:05.000000Z") != strings.TrimPrefix(source, "inactive:") || time.Since(anchor) < time.Duration(a.Config.InactiveDays)*24*time.Hour {
					return cancel("activity_resumed")
				}
			} else {
				var valid bool
				er = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM academy_communication_events WHERE source_key=$1 AND enrollment_id=$2)`, source, e.ID).Scan(&valid)
				if er != nil {
					return er
				}
				if !valid {
					return cancel("source_event_cancelled")
				}
			}
		}
		if d.Target == "curator" {
			if e.CuratorID == 0 {
				return cancel("curator_unassigned")
			}
			d.Recipient = e.CuratorID
			var allowed bool
			if err = tx.QueryRowContext(ctx, `SELECT academy_can_manage_enrollment($1,$2,false)`, e.ID, d.Recipient).Scan(&allowed); err != nil {
				return err
			}
			if !allowed {
				return cancel("curator_unavailable")
			}
		}
		if d.Target == "owner" {
			if err = tx.QueryRowContext(ctx, `SELECT owner_id FROM courses WHERE id=$1`, d.Course).Scan(&d.Recipient); err != nil {
				return err
			}
		}
		if err = readProgress(ctx, tx, e); err != nil {
			return err
		}
		data := map[string]any{}
		_ = json.Unmarshal(sourceData, &data)
		if a != nil && oneOf(a.Trigger, "deadline_approaching", "deadline_missed") {
			if key, ok := data["block_key"].(string); ok {
				var submitted bool
				if er := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM academy_assignment_submissions s JOIN academy_assessment_versions av ON av.id=s.assessment_id WHERE s.enrollment_id=$1 AND av.block_key=$2 AND s.status IN('submitted','in_review','accepted'))", e.ID, key).Scan(&submitted); er != nil {
					return er
				}
				if submitted {
					return cancel("assignment_already_submitted")
				}
			}
		}
		vars, err = communicationVariablesFor(ctx, tx, e, base, data)
		if err != nil {
			return err
		}
		if d.Target != "student" {
			vars["course.url"] = fmt.Sprintf("%s/profile/courses/%d/academy/crm", base, e.CourseID)
			vars["lesson.url"] = vars["course.url"]
		}
	}
	var email, name string
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT email,full_name,NOT is_blocked AND NOT is_system FROM users WHERE id=$1`, d.Recipient).Scan(&email, &name, &active)
	if err != nil {
		return err
	}
	if !active {
		return cancel("recipient_unavailable")
	}
	if d.Test {
		var authorized bool
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM courses c JOIN users u ON u.id=$2 JOIN users owner ON owner.id=c.owner_id WHERE c.id=$1 AND NOT u.is_blocked AND NOT owner.is_blocked AND(c.owner_id=$2 OR EXISTS(SELECT 1 FROM academy_cohort_staff s JOIN academy_cohorts ch ON ch.id=s.cohort_id WHERE ch.course_id=c.id AND s.user_id=$2 AND s.can_communicate)))`, d.Course, d.Actor).Scan(&authorized)
		if err != nil {
			return err
		}
		if !authorized {
			return cancel("permission_revoked")
		}
		vars = sampleVariables(base, name)
	} else {
		ok, er := preferenceAllowed(ctx, tx, d.Recipient, d.Course, d.Channel, d.Category)
		if er != nil {
			return er
		}
		if !ok {
			return cancel("unsubscribed")
		}
	}
	var global, userHour, userDay, marketingDay int
	err = tx.QueryRowContext(ctx, `SELECT count(*) FILTER(WHERE d.channel='email' AND a.started_at>now()-INTERVAL '1 minute'),count(*) FILTER(WHERE d.recipient_id=$1 AND d.channel=$2 AND a.started_at>now()-INTERVAL '1 hour'),count(*) FILTER(WHERE d.recipient_id=$1 AND d.channel=$2),count(*) FILTER(WHERE d.recipient_id=$1 AND d.channel=$2 AND d.category='marketing') FROM academy_delivery_attempts a JOIN academy_message_deliveries d ON d.id=a.delivery_id WHERE a.started_at>now()-INTERVAL '1 day'`, d.Recipient, d.Channel).Scan(&global, &userHour, &userDay, &marketingDay)
	if err != nil {
		return err
	}
	if d.Channel == "email" && global >= 30 {
		return postpone(60, "rate_limited")
	}
	hourLimit, dayLimit := 3, 10
	if d.Channel == "in_app" {
		hourLimit = 10
		dayLimit = 30
	}
	if userHour >= hourLimit || userDay >= dayLimit || d.Category == "marketing" && marketingDay >= 1 {
		return postpone(3600, "rate_limited")
	}
	token := ""
	if !d.Test {
		_, err = tx.ExecContext(ctx, `INSERT INTO academy_communication_preferences(user_id,course_id,unsubscribe_token) VALUES($1,$2,$3) ON CONFLICT(user_id,course_id) DO NOTHING`, d.Recipient, d.Course, Key())
		if err == nil {
			err = tx.QueryRowContext(ctx, `SELECT unsubscribe_token FROM academy_communication_preferences WHERE user_id=$1 AND course_id=$2`, d.Recipient, d.Course).Scan(&token)
		}
		if err != nil {
			return err
		}
	}
	unsubscribe := ""
	if token != "" {
		unsubscribe = base + "/academy/unsubscribe/" + token
	}
	subject, body, err := RenderMessage(d.Message, vars, base, unsubscribe)
	if err != nil {
		return cancel("invalid_message")
	}
	d.Attempt++
	d.Lease = Key()
	d.MessageID = fmt.Sprintf("academy-%d@fintalent.ru", id)
	_, err = tx.ExecContext(ctx, `UPDATE academy_message_deliveries SET status='sending',attempts=$2,claimed_at=now(),lease_key=$3,error_code='',recipient_id=$4,message_id=$5 WHERE id=$1`, id, d.Attempt, d.Lease, d.Recipient, d.MessageID)
	if err == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO academy_delivery_attempts(delivery_id,number) VALUES($1,$2)`, id, d.Attempt)
	}
	if err != nil {
		return err
	}
	if d.Channel == "in_app" {
		eid := d.Course
		entity := "academy_course"
		if e != nil && d.Recipient == e.UserID {
			eid = e.ID
			entity = "academy_enrollment"
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body,entity_type,entity_id) VALUES($1,'academy_communication',$2,$3,$4,$5)`, d.Recipient, subject, communicationPlainText(d.Message, vars), entity, eid)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE academy_message_deliveries SET status='sent',sent_at=now(),lease_key=NULL WHERE id=$1`, id)
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE academy_delivery_attempts SET outcome='sent',finished_at=now() WHERE delivery_id=$1 AND number=$2`, id, d.Attempt)
		}
		if err == nil {
			err = refreshCampaigns(ctx, tx)
		}
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	result := CommunicationResult{ErrorCode: "transport_unavailable"}
	if send != nil {
		result = send(ctx, email, subject, body, d.MessageID, unsubscribe)
	}
	// Never retry an expired sending lease automatically: SMTP may already have accepted it.
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer finishCancel()
	finish, err := db.BeginTx(finishCtx, nil)
	if err != nil {
		return err
	}
	defer finish.Rollback()
	status := "sent"
	delay := min(3600, 30*(1<<min(d.Attempt, 7)))
	if result.ErrorCode != "" {
		status = "failed"
		if result.Retryable && !result.Uncertain && d.Attempt < 5 {
			status = "queued"
		}
	}
	res, err := finish.ExecContext(finishCtx, `UPDATE academy_message_deliveries SET status=$3,error_code=$4,uncertain=$5,provider_id=$6,lease_key=NULL,sent_at=CASE WHEN $3='sent' THEN now() END,next_attempt_at=now()+$7*INTERVAL '1 second' WHERE id=$1 AND lease_key=$2 AND status='sending'`, id, d.Lease, status, result.ErrorCode, result.Uncertain, result.ProviderID, delay)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return finish.Commit()
	}
	_, err = finish.ExecContext(finishCtx, `UPDATE academy_delivery_attempts SET outcome=$3,error_code=$4,finished_at=now() WHERE delivery_id=$1 AND number=$2`, id, d.Attempt, status, result.ErrorCode)
	if err == nil {
		err = refreshCampaigns(finishCtx, finish)
	}
	if err != nil {
		return err
	}
	return finish.Commit()
}
func refreshCampaigns(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `UPDATE academy_campaigns c SET status=CASE WHEN d.sending>0 THEN 'sending' WHEN d.queued>0 THEN CASE WHEN c.send_at>now() THEN 'scheduled' ELSE 'sending' END WHEN d.failed>0 AND d.sent>0 THEN 'partial' WHEN d.failed>0 THEN 'failed' WHEN d.sent>0 THEN 'completed' ELSE 'cancelled' END FROM(SELECT campaign_id,count(*) FILTER(WHERE status='queued') queued,count(*) FILTER(WHERE status='sending') sending,count(*) FILTER(WHERE status='failed') failed,count(*) FILTER(WHERE status IN('sent','delivered')) sent FROM academy_message_deliveries WHERE campaign_id IS NOT NULL GROUP BY campaign_id)d WHERE c.id=d.campaign_id AND c.status NOT IN('draft','cancelled')`)
	return err
}

func ObserveAutomations(ctx context.Context, db *sql.DB, base string) error {
	rows, err := db.QueryContext(ctx, `SELECT id FROM academy_automations WHERE status='active' ORDER BY id`)
	if err != nil {
		return err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = observeAutomation(ctx, db, id, base); err != nil {
			return err
		}
	}
	return nil
}
func observeAutomation(ctx context.Context, db *sql.DB, id int64, base string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT to_jsonb(a) FROM academy_automations a WHERE id=$1 AND status='active' FOR UPDATE SKIP LOCKED`, id).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var a Automation
	if err = json.Unmarshal(raw, &a); err != nil {
		return err
	}
	var events *sql.Rows
	if a.Trigger == "inactive" {
		events, err = tx.QueryContext(ctx, `SELECT 'inactive:'||to_char(COALESCE(e.last_activity_at,e.enrolled_at) AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),e.id,COALESCE(e.last_activity_at,e.enrolled_at)+$3*INTERVAL '1 day','{}'::jsonb,'' FROM academy_enrollments e WHERE e.course_id=$1 AND COALESCE(e.last_activity_at,e.enrolled_at)<now()-$3*INTERVAL '1 day' AND NOT EXISTS(SELECT 1 FROM academy_automation_runs r WHERE r.automation_id=$2 AND r.generation=$4 AND r.enrollment_id=e.id AND r.source_key='inactive:'||to_char(COALESCE(e.last_activity_at,e.enrolled_at) AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')) ORDER BY e.id LIMIT 100`, a.CourseID, a.ID, a.Config.InactiveDays, a.Generation)
	} else {
		trigger := a.Trigger
		types := []string{trigger}
		if trigger == "enrolled" {
			types = []string{"enrolled", "student_enrolled"}
		}
		if oneOf(trigger, "test_passed", "test_failed") {
			types = []string{"test_finished"}
		}
		if trigger == "assignment_reviewed" {
			types = []string{"assignment_accepted", "assignment_rejected", "assignment_revision_requested"}
		}
		if oneOf(trigger, "module_unlocked", "lesson_unlocked") {
			types = []string{"lesson_unlocked"}
		}
		if trigger == "deadline_approaching" {
			types = []string{"deadline_missed"}
		}
		events, err = tx.QueryContext(ctx, `SELECT x.source_key,x.enrollment_id,x.event_at,x.data,x.lesson_key FROM academy_communication_events x WHERE x.course_id=$1 AND x.event_type IN(SELECT value FROM jsonb_array_elements_text($2::jsonb)) AND ($6<>'enrolled' OR x.source_key LIKE 'learning:%') AND x.event_at>= $3 AND NOT EXISTS(SELECT 1 FROM academy_automation_runs r WHERE r.automation_id=$4 AND r.generation=$5 AND r.enrollment_id=x.enrollment_id AND r.source_key=x.source_key) AND($6 NOT IN('test_passed','test_failed') OR COALESCE((x.data->'details'->>'passed')::boolean,false)=($6='test_passed')) ORDER BY x.event_at,x.source_key LIMIT 100`, a.CourseID, jsonValue(types), a.ActiveSince, a.ID, a.Generation, a.Trigger)
	}
	if err != nil {
		return err
	}
	type sourceEvent struct {
		Key        string
		Enrollment int64
		At         time.Time
		Data       []byte
		Lesson     string
	}
	items := []sourceEvent{}
	for events.Next() {
		var v sourceEvent
		if err = events.Scan(&v.Key, &v.Enrollment, &v.At, &v.Data, &v.Lesson); err != nil {
			events.Close()
			return err
		}
		items = append(items, v)
	}
	err = events.Err()
	events.Close()
	if err != nil {
		return err
	}
	for _, v := range items {
		var run int64
		er := tx.QueryRowContext(ctx, `INSERT INTO academy_automation_runs(automation_id,generation,enrollment_id,source_key,event_type,event_at,source_data,conditions) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING RETURNING id`, a.ID, a.Generation, v.Enrollment, v.Key, a.Trigger, v.At, string(v.Data), jsonValue(a.Config.Audience)).Scan(&run)
		if er == sql.ErrNoRows {
			continue
		}
		if er != nil {
			return er
		}
		skip := func(reason string) error {
			_, err := tx.ExecContext(ctx, `UPDATE academy_automation_runs SET skipped_reason=$2 WHERE id=$1`, run, reason)
			return err
		}
		ok, er := matchesSegment(ctx, tx, a.Config.Audience, a.CourseID, a.Creator, v.Enrollment)
		if er != nil {
			return er
		}
		if !ok {
			if er = skip("conditions_not_matched"); er != nil {
				return er
			}
			continue
		}
		e, er := loadEnrollment(ctx, tx, v.Enrollment, a.Creator, true, false)
		if er == sql.ErrNoRows {
			if er = skip("permission_revoked"); er != nil {
				return er
			}
			continue
		}
		if er != nil {
			return er
		}
		if e.EffectiveStatus != "active" {
			if er = skip("enrollment_unavailable"); er != nil {
				return er
			}
			continue
		}
		if a.Trigger == "module_unlocked" {
			first := false
			for _, m := range e.Program.Modules {
				for _, l := range m.Lessons {
					if l.Status == "published" {
						first = l.Key == v.Lesson
						break
					}
				}
				if first {
					break
				}
			}
			if !first {
				if er = skip("not_module_opening"); er != nil {
					return er
				}
				continue
			}
		}
		for _, step := range a.Config.Steps {
			due := v.At
			if a.Trigger == "deadline_approaching" {
				due = due.Add(-time.Duration(a.Config.LeadDays) * 24 * time.Hour)
			}
			triggerAt := due
			switch step.Timing {
			case "delay":
				due = due.Add(time.Duration(step.DelayMinutes) * time.Minute)
			case "before_deadline":
				due = v.At.Add(-time.Duration(step.DelayMinutes) * time.Minute)
			case "at":
				due, _ = time.Parse(time.RFC3339, step.At)
				if due.Before(triggerAt) {
					due = triggerAt
				}
			}
			recipient := e.UserID
			if step.Target == "curator" {
				recipient = e.CuratorID
			}
			if step.Target == "owner" {
				if er = tx.QueryRowContext(ctx, `SELECT owner_id FROM courses WHERE id=$1`, e.CourseID).Scan(&recipient); er != nil {
					return er
				}
			}
			if recipient == 0 {
				recipient = e.UserID
			} // Retargeting is checked at execution; missing curator becomes a recorded cancellation.
			_, er = tx.ExecContext(ctx, `INSERT INTO academy_message_deliveries(course_id,actor_id,run_id,enrollment_id,recipient_id,target,channel,category,message,dedup_key,next_attempt_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(dedup_key) DO NOTHING`, a.CourseID, a.Creator, run, e.ID, recipient, step.Target, step.Channel, step.Category, jsonValue(step.Message), "run:"+strconv.FormatInt(run, 10)+":"+step.Key, due)
			if er != nil {
				return er
			}
		}
	}
	return tx.Commit()
}
