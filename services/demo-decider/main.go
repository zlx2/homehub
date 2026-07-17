package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const maxBodyBytes = 8 << 10

type app struct {
	allowedOrigins map[string]struct{}
}

type decideRequest struct {
	Options []string `json:"options"`
}

type decideResponse struct {
	Choice    string    `json:"choice"`
	Index     int       `json:"index"`
	DecidedAt time.Time `json:"decided_at"`
}

func main() {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "serve":
		if err := serve(); err != nil {
			slog.Error("server stopped", "error", err)
			os.Exit(1)
		}
	case "healthcheck":
		if err := healthcheck(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", command)
		os.Exit(2)
	}
}

func serve() error {
	listenAddress := envOrDefault("DEMO_DECIDER_LISTEN_ADDRESS", ":8080")
	application := &app{allowedOrigins: parseOrigins(os.Getenv("DEMO_DECIDER_ALLOWED_ORIGINS"))}

	server := &http.Server{
		Addr:              listenAddress,
		Handler:           application.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	slog.Info("demo decider listening", "address", listenAddress)
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.handlePage)
	mux.HandleFunc("GET /assets/app.css", a.handleCSS)
	mux.HandleFunc("GET /assets/app.js", a.handleJS)
	mux.HandleFunc("POST /api/decide", a.handleDecide)
	mux.HandleFunc("GET /health/live", a.handleHealth)
	return requestLogger(mux)
}

func (a *app) handlePage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, pageHTML)
}

func (a *app) handleCSS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = io.WriteString(w, appCSS)
}

func (a *app) handleJS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = io.WriteString(w, appJS)
}

func (a *app) handleDecide(w http.ResponseWriter, r *http.Request) {
	if !a.originAllowed(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "origin_not_allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input decideRequest
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	if err := ensureJSONEOF(decoder); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	options, err := normalizeOptions(input.Options)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	selected, err := rand.Int(rand.Reader, big.NewInt(int64(len(options))))
	if err != nil {
		slog.Error("random selection failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	index := int(selected.Int64())
	writeJSON(w, http.StatusOK, decideResponse{Choice: options[index], Index: index, DecidedAt: time.Now().UTC()})
}

func (a *app) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *app) originAllowed(r *http.Request) bool {
	_, allowed := a.allowedOrigins[r.Header.Get("Origin")]
	return allowed
}

func parseOrigins(raw string) map[string]struct{} {
	origins := make(map[string]struct{})
	for _, value := range strings.Split(raw, ",") {
		if origin := strings.TrimSpace(value); origin != "" {
			origins[origin] = struct{}{}
		}
	}
	return origins
}

func normalizeOptions(input []string) ([]string, error) {
	if len(input) < 2 || len(input) > 12 {
		return nil, errors.New("options_must_contain_2_to_12_items")
	}
	options := make([]string, 0, len(input))
	for _, raw := range input {
		option := strings.TrimSpace(raw)
		if option == "" || len([]rune(option)) > 80 {
			return nil, errors.New("each_option_must_contain_1_to_80_characters")
		}
		options = append(options, option)
	}
	return options, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values")
	}
	return err
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}

func healthcheck() error {
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://127.0.0.1:8080/health/live")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected health status %d", response.StatusCode)
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

const pageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>随机决策器 · HomeHub</title>
  <link rel="stylesheet" href="/demo/decider/assets/app.css">
</head>
<body>
  <main>
    <a class="back" href="/">← 返回 HomeHub</a>
    <section class="card">
      <p class="eyebrow">GO · 无状态服务</p>
      <h1>随机决策器</h1>
      <p class="intro">把纠结交给密码学安全的随机数。每行填写一个选项，最多 12 个。</p>
      <form id="form">
        <label for="options">候选项</label>
        <textarea id="options" rows="7">今天吃面
今天吃饭
今天吃饺子</textarea>
        <button type="submit">替我决定</button>
      </form>
      <div id="result" class="result" aria-live="polite" hidden></div>
    </section>
  </main>
  <script src="/demo/decider/assets/app.js" defer></script>
</body>
</html>`

const appCSS = `:root{color-scheme:dark;font-family:Inter,ui-sans-serif,system-ui,sans-serif;background:#0b1020;color:#eef2ff}*{box-sizing:border-box}body{margin:0;min-height:100vh;background:radial-gradient(circle at 20% 10%,#233765 0,transparent 38%),#0b1020}main{width:min(720px,calc(100% - 32px));margin:0 auto;padding:48px 0}.back{color:#a5b4fc;text-decoration:none}.card{margin-top:24px;padding:clamp(24px,5vw,48px);border:1px solid #334155;border-radius:24px;background:#111a30cc;box-shadow:0 24px 80px #0006}.eyebrow{margin:0;color:#67e8f9;font-size:.78rem;font-weight:800;letter-spacing:.14em}h1{margin:.5rem 0;font-size:clamp(2rem,7vw,4rem)}.intro{color:#b7c2d9;line-height:1.7}label{display:block;margin:28px 0 8px;font-weight:700}textarea{width:100%;resize:vertical;border:1px solid #475569;border-radius:14px;padding:14px;background:#080d19;color:#fff;font:inherit;line-height:1.7}textarea:focus{outline:2px solid #67e8f9;outline-offset:2px}button{margin-top:16px;border:0;border-radius:999px;padding:13px 22px;background:#67e8f9;color:#082f49;font:inherit;font-weight:900;cursor:pointer}.result{margin-top:24px;padding:22px;border:1px solid #4f46e5;border-radius:16px;background:#312e81}.result strong{display:block;margin-top:6px;font-size:1.7rem}.result.error{border-color:#ef4444;background:#450a0a}`

const appJS = `const form=document.querySelector('#form');const result=document.querySelector('#result');form.addEventListener('submit',async(event)=>{event.preventDefault();const options=document.querySelector('#options').value.split('\n').map(v=>v.trim()).filter(Boolean);result.hidden=false;result.className='result';result.textContent='正在决定…';try{const response=await fetch('/demo/decider/api/decide',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({options})});const body=await response.json();if(!response.ok)throw new Error(body.error||'请求失败');result.innerHTML='<span>这次选中的是</span><strong></strong>';result.querySelector('strong').textContent=body.choice}catch(error){result.classList.add('error');result.textContent='无法决定：'+error.message}});`
