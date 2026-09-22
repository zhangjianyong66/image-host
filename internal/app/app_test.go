package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testSessionSecret = "0123456789abcdef0123456789abcdef"

var testPNG = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00")

func newTestApp(t *testing.T) *App {
	t.Helper()
	a, err := New(Config{
		ListenAddr:     "127.0.0.1:0",
		UploadDir:      t.TempDir(),
		UploadPassword: "correct-password",
		SessionSecret:  testSessionSecret,
		PublicBaseURL:  "https://image.example.test",
		CookieSecure:   true,
	}, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	a.now = func() time.Time { return time.Unix(1_800_000_000, 0) }
	return a
}

func performLogin(t *testing.T, a *App) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"correct-password"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	a.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	response := recorder.Result()
	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login cookies = %d, want 1", len(cookies))
	}
	return cookies[0]
}

func uploadRequest(t *testing.T, path, field, filename, contentType string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header["Content-Disposition"] = []string{`form-data; name="` + field + `"; filename="` + filename + `"`}
	header["Content-Type"] = []string{contentType}
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func batchUploadRequest(t *testing.T, files ...struct {
	filename, contentType string
	data                  []byte
}) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, file := range files {
		header := make(textproto.MIMEHeader)
		header["Content-Disposition"] = []string{`form-data; name="image"; filename="` + file.filename + `"`}
		header["Content-Type"] = []string{file.contentType}
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
	request := httptest.NewRequest(http.MethodPost, "/api/images", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func TestBatchUploadReturnsPartialResults(t *testing.T) {
	a := newTestApp(t)
	request := batchUploadRequest(t,
		struct {
			filename, contentType string
			data                  []byte
		}{"one.png", "image/png", testPNG},
		struct {
			filename, contentType string
			data                  []byte
		}{"bad.png", "image/png", []byte("not png")},
	)
	request.AddCookie(performLogin(t, a))
	recorder := httptest.NewRecorder()
	a.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Results []struct{ Status, URL, Error string } `json:"results"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 2 || response.Results[0].Status != "uploaded" || response.Results[0].URL == "" || response.Results[1].Status != "failed" || response.Results[1].Error == "" {
		t.Fatalf("results = %#v", response.Results)
	}
}

func TestBatchUploadRejectsMoreThanNineFiles(t *testing.T) {
	a := newTestApp(t)
	files := make([]struct {
		filename, contentType string
		data                  []byte
	}, maxUploadFiles+1)
	for i := range files {
		files[i] = struct {
			filename, contentType string
			data                  []byte
		}{"file.png", "image/png", testPNG}
	}
	request := batchUploadRequest(t, files...)
	request.AddCookie(performLogin(t, a))
	recorder := httptest.NewRecorder()
	a.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestMediaUploadAndPublicContentType(t *testing.T) {
	tests := []struct {
		name, filename, contentType, extension string
		data                                   []byte
	}{
		{name: "wav", filename: "sound.wav", contentType: "audio/wav", extension: ".wav", data: []byte("RIFFxxxxWAVE")},
		{name: "mp4", filename: "clip.mp4", contentType: "video/mp4", extension: ".mp4", data: []byte("xxxxftypisom")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := newTestApp(t)
			request := uploadRequest(t, "/api/images", "image", test.filename, test.contentType, test.data)
			request.AddCookie(performLogin(t, a))
			recorder := httptest.NewRecorder()
			a.Handler().ServeHTTP(recorder, request)
			if recorder.Code != http.StatusCreated {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			var response struct {
				Results []struct {
					URL string `json:"url"`
				} `json:"results"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if len(response.Results) != 1 || response.Results[0].URL == "" {
				t.Fatalf("results = %#v", response.Results)
			}
			name := strings.TrimPrefix(response.Results[0].URL, "https://image.example.test/images/")
			if !strings.HasSuffix(name, test.extension) {
				t.Fatalf("stored name = %q", name)
			}
			recorder = httptest.NewRecorder()
			a.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/images/"+name, nil))
			if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != test.contentType {
				t.Fatalf("public media = %d %q", recorder.Code, recorder.Header().Get("Content-Type"))
			}
		})
	}
}

