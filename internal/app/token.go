package app

import (
	"net/http"
	"strings"
)

// validBearerToken follows the token character grammar in RFC 6750.
func validBearerToken(token string) bool {
	if token == "" {
		return false
	}
	padding := false
	for _, char := range token {
		if char == '=' {
			padding = true
			continue
		}
		if padding || !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("-._~+/", char)) {
			return false
		}
	}
	return token[0] != '='
}

func (a *App) requireUploadToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		values := r.Header.Values("Authorization")
		if len(values) == 1 {
			fields := strings.Fields(values[0])
			if len(fields) == 2 && strings.EqualFold(fields[0], "Bearer") {
				matched := false
				for _, token := range a.config.UploadTokens {
					// Compare every configured token so the match position is not exposed.
					if passwordMatches(fields[1], token) {
						matched = true
					}
				}
				if matched {
					next(w, r)
					return
				}
			}
		}
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, http.StatusUnauthorized, "上传令牌缺失或无效")
	}
}
