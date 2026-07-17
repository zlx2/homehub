use std::fmt;
use std::fs;
use std::path::Path;
use std::time::{SystemTime, UNIX_EPOCH};

use base64::Engine as _;
use base64::engine::general_purpose::{STANDARD, STANDARD_NO_PAD, URL_SAFE, URL_SAFE_NO_PAD};
use ed25519_dalek::{Signature, VerifyingKey};
use serde::{Deserialize, Serialize};

pub const HEADER_NAME: &str = "X-HomeHub-Identity";
pub const ISSUER: &str = "homehub-control";
const MAX_TOKEN_SIZE: usize = 8192;
const MAX_TTL_SECONDS: i64 = 90;
const CLOCK_SKEW_SECONDS: i64 = 30;

#[derive(Clone, Debug, Deserialize, Serialize, PartialEq, Eq)]
pub struct Claims {
    #[serde(rename = "iss")]
    pub issuer: String,
    #[serde(rename = "aud")]
    pub audience: String,
    #[serde(rename = "sub")]
    pub subject: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub scopes: Vec<String>,
    #[serde(rename = "azp", default)]
    pub authorized_party: String,
    #[serde(default)]
    pub models: Vec<String>,
    #[serde(rename = "iat")]
    pub issued_at: i64,
    #[serde(rename = "exp")]
    pub expires: i64,
}

impl Claims {
    pub fn has_scope(&self, expected: &str) -> bool {
        self.scopes.iter().any(|scope| scope == expected)
    }

    pub fn has_any_scope(&self, expected: &[&str]) -> bool {
        expected.iter().any(|scope| self.has_scope(scope))
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum VerifyError {
    InvalidKey,
    InvalidToken,
    UnsupportedToken,
    InvalidSignature,
    InvalidClaims,
    Clock,
}

impl fmt::Display for VerifyError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(match self {
            Self::InvalidKey => "invalid HomeHub identity public key",
            Self::InvalidToken => "invalid HomeHub identity token",
            Self::UnsupportedToken => "unsupported HomeHub identity token",
            Self::InvalidSignature => "invalid HomeHub identity signature",
            Self::InvalidClaims => "invalid HomeHub identity claims",
            Self::Clock => "system clock is unavailable",
        })
    }
}

impl std::error::Error for VerifyError {}

#[derive(Clone)]
pub struct Verifier {
    key: VerifyingKey,
    audience: String,
}

#[derive(Deserialize)]
struct Header {
    alg: String,
    typ: String,
}

impl Verifier {
    pub fn from_public_key_file(path: impl AsRef<Path>, audience: impl Into<String>) -> Result<Self, VerifyError> {
        let value = fs::read_to_string(path).map_err(|_| VerifyError::InvalidKey)?;
        Self::from_encoded_public_key(&value, audience)
    }

    pub fn from_encoded_public_key(value: &str, audience: impl Into<String>) -> Result<Self, VerifyError> {
        let trimmed = value.trim();
        let decoded = [URL_SAFE_NO_PAD, URL_SAFE, STANDARD_NO_PAD, STANDARD]
            .iter()
            .find_map(|encoding| encoding.decode(trimmed).ok().filter(|bytes| bytes.len() == 32))
            .ok_or(VerifyError::InvalidKey)?;
        let key_bytes: [u8; 32] = decoded.try_into().map_err(|_| VerifyError::InvalidKey)?;
        let key = VerifyingKey::from_bytes(&key_bytes).map_err(|_| VerifyError::InvalidKey)?;
        let audience = audience.into();
        if audience.trim().is_empty() {
            return Err(VerifyError::InvalidKey);
        }
        Ok(Self { key, audience })
    }

    pub fn verify(&self, token: &str) -> Result<Claims, VerifyError> {
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map_err(|_| VerifyError::Clock)?
            .as_secs() as i64;
        self.verify_at(token, now)
    }