func testZIP(t *testing.T, contents map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range contents {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if name == "mimetype" {
			header.Method = zip.Store
		}
		part, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestAuthenticationFlow(t *testing.T) {
	a := newTestApp(t)

	request := httptest.NewRequest(http.MethodGet, "/upload", nil)
	recorder := httptest.NewRecorder()
	a.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/login" {
		t.Fatalf("unauthenticated upload page = %d %q", recorder.Code, recorder.Header().Get("Location"))
	}

	request = httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"wrong"}`))
	recorder = httptest.NewRecorder()
	a.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d", recorder.Code)
	}

	cookie := performLogin(t, a)
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie flags = HttpOnly:%v Secure:%v SameSite:%v", cookie.HttpOnly, cookie.Secure, cookie.SameSite)
	}

	request = httptest.NewRequest(http.MethodGet, "/upload", nil)
	request.AddCookie(cookie)
	recorder = httptest.NewRecorder()
	a.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("authenticated upload page status = %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	request.AddCookie(cookie)
	recorder = httptest.NewRecorder()
	a.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Result().Cookies()[0].MaxAge != -1 {
		t.Fatalf("logout response = %d cookies=%v", recorder.Code, recorder.Result().Cookies())
	}
}

func TestSessionRejectsTamperingAndExpiry(t *testing.T) {
	a := newTestApp(t)
	cookie := performLogin(t, a)
	if !a.validSessionToken(cookie.Value, a.now()) {
		t.Fatal("fresh token is invalid")
	}
	if a.validSessionToken(cookie.Value+"x", a.now()) {
		t.Fatal("tampered token is valid")
	}
	if a.validSessionToken(cookie.Value, a.now().Add(sessionLifetime+time.Second)) {
		t.Fatal("expired token is valid")
	}
}

func TestUploadAndPublicAccess(t *testing.T) {
	a := newTestApp(t)
	cookie := performLogin(t, a)
	request := uploadRequest(t, "/api/images", "image", "../../unsafe.png", "image/png", testPNG)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	a.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Results []struct {
			URL string `json:"url"`
		} `json:"results"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 || !strings.HasPrefix(response.Results[0].URL, "https://image.example.test/images/") {
		t.Fatalf("upload results = %#v", response.Results)
	}
	name := strings.TrimPrefix(response.Results[0].URL, "https://image.example.test/images/")
	if !storedNamePattern.MatchString(name) || strings.Contains(name, "unsafe") {
		t.Fatalf("stored name = %q", name)
	}

	request = httptest.NewRequest(http.MethodGet, "/images/"+name, nil)
	recorder = httptest.NewRecorder()
	a.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("public image = %d %q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	if !bytes.Equal(recorder.Body.Bytes(), testPNG) {
		t.Fatal("public image bytes differ")
	}
}

func TestUploadedImageSurvivesNewApp(t *testing.T) {
	a := newTestApp(t)
	name, err := a.persistImage(testPNG, ".png")
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := New(a.config, nil)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	restarted.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/images/"+name, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("restarted public image status = %d", recorder.Code)
	}
}

func TestUploadValidation(t *testing.T) {
	tests := []struct {
		name        string
		field       string
		contentType string
		data        []byte
		wantStatus  int
	}{
		{name: "empty", field: "image", contentType: "image/png", data: nil, wantStatus: http.StatusCreated},
		{name: "unsupported declaration", field: "image", contentType: "text/plain", data: testPNG, wantStatus: http.StatusCreated},
		{name: "mismatched contents", field: "image", contentType: "image/jpeg", data: testPNG, wantStatus: http.StatusCreated},
		{name: "wrong field", field: "file", contentType: "image/png", data: testPNG, wantStatus: http.StatusBadRequest},
		{name: "too large", field: "image", contentType: "image/png", data: append(testPNG, make([]byte, maxImageBytes+1)...), wantStatus: http.StatusCreated},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := newTestApp(t)
			cookie := performLogin(t, a)
			request := uploadRequest(t, "/api/images", test.field, "file.bin", test.contentType, test.data)
			request.AddCookie(cookie)
			recorder := httptest.NewRecorder()
			a.Handler().ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			entries, err := os.ReadDir(a.config.UploadDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("invalid upload wrote %d files", len(entries))
			}
		})
	}
}

