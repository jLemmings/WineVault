package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func labelImage(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 80, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 80; x++ {
			img.Set(x, y, color.RGBA{240, 230, 210, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func scanRequest(t *testing.T, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "label.png")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(data)
	writer.Close()
	r := httptest.NewRequest("POST", "/api/wine-scan", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	return r
}
func scannerStore(provider *httptest.Server) *Store {
	return &Store{scanner: &LabelScanner{apiKey: "test-secret-not-a-real-key", model: "test-vision", endpoint: provider.URL, client: provider.Client(), slots: make(chan struct{}, 2)}}
}
func providerResponse(w http.ResponseWriter, result WineRecognition) {
	text, _ := json.Marshal(result)
	message := map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": string(text)}}}
	writeJSON(w, 200, map[string]any{"status": "completed", "output": []any{message}})
}
func sampleRecognition() WineRecognition {
	year := 2018
	return WineRecognition{Status: "recognized", Name: "Test Estate Reserve", Vintage: &year, Region: "Bordeaux, France", Type: "Red", Confidence: "medium", Notes: "Verify the vintage on the label."}
}

func TestWineScannerRequestAndResponse(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer test-secret-not-a-real-key" {
			t.Error("missing server-side credential")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["model"] != "test-vision" || body["store"] != false {
			t.Error("incorrect model or storage flag")
		}
		input := body["input"].([]any)[0].(map[string]any)["content"].([]any)[1].(map[string]any)
		if input["type"] != "input_image" || !strings.HasPrefix(input["image_url"].(string), "data:image/jpeg;base64,") {
			t.Error("image was not normalized for vision input")
		}
		format := body["text"].(map[string]any)["format"].(map[string]any)
		if format["type"] != "json_schema" || format["strict"] != true {
			t.Error("structured output missing")
		}
		providerResponse(w, sampleRecognition())
	}))
	defer provider.Close()
	s := scannerStore(provider)
	w := httptest.NewRecorder()
	s.dataRoutes().ServeHTTP(w, scanRequest(t, labelImage(t)))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result WineRecognition
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Name != "Test Estate Reserve" || result.Vintage == nil || *result.Vintage != 2018 || calls.Load() != 1 {
		t.Fatal("wrong recognition", result)
	}
	if strings.Contains(w.Body.String(), "test-secret") {
		t.Fatal("credential leaked")
	}
}

func TestWineScannerRejectsInvalidUploads(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer provider.Close()
	s := scannerStore(provider)
	for _, tc := range []struct {
		name   string
		data   []byte
		status int
	}{{"not an image", []byte("fake image"), 415}, {"too large", bytes.Repeat([]byte("a"), maxScanImage+1), 413}, {"truncated", labelImage(t)[:40], 415}} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			s.dataRoutes().ServeHTTP(w, scanRequest(t, tc.data))
			if w.Code != tc.status {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatal("invalid image sent to provider")
	}
}

func TestWineScannerUnavailableAndProviderErrors(t *testing.T) {
	s := &Store{}
	w := call(s, "GET", "/api/wine-scan/status", "")
	if !strings.Contains(w.Body.String(), `"available":false`) {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	s.dataRoutes().ServeHTTP(w, scanRequest(t, labelImage(t)))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	for _, status := range []int{401, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				io.WriteString(w, "secret provider error")
			}))
			defer provider.Close()
			s := scannerStore(provider)
			w := httptest.NewRecorder()
			s.dataRoutes().ServeHTTP(w, scanRequest(t, labelImage(t)))
			expected := 502
			if status == 401 {
				expected = 503
			}
			if status == 429 {
				expected = 429
			}
			if w.Code != expected || strings.Contains(w.Body.String(), "secret") {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
}

func TestWineScannerHandlesUncertaintyAndBadResults(t *testing.T) {
	for _, tc := range []struct {
		name   string
		body   any
		status int
	}{
		{"incomplete", map[string]any{"status": "incomplete"}, 502},
		{"bad JSON", map[string]any{"status": "completed", "output": []any{map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": "not JSON"}}}}}, 502},
		{"refused", map[string]any{"status": "completed", "output": []any{map[string]any{"type": "message", "content": []any{map[string]any{"type": "refusal"}}}}}, 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, tc.body) }))
			defer provider.Close()
			s := scannerStore(provider)
			w := httptest.NewRecorder()
			s.dataRoutes().ServeHTTP(w, scanRequest(t, labelImage(t)))
			if w.Code != tc.status {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := sampleRecognition()
		result.Status = "unreadable"
		providerResponse(w, result)
	}))
	defer provider.Close()
	s := scannerStore(provider)
	w := httptest.NewRecorder()
	s.dataRoutes().ServeHTTP(w, scanRequest(t, labelImage(t)))
	var result WineRecognition
	json.Unmarshal(w.Body.Bytes(), &result)
	if w.Code != 200 || result.Status != "unreadable" || result.Name != "" || result.Vintage != nil {
		t.Fatal("uncertain result filled invented details", w.Body.String())
	}
}

