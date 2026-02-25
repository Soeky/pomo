package dashboard

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRegistryByID(t *testing.T) {
	t.Parallel()

	r := NewRegistry(nil)
	if _, ok := r.ByID("totals"); !ok {
		t.Fatalf("expected totals module to be registered")
	}
	if _, ok := r.ByID("missing"); ok {
		t.Fatalf("did not expect missing module to be registered")
	}
}

func TestRegistryAllWithoutDB(t *testing.T) {
	t.Parallel()

	r := NewRegistry(nil)
	_, err := r.All(context.Background(), time.Now().Add(-time.Hour), time.Now())
	if err == nil {
		t.Fatalf("expected error without db")
	}
	if !strings.Contains(err.Error(), "database is not initialized") {
		t.Fatalf("unexpected error: %v", err)
	}
}
