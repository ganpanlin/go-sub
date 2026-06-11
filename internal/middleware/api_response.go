package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

type responseEnvelope struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	return r.body.Write(data)
}

// ApiResponseMiddleware wraps API responses as {code,msg,data}.
func ApiResponseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, req)

		body := rec.body.Bytes()
		isError := rec.status >= http.StatusBadRequest

		msg := "ok"
		var data interface{}

		if isError {
			msg = extractErrorMessage(body)
			if msg == "" {
				msg = http.StatusText(rec.status)
			}
			data = nil
			slog.Info("api error response", "method", req.Method, "path", req.URL.Path, "status", rec.status, "msg", msg, "body_bytes", len(body))
		} else if rec.status == http.StatusNoContent || len(bytes.TrimSpace(body)) == 0 {
			data = nil
		} else {
			contentType := rec.Header().Get("Content-Type")
			if len(bytes.TrimSpace(body)) > 0 {
				if strings.Contains(contentType, "application/json") {
					if err := json.Unmarshal(body, &data); err != nil {
						data = string(body)
						slog.Error("api json parse error", "method", req.Method, "path", req.URL.Path, "error", err)
					}
				} else {
					data = string(body)
				}
			}
		}

		code := 0
		if isError {
			code = rec.status
		}

		// Set headers and write response (no gzip to avoid header ordering issues)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(responseEnvelope{Code: code, Msg: msg, Data: data})
	})
}

func extractErrorMessage(body []byte) string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return ""
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err == nil {
		for _, key := range []string{"msg", "message", "error"} {
			if value, ok := obj[key].(string); ok {
				return value
			}
		}
	}

	return strings.TrimSpace(string(body))
}
