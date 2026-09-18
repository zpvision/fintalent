package main

import (
	"bytes"
	"crypto/tls"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"
)

//go:embed mail/templates/welcome.html
var welcomeEmailTemplate string

//go:embed mail/templates/password_reset.html
var passwordResetEmailTemplate string

//go:embed mail/templates/profimarket_order.html
var profiMarketOrderEmailTemplate string

//go:embed mail/logo.png
var emailLogo []byte

type smtpConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     mail.Address
}

type welcomeEmailData struct {
	UserName   string
	ProfileURL string
}

type passwordResetEmailData struct {
	ResetCode         string
	ExpirationMinutes int
}

type profiMarketOrderEmailData struct {
	SellerName   string
	BuyerName    string
	BuyerEmail   string
	ProductTitle string
	ActionTitle  string
	PriceText    string
	PurchaseID   int64
	OrdersURL    string
}

func loadSMTPConfig() (smtpConfig, error) {
	port, err := strconv.Atoi(strings.TrimSpace(os.Getenv("SMTP_PORT")))
	if err != nil || port < 1 || port > 65535 {
		return smtpConfig{}, errors.New("SMTP_PORT не настроен")
	}
	config := smtpConfig{
		Host:     strings.TrimSpace(os.Getenv("SMTP_HOST")),
		Port:     port,
		Username: strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		Password: os.Getenv("SMTP_PASSWORD"),
	}
	if config.Host == "" || config.Username == "" || config.Password == "" {
		return smtpConfig{}, errors.New("SMTP не настроен")
	}
	config.From = mail.Address{Name: strings.TrimSpace(os.Getenv("SMTP_FROM_NAME")), Address: strings.TrimSpace(os.Getenv("SMTP_FROM"))}
	if config.From.Name == "" {
		config.From.Name = "FinTalent"
	}
	if config.From.Address == "" {
		config.From.Address = config.Username
	}
	if _, err := mail.ParseAddress(config.From.Address); err != nil {
		return smtpConfig{}, errors.New("SMTP_FROM содержит некорректный адрес")
	}
	return config, nil
}