    fn verify_at(&self, token: &str, now: i64) -> Result<Claims, VerifyError> {
        if token.is_empty() || token.len() > MAX_TOKEN_SIZE {
            return Err(VerifyError::InvalidToken);
        }
        let mut parts = token.split('.');
        let encoded_header = parts.next().ok_or(VerifyError::InvalidToken)?;
        let encoded_claims = parts.next().ok_or(VerifyError::InvalidToken)?;
        let encoded_signature = parts.next().ok_or(VerifyError::InvalidToken)?;
        if parts.next().is_some() {
            return Err(VerifyError::InvalidToken);
        }
        let header: Header = serde_json::from_slice(
            &URL_SAFE_NO_PAD.decode(encoded_header).map_err(|_| VerifyError::InvalidToken)?,
        )
        .map_err(|_| VerifyError::InvalidToken)?;
        if header.alg != "EdDSA" || header.typ != "JWT" {
            return Err(VerifyError::UnsupportedToken);
        }
        let signature_bytes = URL_SAFE_NO_PAD
            .decode(encoded_signature)
            .map_err(|_| VerifyError::InvalidSignature)?;
        let signature = Signature::from_slice(&signature_bytes).map_err(|_| VerifyError::InvalidSignature)?;
        let unsigned = format!("{encoded_header}.{encoded_claims}");
        self.key
            .verify_strict(unsigned.as_bytes(), &signature)
            .map_err(|_| VerifyError::InvalidSignature)?;
        let claims: Claims = serde_json::from_slice(
            &URL_SAFE_NO_PAD.decode(encoded_claims).map_err(|_| VerifyError::InvalidToken)?,
        )
        .map_err(|_| VerifyError::InvalidToken)?;
        if claims.issuer != ISSUER
            || claims.audience != self.audience
            || claims.subject.is_empty()
            || claims.expires <= now
            || claims.issued_at > now + CLOCK_SKEW_SECONDS
            || claims.expires < claims.issued_at
            || claims.expires - claims.issued_at > MAX_TTL_SECONDS
        {
            return Err(VerifyError::InvalidClaims);
        }
        Ok(claims)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::{Signer as _, SigningKey};

    const NOW: i64 = 1_784_290_400;

    fn claims() -> Claims {
        Claims {
            issuer: ISSUER.to_owned(),
            audience: "notes".to_owned(),
            subject: "owner-1".to_owned(),
            name: "Luna".to_owned(),
            scopes: vec!["admin".to_owned(), "portal.view".to_owned()],
            authorized_party: String::new(),
            models: Vec::new(),
            issued_at: NOW,
            expires: NOW + 60,
        }
    }

    fn sign(key: &SigningKey, claims: &Claims) -> String {
        let header = URL_SAFE_NO_PAD.encode(br#"{"alg":"EdDSA","typ":"JWT"}"#);
        let payload = URL_SAFE_NO_PAD.encode(serde_json::to_vec(claims).unwrap());
        let unsigned = format!("{header}.{payload}");
        let signature = key.sign(unsigned.as_bytes());
        format!("{unsigned}.{}", URL_SAFE_NO_PAD.encode(signature.to_bytes()))
    }

    fn verifier(key: &SigningKey) -> Verifier {
        let encoded = URL_SAFE_NO_PAD.encode(key.verifying_key().to_bytes());
        Verifier::from_encoded_public_key(&encoded, "notes").unwrap()
    }

    #[test]
    fn accepts_service_bound_token() {
        let key = SigningKey::from_bytes(&[7_u8; 32]);
        let got = verifier(&key).verify_at(&sign(&key, &claims()), NOW).unwrap();
        assert_eq!(got.subject, "owner-1");
        assert!(got.has_scope("admin"));
    }

    #[test]
    fn rejects_wrong_audience_expiry_and_signature() {
        let key = SigningKey::from_bytes(&[7_u8; 32]);
        let other_key = SigningKey::from_bytes(&[8_u8; 32]);
        let verifier = verifier(&key);

        let mut wrong_audience = claims();
        wrong_audience.audience = "other".to_owned();
        assert_eq!(verifier.verify_at(&sign(&key, &wrong_audience), NOW), Err(VerifyError::InvalidClaims));

        let mut expired = claims();
        expired.expires = NOW;
        assert_eq!(verifier.verify_at(&sign(&key, &expired), NOW), Err(VerifyError::InvalidClaims));
        assert_eq!(verifier.verify_at(&sign(&other_key, &claims()), NOW), Err(VerifyError::InvalidSignature));
    }
}
