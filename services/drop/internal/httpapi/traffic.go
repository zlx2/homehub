package httpapi

import (
	"io"
	"net/http"
	"strings"
	"time"
)

type trafficResponseWriter struct {
	http.ResponseWriter
	bytes  int64
	status int
}

func (w *trafficResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *trafficResponseWriter) Write(content []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	written, err := w.ResponseWriter.Write(content)
	w.bytes += int64(written)
	return written, err
}

// ReadFrom preserves the optimized file-to-socket path exposed by net/http's
// ResponseWriter. Without it, io.CopyN (used by http.ServeContent) falls back
// to copying through a small userspace buffer whenever traffic metering wraps
// the response.
func (w *trafficResponseWriter) ReadFrom(src io.Reader) (int64, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}

	var (
		written int64
		err     error
	)
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		written, err = readerFrom.ReadFrom(src)
	} else {
		written, err = io.Copy(w.ResponseWriter, src)
	}
	w.bytes += written
	return written, err
}

func (w *trafficResponseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *trafficResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (a *API) measureTraffic(entry EntryPoint, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/health/") {
			next.ServeHTTP(w, r)
			return
		}
		meter := &trafficResponseWriter{ResponseWriter: w}
		next.ServeHTTP(meter, r)
		if err := a.store.RecordTraffic(r.Context(), time.Now(), string(entry), trafficCategory(r.URL.Path), meter.bytes); err != nil {
			a.logger.Warn("record response traffic", "error", err)
		}
	})
}

func trafficCategory(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/v1/attachments/") && strings.HasSuffix(path, "/preview"):
		return "preview"
	case strings.HasPrefix(path, "/api/v1/attachments/"):
		return "attachment"
	case strings.HasPrefix(path, "/assets/"):
		return "asset"
	case path == "/api/v1/events":
		return "event"
	case strings.HasPrefix(path, "/api/"):
		return "api"
	default:
		return "page"
	}
}
