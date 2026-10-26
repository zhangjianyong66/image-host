package app

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testUploadToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
const secondUploadToken = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

func newTokenTestApp(t *testing.T) *App {
	t.Helper()
	a := newTestApp(t)
	a.config.UploadTokens = []string{testUploadToken, secondUploadToken}
	return a
}

type mediaTestFile struct {
	field, filename, contentType string
	data                         []byte
}

func mediaRequest(t *testing.T, path string, files ...mediaTestFile) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, file := range files {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, file.field, file.filename))
		header.Set("Content-Type", file.contentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(file.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+testUploadToken)
	return req
}

func assertUploadDirectoryEmpty(t *testing.T, a *App) {
	t.Helper()
	files, err := os.ReadDir(a.config.UploadDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("failed request left %d entries", len(files))
	}
}

func TestLoadUploadTokens(t *testing.T) {
	t.Setenv("UPLOAD_PASSWORD", "password")
	t.Setenv("SESSION_SECRET", testSessionSecret)
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("UPLOAD_DIR", t.TempDir())
	t.Setenv("PUBLIC_BASE_URL", "https://example.test")
	for _, test := range []struct {
		name, raw string
		want      int
		invalid   bool
	}{
		{name: "disabled"},
		{name: "whitespace disabled", raw: "  "},
		{name: "single", raw: testUploadToken, want: 1},
		{name: "multiple trimmed", raw: " " + testUploadToken + " , " + secondUploadToken + " ", want: 2},
		{name: "short", raw: "short", invalid: true},
		{name: "empty item", raw: testUploadToken + ",," + secondUploadToken, invalid: true},
		{name: "trailing item", raw: testUploadToken + ",", invalid: true},
		{name: "whitespace", raw: testUploadToken + " secret", invalid: true},
		{name: "invalid characters", raw: testUploadToken + ":", invalid: true},
		{name: "invalid padding", raw: testUploadToken + "=x", invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("NEWAPI_MEDIA_UPLOAD_TOKENS", test.raw)
			cfg, err := LoadConfig()
			if (err != nil) != test.invalid {
				t.Fatalf("error = %v, invalid = %v", err, test.invalid)
			}
			if test.invalid {
				if strings.Contains(err.Error(), testUploadToken) {
					t.Fatal("error exposed token")
				}
				return
			}
			if len(cfg.UploadTokens) != test.want {
				t.Fatalf("loaded %d tokens, want %d", len(cfg.UploadTokens), test.want)
			}
			if test.want > 0 && cfg.UploadTokens[0] != testUploadToken {
				t.Fatal("did not preserve trimmed token")
			}
		})
	}
}

func TestConfigRejectsInvalidDirectUploadTokens(t *testing.T) {
	a := newTestApp(t)
	for _, token := range []string{"", "short", testUploadToken + " ", testUploadToken + "\r\n"} {
		cfg := a.config
		cfg.UploadTokens = []string{token}
		if _, err := New(cfg, nil); err == nil {
			t.Fatal("New accepted invalid tokens")
		}
	}
}

type unreadableBody struct{ read bool }

func (body *unreadableBody) Read([]byte) (int, error) {
	body.read = true
	return 0, fmt.Errorf("body must not be parsed")
}
func (body *unreadableBody) Close() error { return nil }

func TestMediaUploadRequiresBearerBeforeReadingBody(t *testing.T) {
	for _, path := range []string{"/api/v1/media/images", "/api/v1/media/images/batch"} {
		for _, test := range []struct {
			name, header                string
			cookie, disabled, duplicate bool
		}{
			{name: "missing"},
			{name: "wrong", header: "Bearer " + secondUploadToken + "wrong"},
			{name: "basic", header: "Basic " + testUploadToken},
			{name: "empty", header: "Bearer"},
			{name: "extra", header: "Bearer " + testUploadToken + " extra"},
			{name: "cookie alone", cookie: true},
			{name: "disabled", header: "Bearer " + testUploadToken, disabled: true},
			{name: "duplicate", header: "Bearer " + testUploadToken, duplicate: true},
		} {
			t.Run(path+"/"+test.name, func(t *testing.T) {
				a := newTokenTestApp(t)
				if test.disabled {
					a.config.UploadTokens = nil
				}
				body := &unreadableBody{}
				req := httptest.NewRequest(http.MethodPost, path, body)
				req.Header.Set("Authorization", test.header)
				if test.duplicate {
					req.Header.Add("Authorization", "Bearer "+secondUploadToken)
				}
				if test.cookie {
					req.AddCookie(performLogin(t, a))
				}
				recorder := httptest.NewRecorder()
				a.Handler().ServeHTTP(recorder, req)
				if recorder.Code != http.StatusUnauthorized || body.read {
					t.Fatalf("status = %d, body read = %v", recorder.Code, body.read)
				}
				if recorder.Header().Get("WWW-Authenticate") != "Bearer" {
					t.Fatal("missing Bearer challenge")
				}
				assertUploadDirectoryEmpty(t, a)
			})
		}
	}
}

