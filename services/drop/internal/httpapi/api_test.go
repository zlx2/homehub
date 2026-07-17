package httpapi

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"drop/internal/auth"
	"drop/internal/config"
	"drop/internal/store"
)

type testEnvironment struct {
	api    *API
	store  *store.Store
	cfg    config.Config
	hub    *Hub
	public http.Handler
	owner  http.Handler
	hermes http.Handler
}

func TestAuthenticationAndRoleBoundaries(t *testing.T) {
	env := newTestEnvironment(t)

	unauthorized := perform(env.public, http.MethodGet, "/api/v1/items", nil, nil, "203.0.113.5:1000")
	assertErrorCode(t, unauthorized, http.StatusUnauthorized, "unauthorized")
	if strings.Contains(unauthorized.Body.String(), "items") {
		t.Fatalf("unauthorized response leaked item information: %s", unauthorized.Body.String())
	}

	forgedOwner := perform(env.owner, http.MethodGet, "/api/v1/status", nil,
		map[string]string{"Tailscale-User-Login": "owner@example.com"}, "203.0.113.5:1000")
	assertErrorCode(t, forgedOwner, http.StatusUnauthorized, "unauthorized")
	env.api.cfg.AllowNonLoopback = true
	containerOwner := perform(env.owner, http.MethodGet, "/api/v1/status", nil,
		map[string]string{"Tailscale-User-Login": "owner@example.com"}, "172.20.0.1:1000")
	if containerOwner.Code != http.StatusOK {
		t.Fatalf("explicit container owner status = %d: %s", containerOwner.Code, containerOwner.Body.String())
	}
	containerHermes := perform(env.hermes, http.MethodGet, "/api/v1/status", nil,
		map[string]string{"Authorization": "Bearer " + env.cfg.HermesToken}, "172.20.0.1:1000")
	if containerHermes.Code != http.StatusOK {
		t.Fatalf("explicit container Hermes status = %d: %s", containerHermes.Code, containerHermes.Body.String())
	}
	env.api.cfg.AllowNonLoopback = false

	codeResponse := ownerRequest(env, http.MethodPost, "/api/v1/auth/codes", nil)
	if codeResponse.Code != http.StatusCreated {
		t.Fatalf("create code status = %d: %s", codeResponse.Code, codeResponse.Body.String())
	}
	var generated struct {
		Code      string `json:"code"`
		RedeemURL string `json:"redeem_url"`
		QRDataURL string `json:"qr_data_url"`
	}
	decodeResponse(t, codeResponse, &generated)
	if generated.Code == "" || generated.RedeemURL != "https://drop.example.test/#code="+generated.Code {
		t.Fatalf("generated authorization response = %#v", generated)
	}
	qrPNG, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(generated.QRDataURL, "data:image/png;base64,"))
	if err != nil || !bytes.HasPrefix(qrPNG, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("generated QR image is invalid: %v", err)
	}

	redeemBody := strings.NewReader(`{"code":` + quoteJSON(generated.Code) + `}`)
	redeemed := perform(env.public, http.MethodPost, "/api/v1/auth/redeem", redeemBody,
		map[string]string{
			"Content-Type": "application/json",
			"User-Agent":   "Mozilla/5.0 (iPhone) AppleWebKit/605.1.15 Via/5.8 Safari/604.1",
		}, "203.0.113.5:1000")
	if redeemed.Code != http.StatusOK {
		t.Fatalf("redeem status = %d: %s", redeemed.Code, redeemed.Body.String())
	}
	cookies := redeemed.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].MaxAge != 12*60*60 {
		t.Fatalf("session cookie = %#v", cookies)
	}

	reused := perform(env.public, http.MethodPost, "/api/v1/auth/redeem",
		strings.NewReader(`{"code":`+quoteJSON(generated.Code)+`}`), map[string]string{"Content-Type": "application/json"}, "203.0.113.5:1001")
	assertErrorCode(t, reused, http.StatusUnauthorized, "authorization_code_invalid")

	guestHeaders := map[string]string{"Cookie": cookies[0].Name + "=" + cookies[0].Value}
	guestStatus := perform(env.public, http.MethodGet, "/api/v1/status", nil, guestHeaders, "203.0.113.5:1000")
	assertErrorCode(t, guestStatus, http.StatusForbidden, "forbidden")
	guestCode := perform(env.public, http.MethodPost, "/api/v1/auth/codes", nil, guestHeaders, "203.0.113.5:1000")
	assertErrorCode(t, guestCode, http.StatusForbidden, "forbidden")

	deviceList := ownerRequest(env, http.MethodGet, "/api/v1/auth/sessions", nil)
	if deviceList.Code != http.StatusOK {
		t.Fatalf("device list = %d: %s", deviceList.Code, deviceList.Body.String())
	}
	var listed struct {
		Sessions []trustedSessionResponse `json:"sessions"`
	}
	decodeResponse(t, deviceList, &listed)
	if len(listed.Sessions) != 1 || listed.Sessions[0].DeviceName != "Via · iPhone" || listed.Sessions[0].Current {
		t.Fatalf("trusted devices = %#v", listed.Sessions)
	}
	guestDeviceList := perform(env.public, http.MethodGet, "/api/v1/auth/sessions", nil, guestHeaders, "203.0.113.5:1000")
	var guestListed struct {
		Sessions []trustedSessionResponse `json:"sessions"`
	}
	decodeResponse(t, guestDeviceList, &guestListed)
	if guestDeviceList.Code != http.StatusOK || len(guestListed.Sessions) != 1 || !guestListed.Sessions[0].Current {
		t.Fatalf("guest trusted devices = %d %#v", guestDeviceList.Code, guestListed.Sessions)
	}
	revoked := ownerRequest(env, http.MethodDelete, "/api/v1/auth/sessions/"+strconv.FormatInt(listed.Sessions[0].ID, 10), nil)
	if revoked.Code != http.StatusNoContent {
		t.Fatalf("revoke device = %d: %s", revoked.Code, revoked.Body.String())
	}
	revokedGuest := perform(env.public, http.MethodGet, "/api/v1/items", nil, guestHeaders, "203.0.113.5:1000")
	assertErrorCode(t, revokedGuest, http.StatusUnauthorized, "unauthorized")

	hermesStatus := perform(env.hermes, http.MethodGet, "/api/v1/status", nil,
		map[string]string{"Authorization": "Bearer " + env.cfg.HermesToken}, "127.0.0.1:1000")
	if hermesStatus.Code != http.StatusOK {
		t.Fatalf("Hermes status = %d: %s", hermesStatus.Code, hermesStatus.Body.String())
	}
}

