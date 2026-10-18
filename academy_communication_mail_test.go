package main

import (
	"errors"
	"fmt"
	"net/textproto"
	"testing"
)

func TestAcademySMTPResultClassification(t *testing.T) {
	for _, c := range []struct {
		err              error
		code             string
		retry, uncertain bool
	}{
		{nil, "", false, false},
		{errors.New("завершение SMTP-сессии: connection lost"), "", false, false},
		{fmt.Errorf("SMTP RCPT TO: %w", &textproto.Error{Code: 450, Msg: "temporary"}), "smtp_rejected", true, false},
		{fmt.Errorf("завершение письма: %w", &textproto.Error{Code: 550, Msg: "rejected"}), "smtp_rejected", false, false},
		{errors.New("подключение к SMTP: timeout"), "smtp_connection_failed", true, false},
		{errors.New("отправка письма: connection reset"), "smtp_result_unknown", false, true},
		{errors.New("завершение письма: EOF"), "smtp_result_unknown", false, true},
	} {
		got := classifyAcademySMTPResult(c.err)
		if got.ErrorCode != c.code || got.Retryable != c.retry || got.Uncertain != c.uncertain {
			t.Fatalf("%v: %+v", c.err, got)
		}
	}
}
