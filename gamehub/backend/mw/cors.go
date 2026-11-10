package mw

import (
	"net/http"
	"strings"
)

// CORS อนุญาตเฉพาะ origin ที่อยู่ในรายการ (ALLOWED_ORIGINS) ไม่ใช้ "*"
// origin ที่ไม่อยู่ในรายการจะไม่ได้ header ใด ๆ เบราว์เซอร์จึงบล็อกเอง
func CORS(allowed []string, next http.Handler) http.Handler {
	origins := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins[origin] = struct{}{}
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// ผลลัพธ์ขึ้นกับ Origin จึงต้องบอก cache ทุกครั้ง ไม่ว่าจะอนุญาตหรือไม่
		w.Header().Add("Vary", "Origin")
		if _, ok := origins[origin]; ok {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