func TestDescribeDevice(t *testing.T) {
	for _, test := range []struct{ userAgent, want string }{
		{"Mozilla/5.0 (iPhone) Via/5.8 Safari/604.1", "Via · iPhone"},
		{"Mozilla/5.0 (iPad) CriOS/125.0 Mobile Safari/604.1", "Chrome · iPad"},
		{"", "浏览器 · 设备"},
	} {
		if got := describeDevice(test.userAgent); got != test.want {
			t.Errorf("describeDevice(%q) = %q, want %q", test.userAgent, got, test.want)
		}
	}
}

func TestWebPagesFollowMinimalComposerContract(t *testing.T) {
	env := newTestEnvironment(t)
	authPage := perform(env.public, http.MethodGet, "/", nil, nil, "203.0.113.5:1000")
	if authPage.Code != http.StatusOK || !strings.Contains(authPage.Body.String(), `data-page="auth"`) {
		t.Fatalf("public auth page = %d: %s", authPage.Code, authPage.Body.String())
	}
	if strings.Contains(strings.ToLower(authPage.Body.String()), "<nav") {
		t.Fatal("auth page unexpectedly contains navigation")
	}

	appPage := ownerRequest(env, http.MethodGet, "/", nil)
	content := appPage.Body.String()
	for _, required := range []string{`id="app"`, `data-page="app"`, `data-role="owner"`, `type="module"`, `/assets/app.js`, `/favicon.ico`} {
		if appPage.Code != http.StatusOK || !strings.Contains(content, required) {
			t.Fatalf("owner page missing %q: %d %s", required, appPage.Code, content)
		}
	}
	if strings.Contains(strings.ToLower(content), "<nav") {
		t.Fatal("owner page unexpectedly contains navigation")
	}
	if csp := appPage.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") {
		t.Fatalf("CSP = %q", csp)
	}

	asset := perform(env.public, http.MethodGet, "/assets/app.js", nil, nil, "203.0.113.5:1000")
	if asset.Code != http.StatusOK || !strings.Contains(asset.Header().Get("Content-Type"), "javascript") ||
		!strings.Contains(asset.Header().Get("Cache-Control"), "must-revalidate") || asset.Header().Get("ETag") == "" {
		t.Fatalf("JS asset = %d %#v", asset.Code, asset.Header())
	}
	cachedAsset := perform(env.public, http.MethodGet, "/assets/app.js", nil,
		map[string]string{"If-None-Match": asset.Header().Get("ETag")}, "203.0.113.5:1000")
	if cachedAsset.Code != http.StatusNotModified || cachedAsset.Body.Len() != 0 {
		t.Fatalf("cached JS asset = %d %q", cachedAsset.Code, cachedAsset.Body.String())
	}
	versionedAsset := perform(env.public, http.MethodGet, "/assets/app.js?v="+embeddedWeb.version, nil,
		map[string]string{"Accept-Encoding": "gzip"}, "203.0.113.5:1000")
	if versionedAsset.Code != http.StatusOK || versionedAsset.Header().Get("Content-Encoding") != "gzip" ||
		!strings.Contains(versionedAsset.Header().Get("Cache-Control"), "immutable") ||
		versionedAsset.Body.Len() >= asset.Body.Len() {
		t.Fatalf("versioned compressed asset = %d %#v (%d bytes)", versionedAsset.Code, versionedAsset.Header(), versionedAsset.Body.Len())
	}
	reader, err := gzip.NewReader(versionedAsset.Body)
	if err != nil {
		t.Fatalf("open compressed JS asset: %v", err)
	}
	decompressed, err := io.ReadAll(reader)
	if closeErr := reader.Close(); err == nil {
		err = closeErr
	}
	if err != nil || !bytes.Equal(decompressed, asset.Body.Bytes()) {
		t.Fatalf("compressed JS asset round trip failed: %v", err)
	}

	favicon := perform(env.public, http.MethodGet, "/favicon.ico?v="+embeddedWeb.version, nil,
		map[string]string{"Accept-Encoding": "gzip"}, "203.0.113.5:1000")
	if favicon.Code != http.StatusOK || favicon.Header().Get("Content-Type") != "image/x-icon" ||
		favicon.Header().Get("Content-Encoding") != "" ||
		!strings.Contains(favicon.Header().Get("Cache-Control"), "immutable") || favicon.Body.Len() == 0 {
		t.Fatalf("favicon asset = %d %#v (%d bytes)", favicon.Code, favicon.Header(), favicon.Body.Len())
	}
}