func TestRecognitionValidation(t *testing.T) {
	result := sampleRecognition()
	result.Vintage = nil
	if !validRecognition(result) {
		t.Fatal("unknown vintage should remain reviewable")
	}
	result.NonVintage = true
	if !validRecognition(result) {
		t.Fatal("NV should be accepted")
	}
	year := 2019
	result.Vintage = &year
	if validRecognition(result) {
		t.Fatal("ambiguous NV plus vintage accepted")
	}
	result.NonVintage = false
	year = time.Now().Year() + 2
	if validRecognition(result) {
		t.Fatal("future vintage accepted")
	}
}

func TestScannerProviderDiagnostics(t *testing.T) {
	for _, tc := range []struct{ code, message string }{
		{"insufficient_quota", "credit or spending limit"},
		{"rate_limit_exceeded", "temporarily limited"},
		{"project_spend_limit_exceeded", "credit or spending limit"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			var logs bytes.Buffer
			previous := log.Writer()
			log.SetOutput(&logs)
			defer log.SetOutput(previous)
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("x-request-id", "req-test")
				w.Header().Set("Retry-After", "30")
				writeJSON(w, 429, map[string]any{"error": map[string]any{
					"code": tc.code, "type": "test-secret-not-a-real-key", "message": "private photo and credential content",
				}})
			}))
			defer provider.Close()
			w := httptest.NewRecorder()
			scannerStore(provider).dataRoutes().ServeHTTP(w, scanRequest(t, labelImage(t)))
			if w.Code != 429 || !strings.Contains(w.Body.String(), tc.message) {
				t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
			}
			for _, expected := range []string{"status=429", `request_id="req-test"`, tc.code, `retry_after="30"`, "[REDACTED]"} {
				if !strings.Contains(logs.String(), expected) {
					t.Errorf("missing diagnostic %s", expected)
				}
			}
			for _, secret := range []string{"test-secret-not-a-real-key", "private photo"} {
				if strings.Contains(logs.String()+w.Body.String(), secret) {
					t.Error("sensitive content leaked")
				}
			}
		})
	}
}

func TestScannedWineCanBeSavedAsNonVintage(t *testing.T) {
	pool := testPool(t)
	if err := migrate(context.Background(), pool, ""); err != nil {
		t.Fatal(err)
	}
	s := &Store{db: pool}
	w := call(s, "POST", "/api/bottles", `{"name":"Reviewed label","vintage":0,"region":"Champagne, France","type":"Sparkling","rack":"C","slot":29}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var bottle Bottle
	json.Unmarshal(w.Body.Bytes(), &bottle)
	var vintage int
	if err := pool.QueryRow(context.Background(), "SELECT vintage FROM bottles WHERE id=$1", bottle.ID).Scan(&vintage); err != nil || vintage != 0 {
		t.Fatal("NV not persisted", err)
	}
}