func TestMediaUploadSuccessAndAnonymousRead(t *testing.T) {
	for _, authorization := range []string{"Bearer " + testUploadToken, "bearer " + secondUploadToken} {
		a := newTokenTestApp(t)
		req := mediaRequest(t, "/api/v1/media/images", mediaTestFile{"file", "picture.png", "image/png", testPNG})
		req.Header.Set("Authorization", authorization)
		recorder := httptest.NewRecorder()
		a.Handler().ServeHTTP(recorder, req)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		var result mediaUploadResult
		if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		name := strings.TrimPrefix(result.URL, "https://image.example.test/images/")
		if result.Filename != "picture.png" || !storedNamePattern.MatchString(name) || !strings.HasSuffix(name, ".png") {
			t.Fatalf("result = %#v", result)
		}
		files, err := os.ReadDir(a.config.UploadDir)
		if err != nil || len(files) != 1 {
			t.Fatalf("entries = %v, error = %v", files, err)
		}
		info, err := os.Stat(filepath.Join(a.config.UploadDir, name))
		if err != nil || info.Mode().Perm() != 0o640 {
			t.Fatalf("metadata = %v, error = %v", info, err)
		}
		restarted, err := New(a.config, nil)
		if err != nil {
			t.Fatal(err)
		}
		recorder = httptest.NewRecorder()
		restarted.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/images/"+name, nil))
		if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "image/png" || !bytes.Equal(recorder.Body.Bytes(), testPNG) {
			t.Fatalf("public read = %d", recorder.Code)
		}
	}
}

func TestMediaBatchPreservesOrderAndNineFileLimit(t *testing.T) {
	a := newTokenTestApp(t)
	var files []mediaTestFile
	for i := 0; i < maxUploadFiles; i++ {
		files = append(files, mediaTestFile{"file", fmt.Sprintf("%d.txt", i), "text/plain", []byte(fmt.Sprintf("contents-%d", i))})
	}
	req := mediaRequest(t, "/api/v1/media/images/batch", files...)
	recorder := httptest.NewRecorder()
	a.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct{ Items []mediaUploadResult }
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != maxUploadFiles {
		t.Fatalf("result count = %d", len(response.Items))
	}
	for i, result := range response.Items {
		if result.Filename != files[i].filename {
			t.Fatalf("item %d filename = %s", i, result.Filename)
		}
		name := strings.TrimPrefix(result.URL, "https://image.example.test/images/")
		content, err := os.ReadFile(filepath.Join(a.config.UploadDir, name))
		if err != nil || !bytes.Equal(content, files[i].data) {
			t.Fatalf("item %d content = %q, error = %v", i, content, err)
		}
	}
	entries, err := os.ReadDir(a.config.UploadDir)
	if err != nil || len(entries) != maxUploadFiles {
		t.Fatalf("entries = %d, error = %v", len(entries), err)
	}
}

func TestMediaUploadRejectsInvalidRequestsWithoutFiles(t *testing.T) {
	valid := mediaTestFile{"file", "good.png", "image/png", testPNG}
	for _, test := range []struct {
		name, path string
		files      []mediaTestFile
		status     int
	}{
		{name: "empty", path: "/api/v1/media/images", status: 400},
		{name: "empty batch", path: "/api/v1/media/images/batch", status: 400},
		{name: "wrong field", path: "/api/v1/media/images", files: []mediaTestFile{{"image", "bad.png", "image/png", testPNG}}, status: 400},
		{name: "no filename", path: "/api/v1/media/images", files: []mediaTestFile{{"file", "", "image/png", testPNG}}, status: 400},
		{name: "empty file", path: "/api/v1/media/images", files: []mediaTestFile{{"file", "empty.png", "image/png", nil}}, status: 400},
		{name: "unsupported SVG", path: "/api/v1/media/images", files: []mediaTestFile{{"file", "bad.svg", "image/svg+xml", []byte("<svg/>")}}, status: 415},
		{name: "spoofed image", path: "/api/v1/media/images", files: []mediaTestFile{{"file", "bad.png", "image/png", []byte("not png")}}, status: 415},
		{name: "invalid UTF8", path: "/api/v1/media/images", files: []mediaTestFile{{"file", "bad.txt", "text/plain", []byte{0xff}}}, status: 415},
		{name: "oversize image", path: "/api/v1/media/images", files: []mediaTestFile{{"file", "big.png", "image/png", make([]byte, maxImageBytes+1)}}, status: 413},
		{name: "two on single", path: "/api/v1/media/images", files: []mediaTestFile{valid, valid}, status: 400},
		{name: "batch failed second", path: "/api/v1/media/images/batch", files: []mediaTestFile{valid, {"file", "bad.png", "image/png", []byte("bad")}}, status: 415},
		{name: "batch wrong second field", path: "/api/v1/media/images/batch", files: []mediaTestFile{valid, {"image", "bad.png", "image/png", testPNG}}, status: 400},
		{name: "ten files", path: "/api/v1/media/images/batch", files: []mediaTestFile{valid, valid, valid, valid, valid, valid, valid, valid, valid, valid}, status: 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := newTokenTestApp(t)
			req := mediaRequest(t, test.path, test.files...)
			recorder := httptest.NewRecorder()
			a.Handler().ServeHTTP(recorder, req)
			if recorder.Code != test.status {
				t.Fatalf("status = %d, want %d, body = %s", recorder.Code, test.status, recorder.Body.String())
			}
			var response map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response["error"] == "" || response["error"] == nil || response["url"] != nil || response["items"] != nil {
				t.Fatalf("failure response = %s, error = %v", recorder.Body.String(), err)
			}
			assertUploadDirectoryEmpty(t, a)
		})
	}
}

