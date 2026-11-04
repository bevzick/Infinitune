#!/usr/bin/env bash
set -Eeuo pipefail

template_file=/etc/kong/kong.template.yml
config_file="${KONG_DECLARATIVE_CONFIG:-/tmp/kong.yml}"
public_key_der=/tmp/auth-jwt-public-key.der
public_key_pem=/tmp/auth-jwt-public-key.pem

cleanup() {
  rm -f "$public_key_der" "$public_key_pem"
}

trap cleanup EXIT

if [[ -z "${AUTH_JWT_PUBLIC_KEY_BASE64:-}" ]]; then
  echo "AUTH_JWT_PUBLIC_KEY_BASE64 is required" >&2
  exit 1
fi

if [[ ! "${AUTH_JWT_ISSUER:-}" =~ ^[a-zA-Z0-9._:/-]+$ ]]; then
  echo "AUTH_JWT_ISSUER must contain only letters, digits, dots, underscores, colons, slashes, and hyphens" >&2
  exit 1
fi

umask 077

if ! printf '%s' "$AUTH_JWT_PUBLIC_KEY_BASE64" | base64 -d > "$public_key_der"; then
  echo "AUTH_JWT_PUBLIC_KEY_BASE64 must be valid base64" >&2
  exit 1
fi

if ! openssl pkey \
  -pubin \
  -inform DER \
  -in "$public_key_der" \
  -out "$public_key_pem" >/dev/null 2>&1; then
  echo "AUTH_JWT_PUBLIC_KEY_BASE64 must contain an Ed25519 public key in DER format" >&2
  exit 1
fi

if ! openssl pkey -pubin -in "$public_key_pem" -text -noout 2>/dev/null \
  | grep -q '^ED25519 Public-Key:'; then
  echo "AUTH_JWT_PUBLIC_KEY_BASE64 must contain an Ed25519 public key" >&2
  exit 1
fi

awk -v issuer="$AUTH_JWT_ISSUER" -v key_file="$public_key_pem" '
  /^[[:space:]]*__AUTH_JWT_PUBLIC_KEY__[[:space:]]*$/ {
    indentation = substr($0, 1, index($0, "_") - 1)
    while ((getline line < key_file) > 0) {
      print indentation line
    }
    close(key_file)
    next
  }

  {
    gsub(/__AUTH_JWT_ISSUER__/, issuer)
    print
  }
' "$template_file" > "$config_file"

cleanup
trap - EXIT

kong config parse "$config_file" >/dev/null

exec /entrypoint.sh kong docker-start
