package httpd

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type SigConfig struct {
	Secret        string
	MaxAgeSeconds int64
}

func SignatureMiddleware(cfg SigConfig) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				ts := r.Header.Get("X-Timestamp")
				sig := r.Header.Get("X-Signature")

				if ts == "" || sig == "" {
					http.Error(w, "missing signature headers", http.StatusUnauthorized)
					return
				}

				tsInt, err := strconv.ParseInt(ts, 10, 64)
				if err != nil {
					http.Error(w, "invalid timestamp", http.StatusUnauthorized)
					return
				}

				now := time.Now().Unix()
				if cfg.MaxAgeSeconds > 0 && (now-tsInt) > cfg.MaxAgeSeconds {
					http.Error(w, "signature expired", http.StatusUnauthorized)
					return
				}

				bodyBytes, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "read body error", http.StatusBadRequest)
					return
				}

				r.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

				mac := hmac.New(sha256.New, []byte(cfg.Secret))
				mac.Write(append(bodyBytes, []byte("."+ts)...))
				expected := hex.EncodeToString(mac.Sum(nil))
				if !hmac.Equal([]byte(expected), []byte(sig)) {
					http.Error(w, "invalid signature", http.StatusUnauthorized)
					return
				}
			}
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