func TestAcceptsGzip(t *testing.T) {
	for _, test := range []struct {
		header string
		want   bool
	}{
		{"gzip, deflate", true},
		{"br, *;q=0.5", true},
		{"gzip;q=0, *;q=1", false},
		{"br", false},
	} {
		if got := acceptsGzip(test.header); got != test.want {
			t.Errorf("acceptsGzip(%q) = %t, want %t", test.header, got, test.want)
		}
	}
}

func TestHealthChecksArePublicAndExcludedFromTraffic(t *testing.T) {
	env := newTestEnvironment(t)
	for _, target := range []string{"/health/live", "/health/ready"} {
		response := perform(env.public, http.MethodGet, target, nil, nil, "127.0.0.1:1000")
		if response.Code != http.StatusNoContent {
			t.Fatalf("%s = %d %s", target, response.Code, response.Body.String())
		}
	}
	report, err := env.store.TrafficReport(context.Background(), time.Now())
	if err != nil || report.Last30Days.Requests != 0 {
		t.Fatalf("health traffic = %#v, %v", report.Last30Days, err)
	}
}

func TestAllowedTTLStopsAtSevenDays(t *testing.T) {
	for _, days := range []int{1, 3, 7} {
		if _, ok := allowedTTL(days); !ok {
			t.Fatalf("allowedTTL(%d) was rejected", days)
		}
	}
	if _, ok := allowedTTL(30); ok {
		t.Fatal("allowedTTL(30) was accepted")
	}
}

