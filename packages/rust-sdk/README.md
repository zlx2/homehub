# HomeHub Rust SDK

The Rust SDK is framework-neutral. Its identity verifier accepts only
short-lived Ed25519 tokens issued by HomeHub Control for the configured service
audience. Web-framework adapters can wrap this core without duplicating the
security-critical token validation rules.
