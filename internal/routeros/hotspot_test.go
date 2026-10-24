package routerosadapter

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHotSpotSnapshotRejectsInvalidRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := HotSpotSnapshot(ctx, TestRequest{})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("HotSpotSnapshot error = %v, want ErrInvalidRequest", err)
	}
}

func TestPreviewRowsCapsLargeLists(t *testing.T) {
	rows := make([]map[string]string, hotspotPreviewLimit+4)
	got := previewRows(rows)
	if len(got) != hotspotPreviewLimit {
		t.Fatalf("preview length = %d, want %d", len(got), hotspotPreviewLimit)
	}
}

func TestRouterOSBool(t *testing.T) {
	for _, value := range []string{"true", "yes", "1"} {
		if !routerOSBool(value) {
			t.Errorf("routerOSBool(%q) = false, want true", value)
		}
	}
	for _, value := range []string{"", "false", "no", "0"} {
		if routerOSBool(value) {
			t.Errorf("routerOSBool(%q) = true, want false", value)
		}
	}
}
