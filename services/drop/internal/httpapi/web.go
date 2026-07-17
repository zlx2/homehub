package httpapi

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

//go:embed web/*
var webFiles embed.FS

var pageTemplates = template.Must(template.ParseFS(webFiles, "web/*.html"))

type embeddedAsset struct {
	content, gzipContent []byte
	contentType          string
	etag, gzipETag       string
}

type embeddedBundle struct {
	assets  map[string]embeddedAsset
	version string
}

var embeddedWeb = func() embeddedBundle {
	assets := make(map[string]embeddedAsset, 3)
	bundleHash := sha256.New()
	for _, item := range []struct {
		name, contentType string
		compress          bool
	}{
		{"app.css", "text/css; charset=utf-8", true},
		{"app.js", "text/javascript; charset=utf-8", true},
		{"favicon.ico", "image/x-icon", false},
	} {
		name, contentType := item.name, item.contentType
		content, err := fs.ReadFile(webFiles, "web/"+name)
		if err != nil {
			panic(fmt.Sprintf("load embedded asset %s: %v", name, err))
		}
		var compressed []byte
		if item.compress {
			var buffer bytes.Buffer
			writer, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
			if err != nil {
				panic(fmt.Sprintf("create gzip writer for %s: %v", name, err))
			}
			if _, err := writer.Write(content); err != nil {
				panic(fmt.Sprintf("compress embedded asset %s: %v", name, err))
			}
			if err := writer.Close(); err != nil {
				panic(fmt.Sprintf("finish embedded asset compression %s: %v", name, err))
			}
			compressed = buffer.Bytes()
		}
		hash := sha256.Sum256(content)
		assets[name] = embeddedAsset{
			content: content, gzipContent: compressed, contentType: contentType,
			etag: fmt.Sprintf("\"%x\"", hash), gzipETag: fmt.Sprintf("\"%x-gzip\"", hash),
		}
		_, _ = bundleHash.Write([]byte(name))
		_, _ = bundleHash.Write(content)
	}
	return embeddedBundle{assets: assets, version: hex.EncodeToString(bundleHash.Sum(nil))[:12]}
}()

type pageData struct {
	Page         string
	Role         Role
	AssetVersion string
	BasePath     string
}

func (a *API) publicPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	cookie, err := r.Cookie(a.cfg.CookieName)
	if err == nil {
		_, valid, validateErr := a.auth.ValidateSession(r.Context(), cookie.Value, clientIP(r, a.cfg.TrustedPublicProxies))
		if validateErr != nil {
			a.logInternal("validate page session", validateErr)
			writeAPIError(w, internalError())
			return
		}
		if valid {
			a.renderPage(w, "app.html", pageData{Page: "app", Role: RoleGuest})
			return
		}
	}
	a.renderPage(w, "auth.html", pageData{Page: "auth"})
}

func (a *API) appPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	a.renderPage(w, "app.html", pageData{Page: "app", Role: principalFrom(r).Role})
}

func (a *API) renderPage(w http.ResponseWriter, name string, data pageData) {
	data.AssetVersion = embeddedWeb.version
	data.BasePath = a.cfg.BasePath
	var output bytes.Buffer
	if err := pageTemplates.ExecuteTemplate(&output, name, data); err != nil {
		a.logInternal("render page", err)
		writeAPIError(w, internalError())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = output.WriteTo(w)
}

func (a *API) asset(w http.ResponseWriter, r *http.Request) {
	name := path.Base(r.URL.Path)
	asset, ok := embeddedWeb.assets[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	content, etag := asset.content, asset.etag
	if len(asset.gzipContent) > 0 && acceptsGzip(r.Header.Get("Accept-Encoding")) {
		content, etag = asset.gzipContent, asset.gzipETag
		w.Header().Set("Content-Encoding", "gzip")
	}
	w.Header().Set("Content-Type", asset.contentType)
	if len(asset.gzipContent) > 0 {
		w.Header().Set("Vary", "Accept-Encoding")
	}
	if r.URL.Query().Get("v") == embeddedWeb.version {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	}
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(content))
}

func acceptsGzip(header string) bool {
	wildcardQuality := float64(-1)
	for _, value := range strings.Split(header, ",") {
		parts := strings.Split(value, ";")
		encoding := strings.ToLower(strings.TrimSpace(parts[0]))
		quality := 1.0
		for _, parameter := range parts[1:] {
			key, raw, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if !found || !strings.EqualFold(key, "q") {
				continue
			}
			parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
			if err != nil {
				quality = 0
			} else {
				quality = parsed
			}
		}
		if encoding == "gzip" {
			return quality > 0
		}
		if encoding == "*" {
			wildcardQuality = quality
		}
	}
	return wildcardQuality > 0
}
