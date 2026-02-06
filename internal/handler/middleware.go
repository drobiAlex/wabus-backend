package handler

import (
	"net/http"

	"github.com/klauspost/compress/gzhttp"
)

func GzipMiddleware(next http.Handler) http.Handler {
	wrapper, _ := gzhttp.NewWrapper(
		gzhttp.MinSize(1024),
		gzhttp.CompressionLevel(6),
	)
	return wrapper(next)
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, If-None-Match")
		w.Header().Set("Access-Control-Expose-Headers", "ETag")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