func TestInlineItemResponseUsesLoadedText(t *testing.T) {
	item := store.Item{
		ID: "inline-item", TextInline: []byte("already loaded"), TextSize: int64(len("already loaded")),
		Source: "owner", CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	response, err := (&API{}).itemResponse(httptest.NewRequest(http.MethodGet, "/api/v1/items", nil), item)
	if err != nil || response.TextPreview != "already loaded" || response.TextTruncated {
		t.Fatalf("inline item response = %#v, %v", response, err)
	}
}

func TestCreateItemIdempotencyKeyPreventsDuplicateRetry(t *testing.T) {
	env := newTestEnvironment(t)
	create := func(text string) itemResponse {
		body, contentType := multipartBody(t, text, 1, nil)
		response := ownerRequestWithHeaders(env, http.MethodPost, "/api/v1/items", body, map[string]string{
			"Content-Type": contentType, "Idempotency-Key": "retry-key-1234567890",
		})
		if response.Code != http.StatusCreated {
			t.Fatalf("create item = %d: %s", response.Code, response.Body.String())
		}
		var item itemResponse
		decodeResponse(t, response, &item)
		return item
	}
	first := create("first body")
	second := create("body resent after response loss")
	if second.ID != first.ID || second.TextPreview != first.TextPreview {
		t.Fatalf("idempotent retry = %#v, want %#v", second, first)
	}
	usage, err := env.store.Usage(context.Background())
	if err != nil || usage.ItemCount != 1 {
		t.Fatalf("usage after idempotent retry = %#v, %v", usage, err)
	}
}

func TestMultipartCreateReadAndDelete(t *testing.T) {
	env := newTestEnvironment(t)
	env.api.cfg.MaxAttachmentBytes = 1 << 20
	env.api.cfg.MaxItemBytes = 2 << 20
	photoBody := testPNG(t, 2000, 1000)
	body, contentType := multipartBody(t, "hello from another device", 3, []uploadFixture{
		{Name: "photo.png", MIME: "image/png", Body: photoBody},
		{Name: "unsafe.html", MIME: "text/html", Body: []byte("<script>alert(1)</script>")},
		{Name: "lesson.mp4", MIME: "video/mp4", Body: []byte("small-video-fixture")},
	})
	created := ownerRequestWithHeaders(env, http.MethodPost, "/api/v1/items", body, map[string]string{"Content-Type": contentType})
	if created.Code != http.StatusCreated {
		t.Fatalf("create item = %d: %s", created.Code, created.Body.String())
	}
	var item itemResponse
	decodeResponse(t, created, &item)
	if item.TextPreview != "hello from another device" || len(item.Attachments) != 3 || item.Source != string(RoleOwner) {
		t.Fatalf("created item = %#v", item)
	}
	if item.ExpiresAt.Sub(item.CreatedAt) != 3*24*time.Hour {
		t.Fatalf("created TTL = %v", item.ExpiresAt.Sub(item.CreatedAt))
	}
	if item.Attachments[0].PreviewURL == "" {
		t.Fatal("image attachment is missing preview URL")
	}
	preview := ownerRequest(env, http.MethodGet, item.Attachments[0].PreviewURL, nil)
	previewConfig, format, previewErr := image.DecodeConfig(bytes.NewReader(preview.Body.Bytes()))
	if preview.Code != http.StatusOK || preview.Header().Get("Content-Type") != "image/jpeg" || previewErr != nil || format != "jpeg" || previewConfig.Width != 1280 || previewConfig.Height != 640 {
		t.Fatalf("image preview = %d %s %#v, %v", preview.Code, format, previewConfig, previewErr)
	}
	previewETag := preview.Header().Get("ETag")
	previewCached := ownerRequestWithHeaders(env, http.MethodGet, item.Attachments[0].PreviewURL, nil, map[string]string{"If-None-Match": previewETag})
	if previewETag == "" || previewCached.Code != http.StatusNotModified || previewCached.Body.Len() != 0 {
		t.Fatalf("preview revalidation = %d %q, etag %q", previewCached.Code, previewCached.Body.String(), previewETag)
	}

	listed := ownerRequest(env, http.MethodGet, "/api/v1/items", nil)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), item.ID) {
		t.Fatalf("list items = %d: %s", listed.Code, listed.Body.String())
	}

	unsafe := ownerRequest(env, http.MethodGet, item.Attachments[1].DownloadURL, nil)
	if unsafe.Code != http.StatusOK || unsafe.Header().Get("Content-Type") != "application/octet-stream" || !strings.HasPrefix(unsafe.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatalf("unsafe attachment headers = %#v", unsafe.Header())
	}
	unsafeETag := unsafe.Header().Get("ETag")
	if cacheControl := unsafe.Header().Get("Cache-Control"); cacheControl != "private, no-cache" || unsafeETag == "" {
		t.Fatalf("attachment cache headers = %#v", unsafe.Header())
	} else {
		revalidated := ownerRequestWithHeaders(env, http.MethodGet, item.Attachments[1].DownloadURL, nil, map[string]string{"If-None-Match": unsafeETag})
		if revalidated.Code != http.StatusNotModified || revalidated.Body.Len() != 0 {
			t.Fatalf("attachment revalidation = %d %q", revalidated.Code, revalidated.Body.String())
		}
	}
	unauthorized := perform(env.public, http.MethodGet, item.Attachments[0].DownloadURL, nil, nil, "203.0.113.5:1000")
	assertErrorCode(t, unauthorized, http.StatusUnauthorized, "unauthorized")
	video := ownerRequest(env, http.MethodGet, item.Attachments[2].DownloadURL, nil)
	if video.Code != http.StatusOK || video.Header().Get("Content-Type") != "video/mp4" || !strings.HasPrefix(video.Header().Get("Content-Disposition"), "inline;") {
		t.Fatalf("inline video headers = %#v", video.Header())
	}
	videoDownload := ownerRequest(env, http.MethodGet, item.Attachments[2].DownloadURL+"?download=1", nil)
	if !strings.HasPrefix(videoDownload.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatalf("video download headers = %#v", videoDownload.Header())
	}

	deleted := ownerRequest(env, http.MethodDelete, "/api/v1/items/"+item.ID, nil)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete item = %d: %s", deleted.Code, deleted.Body.String())
	}
	deletedAttachment := ownerRequestWithHeaders(env, http.MethodGet, item.Attachments[1].DownloadURL, nil, map[string]string{"If-None-Match": unsafeETag})
	assertErrorCode(t, deletedAttachment, http.StatusNotFound, "not_found")
	usage, err := env.store.Usage(context.Background())
	if err != nil || usage.ItemCount != 0 || usage.AttachmentCount != 0 || usage.UsedBytes != 0 {
		t.Fatalf("usage after delete = %#v, %v", usage, err)
	}
}

