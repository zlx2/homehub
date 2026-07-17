# HomeHub Go SDK

The Go SDK contains the security and HTTP plumbing shared by Go business
services. `identity` verifies short-lived, service-bound Ed25519 tokens issued
by HomeHub Control. `httpx` provides request IDs, panic recovery, and baseline
response headers.

Business services receive only the Ed25519 public key. They must never receive
the Control signing seed or trust unsigned `X-HomeHub-*` client headers.
