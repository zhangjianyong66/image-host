package app

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	webassets "image-host/web"
)

const maxLoginBodyBytes = 4 << 10

// App owns the HTTP routes and the local image store.
type App struct {
	config Config
	logger *slog.Logger
	now    func() time.Time
	web    fs.FS
}

// New validates configuration and prepares the persistent upload directory.
func New(config Config, logger *slog.Logger) (*App, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(config.UploadDir, 0o750); err != nil {
		return nil, errors.New("无法创建上传目录: " + err.Error())
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &App{
		config: config,
		logger: logger,
		now:    time.Now,
		web:    webassets.Files,
	}, nil
}

// Handler returns the complete application HTTP handler.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.handleRoot)
	mux.HandleFunc("GET /login", a.handleLoginPage)
	mux.HandleFunc("GET /upload", a.handleUploadPage)
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(a.web)))
	mux.HandleFunc("POST /api/login", a.handleLogin)
	mux.HandleFunc("POST /api/logout", a.handleLogout)
	mux.HandleFunc("GET /api/session", a.handleSession)
	mux.HandleFunc("POST /api/images", a.requireAPIAuth(a.handleUpload))
	mux.HandleFunc("GET /images/{name}", a.handleImage)
	mux.HandleFunc("GET /healthz", a.handleHealth)
	return a.securityHeaders(mux)
}

func (a *App) handleRoot(w http.ResponseWriter, r *http.Request) {
	if a.authenticated(r) {
		http.Redirect(w, r, "/upload", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if a.authenticated(r) {
		http.Redirect(w, r, "/upload", http.StatusSeeOther)
		return
	}
	a.serveWebFile(w, r, "login.html")
}

func (a *App) handleUploadPage(w http.ResponseWriter, r *http.Request) {
	if !a.authenticated(r) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	a.serveWebFile(w, r, "upload.html")
}

func (a *App) serveWebFile(w http.ResponseWriter, r *http.Request, name string) {
	contents, err := fs.ReadFile(a.web, name)
	if err != nil {
		http.Error(w, "页面暂时不可用", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(contents)
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input struct {
		Password string `json:"password"`
	}
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "登录请求格式不正确")
		return
	}
	if err := ensureJSONEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "登录请求格式不正确")
		return
	}
	if !passwordMatches(input.Password, a.config.UploadPassword) {
		writeError(w, http.StatusUnauthorized, "密码错误")
		return
	}
	if err := a.setSessionCookie(w, a.now()); err != nil {
		a.logger.Error("create session cookie", "error", err)
		writeError(w, http.StatusInternalServerError, "暂时无法登录，请稍后重试")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing JSON")
	}
	return nil
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	a.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": false})
}

func (a *App) handleSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": a.authenticated(r)})
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) requireAPIAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.authenticated(r) {
			writeError(w, http.StatusUnauthorized, "请先登录")
			return
		}
		next(w, r)
	}
}

func (a *App) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob: data:; style-src 'self'; script-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func cleanBaseURL(raw string) string {
	return strings.TrimRight(raw, "/")
}
