package capture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testID = "12345678-1234-4234-8234-123456789abc"
const token = "synthetic-test-credential-32-characters"

var testItem = Item{ID: testID, Kind: "todo", Text: "Buy milk\nNext line", Title: "Shopping"}

func TestUnconfiguredMakesNoRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("unexpected request") }))
	defer server.Close()
	for _, config := range []Config{{}, {Endpoint: server.URL}, {Credential: token}, {Endpoint: "http://example.com", Credential: token}} {
		if err := (Client{Config: config}).Sync(context.Background(), func([]Item) error { t.Fatal("save called"); return nil }); err != nil {
			t.Fatal(err)
		}
	}
}
func TestConfigurationValidation(t *testing.T) {
	for _, endpoint := range []string{"https://pinny.example", "http://localhost:8787", "http://127.0.0.1:8787/", "http://[::1]:8787"} {
		if _, err := (Config{endpoint, token}).Normalize(); err != nil {
			t.Errorf("%s: %v", endpoint, err)
		}
	}
	for _, endpoint := range []string{"http://pinny.example", "https://user:secret@host", "https://host/path", "https://host?secret=value", "https://host/#fragment", "file:///notes"} {
		if (Config{endpoint, token}).Configured() {
			t.Errorf("accepted %s", endpoint)
		}
	}
}
func TestSaveBeforeAckAndRestartAfterLostAck(t *testing.T) {
	saved := false
	acknowledged := false
	failAck := true
	count := 0
	receipts, existing := map[string]bool{}, map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("missing auth")
		}
		switch r.URL.Path {
		case "/api/v1/pending":
			items := []Item{}
			if !acknowledged {
				items = append(items, testItem)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		case "/api/v1/ack":
			if !saved {
				t.Error("ack before save")
			}
			var body struct {
				IDs []string `json:"ids"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body.IDs) != 1 || body.IDs[0] != testID {
				t.Error("bad ack IDs")
			}
			if failAck {
				w.WriteHeader(503)
				return
			}
			acknowledged = true
			_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		default:
			t.Error("unexpected path")
		}
	}))
	defer server.Close()
	save := func(items []Item) error {
		fresh, next := Plan(server.URL, items, receipts, existing)
		count += len(fresh)
		for _, item := range fresh {
			existing[NoteID(server.URL, item.ID)] = true
		}
		receipts = next
		saved = true
		return nil
	}
	config := Config{server.URL, token}
	if err := (Client{Config: config}).Sync(context.Background(), save); err == nil {
		t.Fatal("expected ack failure")
	}
	// A restart and a permanent local deletion leave durable receipts in place.
	existing = map[string]bool{}
	failAck = false
	if err := (Client{Config: config}).Sync(context.Background(), save); err != nil {
		t.Fatal(err)
	}
	if count != 1 || !acknowledged {
		t.Fatalf("deliveries=%d ack=%v", count, acknowledged)
	}
}
func TestFailedSaveNeverAcknowledges(t *testing.T) {
	ack := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/ack" {
			ack = true
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []Item{testItem}})
	}))
	defer server.Close()
	err := (Client{Config: Config{server.URL, token}}).Sync(context.Background(), func([]Item) error { return errors.New("secret note contents") })
	if err == nil || ack || strings.Contains(err.Error(), "secret note") {
		t.Fatalf("unsafe failure: %v, ack=%v", err, ack)
	}
}
func TestInvalidResponsesNeverSaveOrAck(t *testing.T) {
	for _, response := range []string{"not json", `{}`, `{"items":null}`, `{"items":[{"id":"bad"}]}`, `{"items":` + strings.Repeat(" ", 2*1024*1024) + `[]}`} {
		t.Run(fmt.Sprintf("length%d", len(response)), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Error("unexpected ack")
				}
				_, _ = w.Write([]byte(response))
			}))
			defer server.Close()
			if err := (Client{Config: Config{server.URL, token}}).Sync(context.Background(), func([]Item) error { t.Error("unexpected save"); return nil }); err == nil {
				t.Error("accepted invalid response")
			}
		})
	}
}
func TestRedirectNeverForwardsCredential(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("followed redirect") }))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer source.Close()
	err := (Client{Config: Config{source.URL, token}}).Sync(context.Background(), func([]Item) error { return nil })
	if err == nil {
		t.Fatal("expected redirect failure")
	}
}
func TestCancellationAndRedactedErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := (Client{Config: Config{server.URL, token}}).Sync(ctx, func([]Item) error { return nil })
	if err == nil || strings.Contains(err.Error(), token) || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("unredacted or missing error: %v", err)
	}
}
func TestPlanRecoveryAndTodoRepresentation(t *testing.T) {
	endpoint := "https://pinny.example"
	noteID := NoteID(endpoint, testID)
	if !idPattern.MatchString(noteID) || noteID != NoteID(endpoint+"/", testID) || noteID == NoteID("https://other.example", testID) {
		t.Fatal("unstable or unscoped ID")
	}
	fresh, receipts := Plan(endpoint, []Item{testItem}, nil, map[string]bool{noteID: true})
	if len(fresh) != 0 || !receipts[noteID] {
		t.Fatal("failed recovery of saved note")
	}
	fresh, _ = Plan(endpoint, []Item{testItem}, receipts, nil)
	if len(fresh) != 0 {
		t.Fatal("receipt did not survive local deletion")
	}
	if testItem.NoteText() != "[ ] Buy milk\r\nNext line" {
		t.Fatal("todo representation")
	}
	complete := testItem
	complete.Completed = true
	if !strings.HasPrefix(complete.NoteText(), "[x] ") {
		t.Fatal("completed representation")
	}
	if Interval != 15*time.Minute {
		t.Fatal("unexpected interval")
	}
}
