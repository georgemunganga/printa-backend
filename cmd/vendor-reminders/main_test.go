package main

import (
	"testing"
	"time"
)

func TestChooseReminderEscalatesThenFallsBackToEvery48Hours(t *testing.T) {
	now := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC)
	for daysPastDue, expected := range map[int]bool{
		0: true,
		1: true,
		2: true,
		3: false,
		4: true,
		5: false,
		6: true,
	} {
		candidate := reminderCandidate{PeriodEnd: now.Add(-time.Duration(daysPastDue) * 24 * time.Hour)}
		kind, _, _, shouldSend := chooseReminder(candidate, now)
		if shouldSend != expected {
			t.Fatalf("day %d expected shouldSend=%t, got %t", daysPastDue, expected, shouldSend)
		}
		if expected && kind != "SUBSCRIPTION_DUE" {
			t.Fatalf("day %d expected subscription-due type, got %s", daysPastDue, kind)
		}
	}
}

func TestChooseReminderSendsGraceExpiryWithin24Hours(t *testing.T) {
	now := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC)
	soon := now.Add(23 * time.Hour)
	later := now.Add(25 * time.Hour)

	kind, _, _, shouldSend := chooseReminder(reminderCandidate{GraceEndsAt: &soon}, now)
	if !shouldSend || kind != "GRACE_EXPIRING" {
		t.Fatalf("expected grace expiry reminder, got kind=%s shouldSend=%t", kind, shouldSend)
	}
	_, _, _, shouldSend = chooseReminder(reminderCandidate{GraceEndsAt: &later}, now)
	if shouldSend {
		t.Fatal("grace reminder must wait until 24 hours before expiry")
	}
}
