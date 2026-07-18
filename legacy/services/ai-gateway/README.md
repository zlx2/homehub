# HomeHub AI Gateway

Internal OpenAI-compatible gateway for HomeHub business services. It currently
routes the stable aliases `fast`, `reasoning`, and `coding` to DeepSeek and
OpenCode Go. Provider names and upstream model IDs stay outside business code.

## Security boundary

- The service has no host port or Traefik route.
- It accepts only a short-lived Ed25519 token with audience `ai-gateway`, scope
  `ai.use`, a non-empty authorized party (`azp`), and a signed model allowlist.
- Business-service identity tokens are rejected because their audience differs.
- Provider credentials are read from Docker secret files materialized by BWS.
- Request and response bodies are never written to application logs.
- Client cookies, authorization, and HomeHub headers are not sent upstream.

## API

- `GET /health/live`
- `GET /health/ready`
- `GET /v1/models`
- `POST /v1/chat/completions`

Chat requests preserve OpenAI-compatible extension fields. The gateway replaces
the stable alias with the configured upstream model. When `stream=true`, the
provider response is flushed as SSE and cancellation follows the caller's
request context.

Provider routing lives in `deploy/ai-gateway/providers.json`. API keys must be
stored in the `HomeHub Production` Bitwarden project as
`ai_deepseek_api_key` and `ai_opencode_go_api_key`.
