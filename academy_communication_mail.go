package main

import (
	"FinTalent/internal/academy"
	"context"
	"errors"
	"log"
	"net/mail"
	"net/textproto"
	"strings"
	"sync"
	"time"
)

func sendAcademyCommunication(ctx context.Context, recipient, subject, body, messageID, unsubscribe string) academy.CommunicationResult {
	if ctx.Err() != nil {
		return academy.CommunicationResult{ErrorCode: "worker_stopped", Retryable: true}
	}
	config, err := loadSMTPConfig()
	if err != nil {
		return academy.CommunicationResult{ErrorCode: "smtp_not_configured"}
	}
	address, err := mail.ParseAddress(recipient)
	if err != nil {
		return academy.CommunicationResult{ErrorCode: "invalid_email"}
	}
	message, err := buildHTMLMessage(config.From, *address, subject, []byte(body))
	if err != nil {
		return academy.CommunicationResult{ErrorCode: "message_encoding"}
	}
	headers := "Message-ID: <" + messageID + ">\r\n"
	if unsubscribe != "" {
		headers += "List-Unsubscribe: <" + strings.Replace(unsubscribe, "/academy/unsubscribe/", "/api/academy/unsubscribe/", 1) + ">\r\nList-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n"
	}
	message = append([]byte(headers), message...)
	err = sendSMTPMessage(config, address.Address, message)
	return classifyAcademySMTPResult(err)
}
func classifyAcademySMTPResult(err error) academy.CommunicationResult {
	if err == nil || strings.HasPrefix(err.Error(), "завершение SMTP-сессии:") {
		return academy.CommunicationResult{}
	}
	var smtpErr *textproto.Error
	if errors.As(err, &smtpErr) {
		return academy.CommunicationResult{ErrorCode: "smtp_rejected", Retryable: smtpErr.Code >= 400 && smtpErr.Code < 500}
	}
	uncertain := strings.HasPrefix(err.Error(), "отправка письма:") || strings.HasPrefix(err.Error(), "завершение письма:")
	return academy.CommunicationResult{ErrorCode: map[bool]string{true: "smtp_result_unknown", false: "smtp_connection_failed"}[uncertain], Retryable: !uncertain, Uncertain: uncertain}
}
func startAcademyCommunicationWorker(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		lastScan := time.Time{}
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				job, cancel := context.WithTimeout(ctx, 90*time.Second)
				if time.Since(lastScan) > time.Minute {
					if err := academy.ObserveAutomations(job, db, applicationBaseURL()); err != nil && ctx.Err() == nil {
						log.Print("academy automation processing failed")
					}
					lastScan = time.Now()
				}
				if err := academy.ProcessCommunications(job, db, applicationBaseURL(), sendAcademyCommunication); err != nil && ctx.Err() == nil {
					log.Print("academy communication delivery failed")
				}
				cancel()
			}
		}
	}()
}