func TestTrafficIsSeparatedByEntryPoint(t *testing.T) {
	env := newTestEnvironment(t)
	_ = ownerRequest(env, http.MethodGet, "/assets/app.js", nil)
	_ = perform(env.public, http.MethodGet, "/api/v1/items", nil, nil, "203.0.113.5:1000")
	status := ownerRequest(env, http.MethodGet, "/api/v1/status", nil)
	if status.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", status.Code, status.Body.String())
	}
	var response struct {
		Traffic store.TrafficReport `json:"traffic"`
	}
	decodeResponse(t, status, &response)
	if response.Traffic.Last24Hours.PublicBytes == 0 || response.Traffic.Last24Hours.TailscaleBytes == 0 {
		t.Fatalf("traffic separation = %#v", response.Traffic.Last24Hours)
	}
}

func TestFailedMultipartLeavesNoVisibleItemOrTempFile(t *testing.T) {
	env := newTestEnvironment(t)
	env.api.cfg.MaxAttachmentBytes = 4
	body, contentType := multipartBody(t, "", 1, []uploadFixture{
		{Name: "small.bin", MIME: "application/octet-stream", Body: []byte("1234")},
		{Name: "large.bin", MIME: "application/octet-stream", Body: []byte("12345")},
	})
	response := ownerRequestWithHeaders(env, http.MethodPost, "/api/v1/items", body, map[string]string{"Content-Type": contentType})
	assertErrorCode(t, response, http.StatusRequestEntityTooLarge, "part_too_large")
	items, err := env.store.ListItems(context.Background(), store.ListOptions{})
	if err != nil || len(items) != 0 {
		t.Fatalf("items after failed upload = %#v, %v", items, err)
	}
	entries, err := os.ReadDir(env.store.TmpDir())
	if err != nil || len(entries) != 0 {
		t.Fatalf("tmp after failed upload = %#v, %v", entries, err)
	}
}

