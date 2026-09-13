package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

type grapeMindsClient struct {
	key, base string
	http      *http.Client
	mu        sync.Mutex
	next      time.Time
}

func newGrapeMindsClient() *grapeMindsClient {
	return &grapeMindsClient{key: strings.TrimSpace(os.Getenv("GRAPEMINDS_API_KEY")), base: "https://api.grapeminds.eu/public/v1", http: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

type infoFailure struct {
	status, message string
	candidates      []grapeCandidate
}

func (e *infoFailure) Error() string         { return e.message }
func infoError(status, message string) error { return &infoFailure{status: status, message: message} }

func (c *grapeMindsClient) request(ctx context.Context, method, path string) (json.RawMessage, error) {
	if c == nil || c.key == "" {
		return nil, infoError("not_configured", "Add GRAPEMINDS_API_KEY to backend/.env and restart the backend.")
	}
	c.mu.Lock()
	delay := time.Until(c.next)
	if delay > 0 {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			c.mu.Unlock()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	c.next = time.Now().Add(1100 * time.Millisecond)
	c.mu.Unlock()
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, infoError("failed", "GrapeMinds could not be reached. Please retry later.")
	}
	defer resp.Body.Close()
	log.Printf("GrapeMinds API request method=%s endpoint=%s status=%d", method, strings.Split(path, "?")[0], resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		switch resp.StatusCode {
		case 401, 403:
			return nil, infoError("failed", "GrapeMinds rejected access. Check the API key, subscription, and storage licence terms in your dashboard.")
		case 429:
			wait := 60 * time.Second
			if n, e := strconv.Atoi(resp.Header.Get("Retry-After")); e == nil && n > 0 && n <= 3600 {
				wait = time.Duration(n) * time.Second
			}
			c.mu.Lock()
			c.next = time.Now().Add(wait)
			c.mu.Unlock()
			return nil, infoError("rate_limited", "GrapeMinds usage is limited. Wait before retrying and check your API allowance.")
		case 404:
			return nil, infoError("not_found", "GrapeMinds has no information available for this wine yet. You can retry later.")
		default:
			return nil, infoError("failed", "GrapeMinds could not complete the request. Please retry later.")
		}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil || len(raw) > 1<<20 || !json.Valid(raw) {
		return nil, infoError("failed", "GrapeMinds returned an invalid response. Please retry later.")
	}
	return raw, nil
}

type grapeCandidate struct {
	ID       int64  `json:"id"`
	Name     string `json:"display_name"`
	Color    string `json:"color"`
	Producer string `json:"producer_display_name"`
}

func (c *grapeMindsClient) search(ctx context.Context, query string) ([]grapeCandidate, error) {
	if len([]rune(strings.TrimSpace(query))) < 3 {
		return nil, infoError("not_found", "Enter at least three characters to search GrapeMinds.")
	}
	raw, err := c.request(ctx, "GET", "/wines/search?q="+url.QueryEscape(query)+"&limit=20")
	if err != nil {
		return nil, err
	}
	var result struct {
		Data []grapeCandidate `json:"data"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Data == nil {
		return nil, infoError("failed", "GrapeMinds returned an invalid search result.")
	}
	filtered := []grapeCandidate{}
	for _, candidate := range result.Data {
		if candidate.ID > 0 && candidate.Name != "" {
			filtered = append(filtered, candidate)
		}
	}
	return filtered, nil
}
func normalizedWineName(name string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(name)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func exactCandidate(name, wineType string, candidates []grapeCandidate) int64 {
	var id int64
	for _, c := range candidates {
		if normalizedWineName(c.Name) != normalizedWineName(name) {
			continue
		}
		color := map[string]string{"Red": "red", "White": "white", "Rosé": "rose"}[wineType]
		if color != "" && c.Color != "" && color != c.Color {
			continue
		}
		if id != 0 && id != c.ID {
			return 0
		}
		id = c.ID
	}
	return id
}
func (c *grapeMindsClient) fetch(ctx context.Context, name, wineType string, id int64) (int64, json.RawMessage, error) {
	if c == nil || c.key == "" {
		return 0, nil, infoError("not_configured", "Add GRAPEMINDS_API_KEY to backend/.env and restart the backend.")
	}
	if id == 0 {
		candidates, err := c.search(ctx, name)
		if err != nil {
			return 0, nil, err
		}
		if len(candidates) == 0 {
			return 0, nil, &infoFailure{status: "not_found", message: "No GrapeMinds match found. Try searching with the producer and wine name.", candidates: candidates}
		}
		id = exactCandidate(name, wineType, candidates)
		if id == 0 {
			return 0, nil, &infoFailure{status: "needs_match", message: "Choose the correct GrapeMinds wine below before its information is attached.", candidates: candidates}
		}
	}
	raw, err := c.request(ctx, "GET", fmt.Sprintf("/wines/%d", id))
	if err != nil {
		return 0, nil, err
	}
	// The detail endpoint is documented as a direct object; also accept a data envelope.
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(raw, &envelope)
	if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		raw = envelope.Data
	}
	var wine struct {
		ID   int64  `json:"id"`
		Name string `json:"display_name"`
	}
	if json.Unmarshal(raw, &wine) != nil || wine.ID != id || strings.TrimSpace(wine.Name) == "" {
		return 0, nil, infoError("failed", "GrapeMinds returned an invalid wine record.")
	}
	return id, raw, nil
}

func failureDetails(err error) (string, string) {
	var failure *infoFailure
	if errors.As(err, &failure) {
		return failure.status, failure.message
	}
	return "failed", "Wine information could not be fetched. Please retry."
}
