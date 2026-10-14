// Package capture implements the portable, incoming-only Pinny inbox protocol.
package capture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const Interval = 15 * time.Minute
const BatchSize = 50

var idPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type Config struct {
	Endpoint   string `json:"Endpoint"`
	Credential string `json:"Credential"`
}

// Normalize permits plaintext HTTP only for an explicit loopback development server.
func (c Config) Normalize() (Config, error) {
	u, err := url.Parse(c.Endpoint)
	if err != nil || u == nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		(u.Path != "" && u.Path != "/") || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) ||
		len(c.Credential) < 32 || strings.ContainsAny(c.Credential, "\r\n") {
		return Config{}, errors.New("configure an HTTPS origin and an app credential of at least 32 characters (HTTP is allowed only on loopback)")
	}
	c.Endpoint = strings.TrimSuffix(u.String(), "/")
	return c, nil
}
func (c Config) Configured() bool { _, err := c.Normalize(); return err == nil }

type Item struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Text      string `json:"text"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

func (i Item) Valid() bool {
	return idPattern.MatchString(i.ID) && (i.Kind == "note" || i.Kind == "todo") &&
		strings.TrimSpace(i.Text) != "" && len(i.Text) <= 16000 && utf8.ValidString(i.Text) && !strings.ContainsRune(i.Text, 0) &&
		utf8.RuneCountInString(i.Title) <= 100 && !strings.ContainsAny(i.Title, "\r\n\x00") && !(i.Kind == "note" && i.Completed)
}
func (i Item) NoteText() string {
	text := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(i.Text, "\r\n", "\n"), "\r", "\n"), "\n", "\r\n")
	if i.Kind == "todo" {
		if i.Completed {
			return "[x] " + text
		}
		return "[ ] " + text
	}
	return text
}

// NoteID namespaces deliveries by endpoint and is stable across retries, crashes,
// credential rotation, and restoring a backup. It is an ordinary UUID-shaped ID.
func NoteID(endpoint, deliveryID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSuffix(endpoint, "/") + "\x00" + deliveryID))
	digest[6] = (digest[6] & 15) | 0x50
	digest[8] = (digest[8] & 63) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", digest[:4], digest[4:6], digest[6:8], digest[8:10], digest[10:16])
}

type Client struct {
	Config Config
	HTTP   *http.Client
}

// Sync processes at most 20 bounded batches per action. Save must durably save
// both notes and receipts before returning nil; it must tolerate repeat delivery.
// No request is made for an unconfigured client. Errors deliberately omit URLs,
// response bodies, credentials and note contents.
func (c Client) Sync(ctx context.Context, save func([]Item) error) error {
	if !c.Config.Configured() {
		return nil
	}
	config, _ := c.Config.Normalize()
	c.Config = config
	for batch := 0; batch < 20; batch++ {
		var result struct {
			Items *[]Item `json:"items"`
		}
		if err := c.request(ctx, "GET", "/api/v1/pending", nil, &result); err != nil {
			return err
		}
		if result.Items == nil || len(*result.Items) > BatchSize {
			return errors.New("invalid inbox response")
		}
		items := *result.Items
		if len(items) == 0 {
			return nil
		}
		ids := make([]string, len(items))
		seen := map[string]bool{}
		for n, item := range items {
			if !item.Valid() || seen[item.ID] {
				return errors.New("invalid inbox response")
			}
			seen[item.ID] = true
			ids[n] = item.ID
		}
		if err := save(items); err != nil {
			return errors.New("incoming items could not be saved; they remain available for retry")
		}
		if err := c.request(ctx, "POST", "/api/v1/ack", struct {
			IDs []string `json:"ids"`
		}{ids}, nil); err != nil {
			return err
		}
	}
	return nil // More items will be collected on the next action.
}
func (c Client) request(ctx context.Context, method, path string, data, out any) error {
	var body io.Reader
	if data != nil {
		encoded, err := json.Marshal(data)
		if err != nil {
			return errors.New("invalid sync request")
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Config.Endpoint+path, body)
	if err != nil {
		return errors.New("invalid sync configuration")
	}
	req.Header.Set("Authorization", "Bearer "+c.Config.Credential)
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	// Do not forward app credentials to redirects, including same-origin redirects.
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := copyClient.Do(req)
	if err != nil {
		return errors.New("sync request failed; check your connection and retry")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("sync request failed (HTTP %d)", res.StatusCode)
	}
	bytes, err := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024+1))
	if err != nil || len(bytes) > 2*1024*1024 {
		return errors.New("invalid inbox response")
	}
	if out != nil && json.Unmarshal(bytes, out) != nil {
		return errors.New("invalid inbox response")
	}
	return nil
}