func TestMediaUploadMalformedBodyAndStorageFailure(t *testing.T) {
	t.Run("truncated second part", func(t *testing.T) {
		a := newTokenTestApp(t)
		req := mediaRequest(t, "/api/v1/media/images/batch", mediaTestFile{"file", "good.png", "image/png", testPNG}, mediaTestFile{"file", "second.png", "image/png", testPNG})
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		req.Body = io.NopCloser(bytes.NewReader(body[:len(body)-10]))
		recorder := httptest.NewRecorder()
		a.Handler().ServeHTTP(recorder, req)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		assertUploadDirectoryEmpty(t, a)
	})
	t.Run("not multipart", func(t *testing.T) {
		a := newTokenTestApp(t)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/media/images", strings.NewReader("not multipart"))
		req.Header.Set("Authorization", "Bearer "+testUploadToken)
		recorder := httptest.NewRecorder()
		a.Handler().ServeHTTP(recorder, req)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", recorder.Code)
		}
		assertUploadDirectoryEmpty(t, a)
	})
	t.Run("storage unavailable", func(t *testing.T) {
		a := newTokenTestApp(t)
		a.config.UploadDir = filepath.Join(a.config.UploadDir, "missing")
		req := mediaRequest(t, "/api/v1/media/images", mediaTestFile{"file", "good.png", "image/png", testPNG})
		recorder := httptest.NewRecorder()
		a.Handler().ServeHTTP(recorder, req)
		if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), a.config.UploadDir) {
			t.Fatalf("unsafe storage error = %d %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestUploadTokenDoesNotReplaceBrowserCookie(t *testing.T) {
	a := newTokenTestApp(t)
	req := uploadRequest(t, "/api/images", "image", "picture.png", "image/png", testPNG)
	req.Header.Set("Authorization", "Bearer "+testUploadToken)
	recorder := httptest.NewRecorder()
	a.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("browser accepted Bearer, status = %d", recorder.Code)
	}
	assertUploadDirectoryEmpty(t, a)
	req = uploadRequest(t, "/api/images", "image", "picture.png", "image/png", testPNG)
	req.AddCookie(performLogin(t, a))
	recorder = httptest.NewRecorder()
	a.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated || !strings.Contains(recorder.Body.String(), `"results"`) {
		t.Fatalf("Cookie upload = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestMediaPublicationFailureRollsBackWithoutOverwritingExistingFile(t *testing.T) {
	a := newTokenTestApp(t)
	existingName := strings.Repeat("01", 16) + ".png"
	existingContents := []byte("existing public file")
	if err := os.WriteFile(filepath.Join(a.config.UploadDir, existingName), existingContents, 0o640); err != nil {
		t.Fatal(err)
	}
	file := mediaTestFile{"file", "picture.png", "image/png", testPNG}
	req := mediaRequest(t, "/api/v1/media/images/batch", file, file)

	// No tests in this package run in parallel. Control only the generated
	// filenames so the second publication collides after the first succeeds.
	previousReader := rand.Reader
	rand.Reader = bytes.NewReader(append(make([]byte, 16), bytes.Repeat([]byte{1}, 16)...))
	t.Cleanup(func() { rand.Reader = previousReader })
	recorder := httptest.NewRecorder()
	a.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	files, err := os.ReadDir(a.config.UploadDir)
	if err != nil || len(files) != 1 || files[0].Name() != existingName {
		t.Fatalf("publication failure left entries = %v, error = %v", files, err)
	}
	contents, err := os.ReadFile(filepath.Join(a.config.UploadDir, existingName))
	if err != nil || !bytes.Equal(contents, existingContents) {
		t.Fatalf("existing file changed: contents = %q, error = %v", contents, err)
	}
}
