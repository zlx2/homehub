# HomeHub AI Gateway

Initial provider-neutral skeleton for the shared LLM entry point. It already
uses the HomeHub Go SDK, service-bound Ed25519 identity, bounded request bodies,
OpenAI-compatible paths, request IDs, structured logs, and streaming-safe HTTP
timeouts.

It is deliberately not registered in production Compose yet. A source service
must never reuse its own audience-bound identity at the AI Gateway. The
downstream delegation contract is defined before provider credentials and live
routing are added.
