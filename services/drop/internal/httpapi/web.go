package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:web
var webFiles embed.FS

func webHandler() http.Handler {
	root, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/v1/") || request.URL.Path == "/health/live" || request.URL.Path == "/health/ready" {
			http.NotFound(response, request)
			return
		}
		response.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(response, request)
	})
}
