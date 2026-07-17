package httpapi

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"net/http"
	"strconv"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"
)

const (
	previewMaxDimension = 1280
	previewMaxPixels    = int64(50_000_000)
	previewJPEGQuality  = 80
)

func (a *API) getAttachmentPreview(w http.ResponseWriter, r *http.Request) {
	file, attachment, err := a.store.OpenAttachment(r.Context(), r.PathValue("id"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer func() { _ = file.Close() }()
	if !isThumbnailableImage(attachment.MIMEType) {
		writeAPIError(w, &apiError{Status: http.StatusUnsupportedMediaType, Code: "preview_unavailable", Message: "A compact preview is unavailable for this image"})
		return
	}

	etag := fmt.Sprintf("%q", "preview-v1-"+attachment.ID+"-"+strconv.FormatInt(attachment.Size, 10))
	if r.Header.Get("If-None-Match") == etag {
		setPreviewHeaders(w, attachment.OriginalName, etag)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	config, _, err := image.DecodeConfig(file)
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > previewMaxPixels {
		writeAPIError(w, &apiError{Status: http.StatusUnprocessableEntity, Code: "preview_invalid", Message: "The image cannot be safely previewed"})
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		a.logInternal("seek attachment preview", err)
		writeAPIError(w, internalError())
		return
	}
	decoded, err := imaging.Decode(file, imaging.AutoOrientation(true))
	if err != nil {
		writeAPIError(w, &apiError{Status: http.StatusUnprocessableEntity, Code: "preview_invalid", Message: "The image cannot be decoded"})
		return
	}
	preview := imaging.Fit(decoded, previewMaxDimension, previewMaxDimension, imaging.Lanczos)
	var content bytes.Buffer
	if err := imaging.Encode(&content, preview, imaging.JPEG, imaging.JPEGQuality(previewJPEGQuality)); err != nil {
		a.logInternal("encode attachment preview", err)
		writeAPIError(w, internalError())
		return
	}
	setPreviewHeaders(w, attachment.OriginalName, etag)
	http.ServeContent(w, r, attachment.OriginalName+".preview.jpg", attachment.CreatedAt, bytes.NewReader(content.Bytes()))
}

func setPreviewHeaders(w http.ResponseWriter, name, etag string) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Disposition", contentDisposition("inline", name+".preview.jpg"))
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("ETag", etag)
	w.Header().Set("Vary", "Cookie, Authorization, Tailscale-User-Login")
}

func isThumbnailableImage(value string) bool {
	switch safeContentType(value) {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}
