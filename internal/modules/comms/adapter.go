package comms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"strings"
	"time"
)

// Adapter is the pluggable interface every channel must implement.
// Adding a new channel (e.g. Telegram, Slack) only requires implementing this interface.
type Adapter interface {
	// Channel returns the channel type this adapter handles.
	Channel() ChannelType
	// Send delivers the message and returns a provider reference ID.
	Send(ctx context.Context, msg Message) (providerRef string, err error)
}

// ─── Email Adapter (SMTP / SendGrid) ────────────────────────────────────────

type EmailAdapter struct {
	// SMTP config
	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	FromAddr string
	// SendGrid config (takes priority if APIKey is set)
	SendGridAPIKey string
}

func NewEmailAdapter() *EmailAdapter {
	return &EmailAdapter{
		SMTPHost:       getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:       getEnv("SMTP_PORT", "587"),
		SMTPUser:       getEnv("SMTP_USER", ""),
		SMTPPass:       getEnv("SMTP_PASS", ""),
		FromAddr:       getEnv("SMTP_FROM", "noreply@printa.co.zm"),
		SendGridAPIKey: getEnv("SENDGRID_API_KEY", ""),
	}
}

func (a *EmailAdapter) Channel() ChannelType { return ChannelEmail }

func (a *EmailAdapter) Send(ctx context.Context, msg Message) (string, error) {
	if a.SendGridAPIKey != "" {
		return a.sendViaSendGrid(ctx, msg)
	}
	return a.sendViaSMTP(msg)
}

func (a *EmailAdapter) sendViaSendGrid(ctx context.Context, msg Message) (string, error) {
	body := map[string]interface{}{
		"personalizations": []map[string]interface{}{
			{"to": []map[string]string{{"email": msg.Recipient}}},
		},
		"from":    map[string]string{"email": a.FromAddr},
		"subject": msg.Subject,
		"content": []map[string]string{
			{"type": "text/plain", "value": msg.Body},
		},
	}
	if msg.HTMLBody != "" {
		body["content"] = append(body["content"].([]map[string]string),
			map[string]string{"type": "text/html", "value": msg.HTMLBody})
	}
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.sendgrid.com/v3/mail/send", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+a.SendGridAPIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sendgrid: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("sendgrid error %d: %s", resp.StatusCode, string(b))
	}
	return resp.Header.Get("X-Message-Id"), nil
}

func (a *EmailAdapter) sendViaSMTP(msg Message) (string, error) {
	if a.SMTPUser == "" {
		return "mock-email-ref", nil // sandbox mode — log only
	}
	auth := smtp.PlainAuth("", a.SMTPUser, a.SMTPPass, a.SMTPHost)
	body := msg.Body
	if msg.HTMLBody != "" {
		body = msg.HTMLBody
	}
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	message := "To: " + msg.Recipient + "\r\n" +
		"From: " + a.FromAddr + "\r\n" +
		"Subject: " + msg.Subject + "\r\n" +
		mime + body
	err := smtp.SendMail(a.SMTPHost+":"+a.SMTPPort, auth, a.FromAddr, []string{msg.Recipient}, []byte(message))
	if err != nil {
		return "", fmt.Errorf("smtp: %w", err)
	}
	return fmt.Sprintf("smtp-%d", time.Now().UnixNano()), nil
}

// ─── SMS Adapter (Africa's Talking / Twilio) ────────────────────────────────

type SMSAdapter struct {
	// Africa's Talking
	ATAPIKey   string
	ATUsername string
	ATSender   string
	// Twilio
	TwilioSID   string
	TwilioToken string
	TwilioFrom  string
}

func NewSMSAdapter() *SMSAdapter {
	return &SMSAdapter{
		ATAPIKey:    getEnv("AT_API_KEY", ""),
		ATUsername:  getEnv("AT_USERNAME", getEnv("AFRICASTALKING_USERNAME", "sandbox")),
		ATSender:    getEnv("AT_SENDER_ID", getEnv("AFRICASTALKING_SENDER_ID", "Printa")),
		TwilioSID:   getEnv("TWILIO_SID", ""),
		TwilioToken: getEnv("TWILIO_TOKEN", ""),
		TwilioFrom:  getEnv("TWILIO_FROM", ""),
	}
}

func (a *SMSAdapter) Channel() ChannelType { return ChannelSMS }

func (a *SMSAdapter) Send(ctx context.Context, msg Message) (string, error) {
	if a.TwilioSID != "" {
		return a.sendViaTwilio(ctx, msg)
	}
	if a.ATAPIKey != "" || getEnv("AFRICASTALKING_API_KEY", "") != "" {
		return a.sendViaAfricasTalking(ctx, msg)
	}
	// Sandbox mode — log only
	fmt.Printf("[SMS SANDBOX] To: %s | Body: %s\n", msg.Recipient, msg.Body)
	return fmt.Sprintf("sms-sandbox-%d", time.Now().UnixNano()), nil
}

