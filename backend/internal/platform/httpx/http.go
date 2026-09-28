package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

func JSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}

func Decode(w http.ResponseWriter, r *http.Request, limit int64, target any) bool {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType != "application/json" {
		Error(w, 415, "请使用 JSON 格式提交")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(target)
	if err == nil {
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			err = errors.New("unexpected data")
		}
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			Error(w, 413, "提交内容过大")
		} else {
			Error(w, 400, "提交内容格式不正确")
		}
		return false
	}
	return true
}

// Cookie authentication requires the browser's writes to originate on this site.
// Reverse proxies must preserve Host; forwarded headers are not blindly trusted.
func SameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				Error(w, 403, "不允许跨站操作")
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				parsed, err := url.Parse(origin)
				if err != nil || !(parsed.Scheme == "http" || parsed.Scheme == "https") || !strings.EqualFold(parsed.Host, r.Host) {
					Error(w, 403, "不允许跨站操作")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