func sendWelcomeEmail(recipientName, recipientEmail string) error {
	config, err := loadSMTPConfig()
	if err != nil {
		return err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("APP_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = "https://fintalent.ru"
	}
	tmpl, err := template.New("welcome").Parse(welcomeEmailTemplate)
	if err != nil {
		return fmt.Errorf("шаблон приветственного письма: %w", err)
	}
	var htmlBody bytes.Buffer
	if err := tmpl.Execute(&htmlBody, welcomeEmailData{UserName: recipientName, ProfileURL: baseURL + "/profile"}); err != nil {
		return fmt.Errorf("формирование приветственного письма: %w", err)
	}
	message, err := buildHTMLMessage(config.From, mail.Address{Name: recipientName, Address: recipientEmail}, "Добро пожаловать в FinTalent", htmlBody.Bytes())
	if err != nil {
		return err
	}
	return sendSMTPMessage(config, recipientEmail, message)
}

func sendPasswordResetEmail(recipientName, recipientEmail, code string, expirationMinutes int) error {
	config, err := loadSMTPConfig()
	if err != nil {
		return err
	}
	tmpl, err := template.New("password-reset").Parse(passwordResetEmailTemplate)
	if err != nil {
		return fmt.Errorf("шаблон письма восстановления: %w", err)
	}
	var htmlBody bytes.Buffer
	if err := tmpl.Execute(&htmlBody, passwordResetEmailData{ResetCode: code, ExpirationMinutes: expirationMinutes}); err != nil {
		return fmt.Errorf("формирование письма восстановления: %w", err)
	}
	message, err := buildHTMLMessage(config.From, mail.Address{Name: recipientName, Address: recipientEmail}, "Код восстановления пароля FinTalent", htmlBody.Bytes())
	if err != nil {
		return err
	}
	return sendSMTPMessage(config, recipientEmail, message)
}

func sendProfiMarketOrderEmail(recipientName, recipientEmail string, data profiMarketOrderEmailData) error {
	config, err := loadSMTPConfig()
	if err != nil {
		return err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("APP_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = "https://fintalent.ru"
	}
	data.SellerName = recipientName
	data.OrdersURL = baseURL + "/profile?section=profimarket&tab=orders"
	tmpl, err := template.New("profimarket-order").Parse(profiMarketOrderEmailTemplate)
	if err != nil {
		return fmt.Errorf("шаблон письма о заказе ПрофиМаркета: %w", err)
	}
	var htmlBody bytes.Buffer
	if err = tmpl.Execute(&htmlBody, data); err != nil {
		return fmt.Errorf("формирование письма о заказе ПрофиМаркета: %w", err)
	}
	message, err := buildHTMLMessage(config.From, mail.Address{Name: recipientName, Address: recipientEmail}, data.ActionTitle+" — ПрофиМаркет", htmlBody.Bytes())
	if err != nil {
		return err
	}
	return sendSMTPMessage(config, recipientEmail, message)
}

func buildHTMLMessage(from, to mail.Address, subject string, htmlBody []byte) ([]byte, error) {
	var body bytes.Buffer
	related := multipart.NewWriter(&body)
	htmlHeader := textproto.MIMEHeader{}
	htmlHeader.Set("Content-Type", "text/html; charset=UTF-8")
	htmlHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	htmlPart, err := related.CreatePart(htmlHeader)
	if err != nil {
		return nil, err
	}
	quoted := quotedprintable.NewWriter(htmlPart)
	if _, err := quoted.Write(htmlBody); err != nil {
		return nil, err
	}
	if err := quoted.Close(); err != nil {
		return nil, err
	}
	logoHeader := textproto.MIMEHeader{}
	logoHeader.Set("Content-Type", "image/png; name=logo.png")
	logoHeader.Set("Content-Disposition", "inline; filename=logo.png")
	logoHeader.Set("Content-ID", "<fintalent-logo>")
	logoHeader.Set("Content-Transfer-Encoding", "base64")
	logoPart, err := related.CreatePart(logoHeader)
	if err != nil {
		return nil, err
	}
	encodedLogo := base64.StdEncoding.EncodeToString(emailLogo)
	for len(encodedLogo) > 76 {
		_, _ = fmt.Fprintln(logoPart, encodedLogo[:76])
		encodedLogo = encodedLogo[76:]
	}
	_, _ = fmt.Fprintln(logoPart, encodedLogo)
	if err := related.Close(); err != nil {
		return nil, err
	}

	var message bytes.Buffer
	encodedSubject := mime.BEncoding.Encode("UTF-8", subject)
	fmt.Fprintf(&message, "From: %s\r\n", from.String())
	fmt.Fprintf(&message, "To: %s\r\n", to.String())
	fmt.Fprintf(&message, "Subject: %s\r\n", encodedSubject)
	fmt.Fprintf(&message, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&message, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&message, "Content-Type: multipart/related; boundary=%q\r\n", related.Boundary())
	fmt.Fprintf(&message, "\r\n")
	message.Write(body.Bytes())
	return message.Bytes(), nil
}

func sendSMTPMessage(config smtpConfig, recipient string, message []byte) error {
	address := net.JoinHostPort(config.Host, strconv.Itoa(config.Port))
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	connection, err := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{ServerName: config.Host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return fmt.Errorf("подключение к SMTP: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(30 * time.Second))
	client, err := smtp.NewClient(connection, config.Host)
	if err != nil {
		return fmt.Errorf("SMTP-клиент: %w", err)
	}
	defer client.Close()
	if err := client.Auth(smtp.PlainAuth("", config.Username, config.Password, config.Host)); err != nil {
		return fmt.Errorf("авторизация SMTP: %w", err)
	}
	if err := client.Mail(config.From.Address); err != nil {
		return fmt.Errorf("SMTP MAIL FROM: %w", err)
	}
	if err := client.Rcpt(recipient); err != nil {
		return fmt.Errorf("SMTP RCPT TO: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA: %w", err)
	}
	if _, err := w.Write(message); err != nil {
		return fmt.Errorf("отправка письма: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("завершение письма: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("завершение SMTP-сессии: %w", err)
	}
	return nil
}
