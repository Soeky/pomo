package session

import (
	"testing"
	"time"
)

func TestParseCorrectArgs(t *testing.T) {
	t.Parallel()

	req, err := ParseCorrectArgs([]string{"start", "15m", "ProjectX"})
	if err != nil {
		t.Fatalf("ParseCorrectArgs failed: %v", err)
	}
	if req.SessionType != "start" {
		t.Fatalf("unexpected session type: %s", req.SessionType)
	}
	if req.Topic != "ProjectX" {
		t.Fatalf("unexpected topic: %s", req.Topic)
	}
	if req.BackDuration != 15*time.Minute {
		t.Fatalf("unexpected duration: %v", req.BackDuration)
	}
}

func TestParseCorrectArgsInvalidDuration(t *testing.T) {
	t.Parallel()

	_, err := ParseCorrectArgs([]string{"start", "nonsense"})
	if err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestParseCorrectArgsTooShort(t *testing.T) {
	t.Parallel()

	_, err := ParseCorrectArgs([]string{"start"})
	if err == nil {
		t.Fatalf("expected arg length error")
	}
}

func TestCorrectSessionRejectsInvalidType(t *testing.T) {
	t.Parallel()

	_, err := CorrectSession(time.Now(), CorrectRequest{
		SessionType:  "invalid",
		BackDuration: 5 * time.Minute,
		Topic:        "x",
	})
	if err == nil {
		t.Fatalf("expected invalid type error")
	}
}
