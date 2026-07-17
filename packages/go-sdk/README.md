# HomeHub Go SDK

The Go SDK contains the security and HTTP plumbing shared by Go business
services. `identity` verifies short-lived, service-bound Ed25519 tokens issued
by HomeHub Control. `httpx` provides request IDs, panic recovery, and baseline
response headers.

AI delegations use the same verifier but add `azp` and `models` claims and use
`ai-gateway` as their audience. Business services receive them in
`X-HomeHub-AI-Identity` and forward that value only to the internal AI Gateway
as `X-HomeHub-Identity`.

Business services receive only the Ed25519 public key. They must never receive
the Control signing seed or trust unsigned `X-HomeHub-*` client headers.
