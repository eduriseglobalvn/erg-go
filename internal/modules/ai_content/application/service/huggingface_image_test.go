package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"erg.ninja/pkg/config"
)

func TestHuggingFaceImageClientGenerateToTempFile(t *testing.T) {
	const imageBody = "\x89PNG\r\n\x1a\nfake-image"

	var sawAuth bool
	var sawPrompt bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/generated.png" {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte(imageBody))
			return
		}
		if r.URL.Path != "/fal-ai/fal-ai/flux/krea" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		sawAuth = strings.HasPrefix(r.Header.Get("Authorization"), "Bearer hf_test")
		var body struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		sawPrompt = body.Prompt == "students learning online"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"images": []map[string]string{
				{"url": serverURL(r) + "/generated.png", "content_type": "image/png"},
			},
		})
	}))
	defer server.Close()

	client := newHuggingFaceImageClient(config.AiConfig{
		HuggingFaceImageAPIKey:   "hf_test",
		HuggingFaceImageProvider: "fal-ai",
		HuggingFaceImageModel:    "black-forest-labs/FLUX.1-Krea-dev",
		HuggingFaceImageBaseURL:  server.URL,
		HuggingFaceImageTimeout:  2 * time.Second,
	}, nil)

	path, mime, err := client.GenerateToTempFile(context.Background(), "students learning online")
	if err != nil {
		t.Fatalf("GenerateToTempFile error: %v", err)
	}
	defer removeTempFile(path)
	if !sawAuth {
		t.Fatal("expected bearer auth header")
	}
	if !sawPrompt {
		t.Fatal("expected prompt in request")
	}
	if mime != "image/png" {
		t.Fatalf("expected image/png, got %q", mime)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temp image: %v", err)
	}
	if string(raw) != imageBody {
		t.Fatalf("unexpected temp file body: %q", string(raw))
	}

	removeTempFile(path)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected temp file to be removed, stat err=%v", err)
	}
}

func TestHuggingFaceImageClientRejectsNonImageResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"not ready"}`))
	}))
	defer server.Close()

	client := newHuggingFaceImageClient(config.AiConfig{
		HuggingFaceImageAPIKey:   "hf_test",
		HuggingFaceImageProvider: "fal-ai",
		HuggingFaceImageModel:    "black-forest-labs/FLUX.1-Krea-dev",
		HuggingFaceImageBaseURL:  server.URL,
		HuggingFaceImageTimeout:  2 * time.Second,
	}, nil)

	if _, _, err := client.GenerateToTempFile(context.Background(), "prompt"); err == nil {
		t.Fatal("expected error for non-image response")
	}
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host
}