func (a *SMSAdapter) sendViaAfricasTalking(ctx context.Context, msg Message) (string, error) {
	apiKey := a.ATAPIKey
	if apiKey == "" {
		apiKey = getEnv("AFRICASTALKING_API_KEY", "")
	}
	payload := url.Values{}
	payload.Set("username", a.ATUsername)
	payload.Set("to", msg.Recipient)
	payload.Set("message", msg.Body)
	if a.ATSender != "" {
		payload.Set("from", a.ATSender)
	}
	req, _ := http.NewRequestWithContext(ctx, "POST",
		"https://api.africastalking.com/version1/messaging",
		strings.NewReader(payload.Encode()))
	req.Header.Set("apiKey", apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("africas talking: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("africas talking error %d: %s", resp.StatusCode, string(b))
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return fmt.Sprintf("at-%d", time.Now().UnixNano()), nil
}

func (a *SMSAdapter) sendViaTwilio(ctx context.Context, msg Message) (string, error) {
	payload := fmt.Sprintf("To=%s&From=%s&Body=%s", msg.Recipient, a.TwilioFrom, msg.Body)
	url := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", a.TwilioSID)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(payload))
	req.SetBasicAuth(a.TwilioSID, a.TwilioToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("twilio: %w", err)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if sid, ok := result["sid"].(string); ok {
		return sid, nil
	}
	return fmt.Sprintf("twilio-%d", time.Now().UnixNano()), nil
}

// ─── Push Notification Adapter (FCM) ────────────────────────────────────────

type PushAdapter struct {
	FCMServerKey string
	FCMProjectID string
}

func NewPushAdapter() *PushAdapter {
	return &PushAdapter{
		FCMServerKey: getEnv("FCM_SERVER_KEY", ""),
		FCMProjectID: getEnv("FCM_PROJECT_ID", ""),
	}
}

func (a *PushAdapter) Channel() ChannelType { return ChannelPush }

func (a *PushAdapter) Send(ctx context.Context, msg Message) (string, error) {
	if a.FCMServerKey == "" {
		fmt.Printf("[PUSH SANDBOX] To: %s | Title: %s | Body: %s\n", msg.Recipient, msg.Subject, msg.Body)
		return fmt.Sprintf("push-sandbox-%d", time.Now().UnixNano()), nil
	}
	payload := map[string]interface{}{
		"to": msg.Recipient,
		"notification": map[string]string{
			"title": msg.Subject,
			"body":  msg.Body,
		},
		"data": msg.Metadata,
	}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://fcm.googleapis.com/fcm/send", bytes.NewReader(b))
	req.Header.Set("Authorization", "key="+a.FCMServerKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fcm: %w", err)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if id, ok := result["message_id"].(string); ok {
		return id, nil
	}
	return fmt.Sprintf("fcm-%d", time.Now().UnixNano()), nil
}

// ─── WhatsApp Adapter (Twilio / Meta Cloud API) ──────────────────────────────

type WhatsAppAdapter struct {
	// Twilio WhatsApp
	TwilioSID   string
	TwilioToken string
	TwilioFrom  string // e.g. whatsapp:+14155238886
	// Meta Cloud API
	MetaToken   string
	MetaPhoneID string
}

func NewWhatsAppAdapter() *WhatsAppAdapter {
	return &WhatsAppAdapter{
		TwilioSID:   getEnv("TWILIO_SID", ""),
		TwilioToken: getEnv("TWILIO_TOKEN", ""),
		TwilioFrom:  getEnv("TWILIO_WHATSAPP_FROM", ""),
		MetaToken:   getEnv("META_WHATSAPP_TOKEN", ""),
		MetaPhoneID: getEnv("META_WHATSAPP_PHONE_ID", ""),
	}
}

func (a *WhatsAppAdapter) Channel() ChannelType { return ChannelWhatsApp }

func (a *WhatsAppAdapter) Send(ctx context.Context, msg Message) (string, error) {
	if a.MetaToken != "" {
		return a.sendViaMeta(ctx, msg)
	}
	if a.TwilioSID != "" {
		return a.sendViaTwilio(ctx, msg)
	}
	fmt.Printf("[WHATSAPP SANDBOX] To: %s | Body: %s\n", msg.Recipient, msg.Body)
	return fmt.Sprintf("wa-sandbox-%d", time.Now().UnixNano()), nil
}

func (a *WhatsAppAdapter) sendViaTwilio(ctx context.Context, msg Message) (string, error) {
	payload := fmt.Sprintf("To=whatsapp:%s&From=%s&Body=%s", msg.Recipient, a.TwilioFrom, msg.Body)
	url := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", a.TwilioSID)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(payload))
	req.SetBasicAuth(a.TwilioSID, a.TwilioToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("twilio whatsapp: %w", err)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if sid, ok := result["sid"].(string); ok {
		return sid, nil
	}
	return fmt.Sprintf("twilio-wa-%d", time.Now().UnixNano()), nil
}

func (a *WhatsAppAdapter) sendViaMeta(ctx context.Context, msg Message) (string, error) {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                msg.Recipient,
		"type":              "text",
		"text":              map[string]string{"body": msg.Body},
	}
	b, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/messages", a.MetaPhoneID)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+a.MetaToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("meta whatsapp: %w", err)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if msgs, ok := result["messages"].([]interface{}); ok && len(msgs) > 0 {
		if m, ok := msgs[0].(map[string]interface{}); ok {
			if id, ok := m["id"].(string); ok {
				return id, nil
			}
		}
	}
	return fmt.Sprintf("meta-wa-%d", time.Now().UnixNano()), nil
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
