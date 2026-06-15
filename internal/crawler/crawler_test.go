package crawler

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"treehole/internal/client"
)

func TestGetJSONStringFromInterface(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"nil", nil, ""},
		{"empty array", []interface{}{}, ""},
		{"empty string", "", `""`},
		{"simple string", "hello", `"hello"`},
		{"number", 42, `42`},
		{"map", map[string]interface{}{"key": "val"}, `{"key":"val"}`},
		{"non-empty array", []interface{}{1, 2}, `[1,2]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getJSONStringFromInterface(tt.input)
			if result != tt.expected {
				t.Errorf("getJSONStringFromInterface(%v) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetStringField(t *testing.T) {
	data := map[string]interface{}{
		"name":  "test",
		"empty": "",
	}

	if getStringField(data, "name") != "test" {
		t.Errorf("getStringField(name) != test")
	}
	if getStringField(data, "empty") != "" {
		t.Errorf("getStringField(empty) != empty string")
	}
	if getStringField(data, "missing") != "" {
		t.Errorf("getStringField(missing) != empty string")
	}
}

func TestGetStringFieldNonString(t *testing.T) {
	data := map[string]interface{}{
		"num": 42,
	}
	if getStringField(data, "num") != "" {
		t.Errorf("getStringField(num) should return empty for non-string value")
	}
}

func TestGetIntField(t *testing.T) {
	data := map[string]interface{}{
		"float": float64(42.0),
		"int":   int(100),
	}

	if getIntField(data, "float") != 42 {
		t.Errorf("getIntField(float) = %d, want 42", getIntField(data, "float"))
	}
	if getIntField(data, "int") != 100 {
		t.Errorf("getIntField(int) = %d, want 100", getIntField(data, "int"))
	}
	if getIntField(data, "missing") != 0 {
		t.Errorf("getIntField(missing) = %d, want 0", getIntField(data, "missing"))
	}
}

func TestGetIntFieldNonNumeric(t *testing.T) {
	data := map[string]interface{}{
		"str": "hello",
	}
	if getIntField(data, "str") != 0 {
		t.Errorf("getIntField(str) should return 0 for non-numeric value")
	}
}

func TestSaveMediaStoresPNGWithOriginalExtension(t *testing.T) {
	pngData := mustEncodePNG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngData)
	}))
	defer srv.Close()

	c := mustNewTestClient(t)
	outputDir := t.TempDir()

	if ok := saveMedia(c, srv.URL, 30001, outputDir, false); !ok {
		t.Fatal("saveMedia returned false, want true")
	}

	filename := filepath.Join(outputDir, "30001.png")
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !bytes.Equal(data, pngData) {
		t.Fatal("saved file content mismatch")
	}
}

func TestDownloadMediaByIDRangeDownloadsAndSkipsFailures(t *testing.T) {
	pngData := mustEncodePNG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("id") {
		case "30000", "30001":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := mustNewTestClient(t)
	outputDir := t.TempDir()

	downloaded, skipped := downloadMediaByIDRange(c, 30000, 30002, srv.URL+"?id=%d", outputDir, false)
	if downloaded != 2 {
		t.Fatalf("downloaded=%d, want 2", downloaded)
	}
	if skipped != 1 {
		t.Fatalf("skipped=%d, want 1", skipped)
	}
	for _, id := range []string{"30000.png", "30001.png"} {
		if _, err := os.Stat(filepath.Join(outputDir, id)); err != nil {
			t.Fatalf("expected file %s to exist: %v", id, err)
		}
	}
}

func mustEncodePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	img.Set(1, 1, color.NRGBA{G: 255, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func mustNewTestClient(t *testing.T) *client.Client {
	t.Helper()
	c, err := client.NewClient("test-device")
	if err != nil {
		t.Fatalf("client.NewClient: %v", err)
	}
	return c
}