func TestSSEDeliversSyncAndChangeNotification(t *testing.T) {
	env := newTestEnvironment(t)
	server := httptest.NewServer(env.owner)
	defer server.Close()
	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Tailscale-User-Login", "owner@example.com")
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("open SSE: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	reader := bufio.NewReader(response.Body)
	first := readSSEBlock(t, reader)
	if !strings.Contains(first, "event: sync") {
		t.Fatalf("first SSE event = %q", first)
	}
	env.hub.Publish("created", "item-123")
	second := readSSEBlock(t, reader)
	if !strings.Contains(second, "event: items_changed") || !strings.Contains(second, `"item_id":"item-123"`) {
		t.Fatalf("second SSE event = %q", second)
	}
}

func newTestEnvironment(t *testing.T) *testEnvironment {
	t.Helper()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	storage, err := store.Open(context.Background(), store.Options{
		DataDir: t.TempDir(), QuotaBytes: 1 << 20, InlineTextBytes: 16, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	authService, err := auth.NewService(storage, auth.Options{
		CodeTTL: 30 * time.Minute, SessionTTL: 12 * time.Hour, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		TailscaleUsers: map[string]struct{}{"owner@example.com": {}},
		PublicURL:      "https://drop.example.test",
		HermesToken:    strings.Repeat("h", 40), CookieName: "drop_session", CookieSecure: false,
		SessionTTL: 12 * time.Hour, MaxTextBytes: 1024, MaxAttachmentBytes: 1024,
		MaxItemBytes: 4096, MaxAttachments: 10,
	}
	hub := NewHub()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := New(cfg, storage, authService, hub, logger)
	return &testEnvironment{
		api: api, store: storage, cfg: cfg, hub: hub,
		public: api.Handler(EntryPublic), owner: api.Handler(EntryTailscale), hermes: api.Handler(EntryHermes),
	}
}

func ownerRequest(env *testEnvironment, method, target string, body io.Reader) *httptest.ResponseRecorder {
	return ownerRequestWithHeaders(env, method, target, body, nil)
}

func ownerRequestWithHeaders(env *testEnvironment, method, target string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	if headers == nil {
		headers = make(map[string]string)
	}
	headers["Tailscale-User-Login"] = "owner@example.com"
	return perform(env.owner, method, target, body, headers, "127.0.0.1:1000")
}

func perform(handler http.Handler, method, target string, body io.Reader, headers map[string]string, remote string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, body)
	request.RemoteAddr = remote
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

type uploadFixture struct {
	Name string
	MIME string
	Body []byte
}

func multipartBody(t *testing.T, text string, ttlDays int, files []uploadFixture) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if text != "" {
		if err := writer.WriteField("text", text); err != nil {
			t.Fatal(err)
		}
	}
	if ttlDays != 0 {
		if err := writer.WriteField("ttl_days", quoteInt(ttlDays)); err != nil {
			t.Fatal(err)
		}
	}
	for _, fixture := range files {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="files"; filename="`+fixture.Name+`"`)
		header.Set("Content-Type", fixture.MIME)
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(fixture.Body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

func assertErrorCode(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status || !strings.Contains(response.Body.String(), `"code":"`+code+`"`) {
		t.Fatalf("response = %d %s, want %d/%s", response.Code, response.Body.String(), status, code)
	}
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, value any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), value); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
}

func quoteJSON(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func quoteInt(value int) string {
	return strconv.Itoa(value)
}

func readSSEBlock(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	var builder strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE: %v", err)
		}
		builder.WriteString(line)
		if line == "\n" {
			return builder.String()
		}
	}
}

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	imageData := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			imageData.SetRGBA(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 180, A: 255})
		}
	}
	var content bytes.Buffer
	if err := png.Encode(&content, imageData); err != nil {
		t.Fatal(err)
	}
	return content.Bytes()
}