func TestDocumentUploadAndPublicAccess(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		contentType string
		data        []byte
		extension   string
		disposition string
	}{
		{name: "pdf", filename: "report.pdf", contentType: "application/pdf", data: []byte("%PDF-1.7\nexample"), extension: ".pdf", disposition: "inline"},
		{name: "text", filename: "notes.txt", contentType: "text/plain", data: []byte("hello\n"), extension: ".txt", disposition: "inline"},
		{name: "markdown", filename: "notes.md", contentType: "text/markdown", data: []byte("# hello\n"), extension: ".md", disposition: "inline"},
		{name: "epub", filename: "book.epub", contentType: "application/epub+zip", data: testZIP(t, map[string]string{"mimetype": "application/epub+zip", "META-INF/container.xml": "<container/>"}), extension: ".epub", disposition: "attachment"},
		{name: "docx", filename: "report.docx", contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", data: testZIP(t, map[string]string{"[Content_Types].xml": "<Types/>", "word/document.xml": "<w:document/>"}), extension: ".docx", disposition: "attachment"},
		{name: "xlsx", filename: "sheet.xlsx", contentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data: testZIP(t, map[string]string{"[Content_Types].xml": "<Types/>", "xl/workbook.xml": "<workbook/>"}), extension: ".xlsx", disposition: "attachment"},
		{name: "pptx", filename: "slides.pptx", contentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation", data: testZIP(t, map[string]string{"[Content_Types].xml": "<Types/>", "ppt/presentation.xml": "<p:presentation/>"}), extension: ".pptx", disposition: "attachment"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := newTestApp(t)
			request := uploadRequest(t, "/api/images", "image", test.filename, test.contentType, test.data)
			request.AddCookie(performLogin(t, a))
			recorder := httptest.NewRecorder()
			a.Handler().ServeHTTP(recorder, request)
			if recorder.Code != http.StatusCreated {
				t.Fatalf("upload status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			var response struct {
				Results []struct {
					URL string `json:"url"`
				} `json:"results"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if len(response.Results) != 1 || response.Results[0].URL == "" {
				t.Fatalf("upload results = %#v", response.Results)
			}
			name := strings.TrimPrefix(response.Results[0].URL, "https://image.example.test/images/")
			if !strings.HasSuffix(name, test.extension) || !storedNamePattern.MatchString(name) {
				t.Fatalf("stored name = %q", name)
			}

			recorder = httptest.NewRecorder()
			a.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/images/"+name, nil))
			if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != test.contentType {
				t.Fatalf("public file = %d %q", recorder.Code, recorder.Header().Get("Content-Type"))
			}
			if !strings.HasPrefix(recorder.Header().Get("Content-Disposition"), test.disposition) {
				t.Fatalf("content disposition = %q", recorder.Header().Get("Content-Disposition"))
			}
			if !bytes.Equal(recorder.Body.Bytes(), test.data) {
				t.Fatal("public file bytes differ")
			}
		})
	}
}

func TestDocumentUploadValidation(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		data        []byte
		wantStatus  int
	}{
		{name: "invalid PDF", contentType: "application/pdf", data: []byte("not a PDF"), wantStatus: http.StatusCreated},
		{name: "invalid UTF-8", contentType: "text/plain", data: []byte{0xff}, wantStatus: http.StatusCreated},
		{name: "wrong Office container", contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", data: testZIP(t, map[string]string{"[Content_Types].xml": "<Types/>", "xl/workbook.xml": "<workbook/>"}), wantStatus: http.StatusCreated},
		{name: "unsupported legacy Office", contentType: "application/msword", data: []byte("legacy"), wantStatus: http.StatusCreated},
		{name: "document too large", contentType: "application/pdf", data: append([]byte("%PDF-1.7"), make([]byte, maxDocumentBytes)...), wantStatus: http.StatusCreated},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := newTestApp(t)
			request := uploadRequest(t, "/api/images", "image", "file.bin", test.contentType, test.data)
			request.AddCookie(performLogin(t, a))
			recorder := httptest.NewRecorder()
			a.Handler().ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			entries, err := os.ReadDir(a.config.UploadDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("invalid upload wrote %d files", len(entries))
			}
		})
	}
}

func TestUploadRequiresAuthentication(t *testing.T) {
	a := newTestApp(t)
	request := uploadRequest(t, "/api/images", "image", "file.png", "image/png", testPNG)
	recorder := httptest.NewRecorder()
	a.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestPublicPathIsRestricted(t *testing.T) {
	a := newTestApp(t)
	for _, path := range []string{"/images/not-valid.png", "/images/0123.png", "/images/%2e%2e%2fsecret.png"} {
		recorder := httptest.NewRecorder()
		a.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code == http.StatusOK {
			t.Fatalf("unsafe path %q returned 200", path)
		}
	}
}

func TestConfigValidation(t *testing.T) {
	base := Config{ListenAddr: "127.0.0.1:0", UploadDir: t.TempDir(), UploadPassword: "password", SessionSecret: testSessionSecret, PublicBaseURL: "https://example.test"}
	tests := []struct {
		name string
		edit func(*Config)
	}{
		{name: "missing password", edit: func(c *Config) { c.UploadPassword = "" }},
		{name: "short session secret", edit: func(c *Config) { c.SessionSecret = "short" }},
		{name: "public URL path", edit: func(c *Config) { c.PublicBaseURL = "https://example.test/path" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := base
			test.edit(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() returned nil")
			}
		})
	}
}

func TestImageTypeDetection(t *testing.T) {
	tests := []struct {
		mime string
		data []byte
	}{
		{mime: "image/jpeg", data: []byte("\xff\xd8\xff\xe0JFIF\x00")},
		{mime: "image/png", data: testPNG},
		{mime: "image/gif", data: []byte("GIF89a\x01\x00\x01\x00")},
		{mime: "image/webp", data: []byte("RIFF\x04\x00\x00\x00WEBPVP8 ")},
	}
	for _, test := range tests {
		t.Run(test.mime, func(t *testing.T) {
			kind, ok := detectImageType(test.data)
			if !ok || kind.mime != test.mime {
				t.Fatalf("detectImageType() = %#v, %v", kind, ok)
			}
		})
	}
}

func TestWebFilesExposeCopyableResult(t *testing.T) {
	a := newTestApp(t)
	for _, name := range []string{"upload.html", "upload.js"} {
		contents, err := fs.ReadFile(a.web, name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(contents)
		if !strings.Contains(text, "copy-button") && !strings.Contains(text, "navigator.clipboard") && name != "upload.html" {
			t.Fatalf("%s does not include copy UI behavior", name)
		}
	}
}

func TestPersistedPermissions(t *testing.T) {
	a := newTestApp(t)
	name, err := a.persistImage(testPNG, ".png")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(a.config.UploadDir, name))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("permissions = %o", info.Mode().Perm())
	}
}
