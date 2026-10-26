package app

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	defaultListenAddr    = "127.0.0.1:8080"
	defaultUploadDir     = "./uploads"
	defaultPublicBaseURL = "http://localhost:8080"
)

// Config contains the runtime settings required by the HTTP application.
type Config struct {
	ListenAddr     string
	UploadDir      string
	UploadPassword string
	SessionSecret  string
	PublicBaseURL  string
	CookieSecure   bool
	UploadTokens   []string
}

// LoadConfig reads configuration from environment variables.
func LoadConfig() (Config, error) {
	secure, err := envBool("COOKIE_SECURE", true)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		ListenAddr:     envOrDefault("LISTEN_ADDR", defaultListenAddr),
		UploadDir:      envOrDefault("UPLOAD_DIR", defaultUploadDir),
		UploadPassword: os.Getenv("UPLOAD_PASSWORD"),
		SessionSecret:  os.Getenv("SESSION_SECRET"),
		PublicBaseURL:  envOrDefault("PUBLIC_BASE_URL", defaultPublicBaseURL),
		CookieSecure:   secure,
	}
	if raw := strings.TrimSpace(os.Getenv("NEWAPI_MEDIA_UPLOAD_TOKENS")); raw != "" {
		for _, token := range strings.Split(raw, ",") {
			cfg.UploadTokens = append(cfg.UploadTokens, strings.TrimSpace(token))
		}
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate rejects missing secrets and malformed public URLs before startup.
func (c Config) Validate() error {
	if strings.TrimSpace(c.ListenAddr) == "" {
		return errors.New("LISTEN_ADDR 不能为空")
	}
	if strings.TrimSpace(c.UploadDir) == "" {
		return errors.New("UPLOAD_DIR 不能为空")
	}
	if c.UploadPassword == "" {
		return errors.New("缺少必填环境变量 UPLOAD_PASSWORD")
	}
	if len(c.SessionSecret) < 32 {
		return errors.New("SESSION_SECRET 至少需要 32 个字符")
	}
	for _, token := range c.UploadTokens {
		if len(token) < 32 || !validBearerToken(token) {
			return errors.New("NEWAPI_MEDIA_UPLOAD_TOKENS 每个令牌至少需要 32 个字符，且必须使用合法的 Bearer 令牌字符，不能包含空项")
		}
	}

	u, err := url.Parse(c.PublicBaseURL)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("PUBLIC_BASE_URL 必须是完整的 HTTP 或 HTTPS 地址")
	}
	return nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) (bool, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, errors.New(name + " 必须是 true 或 false")
	}
	return parsed, nil
}
