package main

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"
)

//go:embed migrations/068_notification_outbox.sql
var notificationOutboxSchema string

func prepareNotificationOutboxDatabase(ctx context.Context) error {
	_, err := db.ExecContext(ctx, notificationOutboxSchema)
	return err
}

type outboxWriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func enqueueNotification(ctx context.Context, writer outboxWriter, kind, name, email, subject string, payload any, key string) error {
	// Callers without a business transaction still need an atomic admission check.
	if database, ok := writer.(*sql.DB); ok {
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if err = enqueueNotification(ctx, tx, kind, name, email, subject, payload, key); err != nil {
			return err
		}
		return tx.Commit()
	}
	if kind == "reset" {
		code, ok := payload.(string)
		if !ok {
			return errors.New("invalid reset notification")
		}
		sealed, err := encryptResetCode(code)
		if err != nil {
			return err
		}
		payload = sealed
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err = writer.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('fintalent:notification-outbox-admission'))`); err != nil {
		return err
	}
	if key != "" {
		var exists bool
		if err = writer.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM notification_outbox WHERE dedup_key=$1)`, key).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return nil
		}
	}
	var pending int
	if err = writer.QueryRowContext(ctx, `SELECT count(*) FROM notification_outbox WHERE status IN ('pending','sending')`).Scan(&pending); err != nil {
		return err
	}
	if pending >= 10000 {
		return errors.New("notification queue capacity exceeded")
	}
	_, err = writer.ExecContext(ctx, `INSERT INTO notification_outbox(kind,recipient_name,recipient_email,subject,payload,dedup_key) VALUES($1,$2,$3,$4,$5,NULLIF($6,'')) ON CONFLICT(dedup_key) DO NOTHING`, kind, name, email, subject, string(data), key)
	return err
}

type outboxMessage struct {
	id                         int64
	kind, name, email, subject string
	payload                    []byte
	attempts                   int
	created                    time.Time
}

func deliverOutbox(m outboxMessage) error {
	switch m.kind {
	case "welcome":
		return sendWelcomeEmail(m.name, m.email)
	case "event":
		var data eventNotificationEmailData
		if err := json.Unmarshal(m.payload, &data); err != nil {
			return err
		}
		return sendEventNotificationEmail(m.name, m.email, m.subject, data)
	case "order":
		var data profiMarketOrderEmailData
		if err := json.Unmarshal(m.payload, &data); err != nil {
			return err
		}
		return sendProfiMarketOrderEmail(m.name, m.email, data)
	case "employee":
		var data employeeTestInvitationEmailData
		if err := json.Unmarshal(m.payload, &data); err != nil {
			return err
		}
		return sendEmployeeTestInvitationEmail(m.email, data)
	case "reset":
		// Never send a code after its ten-minute lifetime.
		if time.Since(m.created) > 9*time.Minute {
			return errors.New("reset message expired")
		}
		var code string
		if err := json.Unmarshal(m.payload, &code); err != nil {
			return err
		}
		plain, err := decryptResetCode(code)
		if err != nil {
			return err
		}
		return sendPasswordResetEmail(m.name, m.email, plain, 10)
	default:
		return errors.New("unknown notification kind")
	}
}

// Two workers, short DB leases and bounded SMTP timeouts. Delivery is at least
// once; an SMTP success followed by a process failure can still repeat a mail.
func startNotificationWorkers(ctx context.Context) *sync.WaitGroup {
	wg := &sync.WaitGroup{}
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					processOutbox(ctx)
				}
			}
		}()
	}
	return wg
}

func processOutbox(ctx context.Context) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	// A crash during the final delivery must not leave an expired lease forever.
	if _, err := db.ExecContext(queryCtx, `UPDATE notification_outbox SET status='failed',payload=CASE WHEN kind='reset' THEN '{}'::jsonb ELSE payload END WHERE status='sending' AND attempts>=8 AND next_attempt_at<=NOW()`); err != nil {
		cancel()
		log.Print("notification outbox: cannot recover expired final leases")
		return
	}
	var m outboxMessage
	err := db.QueryRowContext(queryCtx, `UPDATE notification_outbox SET status='sending',attempts=attempts+1,next_attempt_at=NOW()+INTERVAL '2 minutes' WHERE id=(SELECT id FROM notification_outbox WHERE status IN('pending','sending') AND next_attempt_at<=NOW() AND attempts<8 ORDER BY next_attempt_at,id LIMIT 1 FOR UPDATE SKIP LOCKED) RETURNING id,kind,recipient_name,recipient_email,subject,payload,attempts,created_at`).Scan(&m.id, &m.kind, &m.name, &m.email, &m.subject, &m.payload, &m.attempts, &m.created)
	cancel()
	if err == sql.ErrNoRows {
		return
	}
	if err != nil {
		log.Print("notification outbox: cannot claim message")
		return
	}
	err = deliverOutbox(m)
	// Complete the lease even during graceful shutdown, without logging recipients,
	// reset codes, invitation tokens or SMTP credentials.
	completeCtx, completeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer completeCancel()
	if err == nil {
		_, err = db.ExecContext(completeCtx, `UPDATE notification_outbox SET status='sent',sent_at=NOW(),payload='{}',recipient_email='',recipient_name='' WHERE id=$1`, m.id)
	} else {
		status := "pending"
		if m.attempts >= 8 || (m.kind == "reset" && time.Since(m.created) > 9*time.Minute) {
			status = "failed"
		}
		_, err = db.ExecContext(completeCtx, `UPDATE notification_outbox SET status=$2,next_attempt_at=NOW()+($3*INTERVAL '1 second'),payload=CASE WHEN $2='failed' AND kind='reset' THEN '{}'::jsonb ELSE payload END WHERE id=$1`, m.id, status, min(3600, 15*(1<<m.attempts)))
		log.Printf("notification outbox: message %d delivery failed (attempt %d)", m.id, m.attempts)
	}
	if err != nil {
		log.Printf("notification outbox: cannot record delivery status for %d", m.id)
	}
}
