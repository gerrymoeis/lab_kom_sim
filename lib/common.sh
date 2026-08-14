#!/usr/bin/env bash
# =============================================================================
# SIMLABKOM — E2E Production Test — helper library
# Dipakai oleh run_e2e.sh. Berisi log, curl + cookie + CSRF, dan verifikasi.
# =============================================================================
set -euo pipefail

# --- Global (di-set oleh run_e2e.sh sebelum source) ---
# E2E_ROOT, LOG_DIR, OLD_PORT, NEW_PORT, OLD_BASE, NEW_BASE, COOKIE_DIR, ASSETS_DIR

log()    { echo "[$(date '+%H:%M:%S')] $*" | tee -a "${PHASE_LOG:-$LOG_DIR/console.log}"; }
phase()  { log "===== $1 ====="; }
warn()   { log "WARN: $*"; }
fail()   { log "FAIL: $*"; exit 1; }

# csrf_get <url> <out_html> — GET dengan cookie jar, simpan csrf_token dari meta tag.
csrf_get() {
    local url="$1" out="$2"
    curl -sf -c "$COOKIE_JAR" -b "$COOKIE_JAR" "$url" -o "$out" || fail "GET $url gagal"
    grep -o 'name="csrf-token" content="[^"]*"' "$out" | head -1 \
        | sed 's/.*content="//; s/"//' || true
}

# csrf_token <out_html> — extract token dari file HTML yang sudah di-download.
csrf_token() {
    grep -o 'name="csrf-token" content="[^"]*"' "$1" | head -1 \
        | sed 's/.*content="//; s/"//' || true
}

# login <user> <pass> — login via GET /login + POST /login. Simpan cookie jar per user.
login() {
    local user="$1" pass="$2"
    COOKIE_JAR="$COOKIE_DIR/${user}.jar"
    rm -f "$COOKIE_JAR"
    local login_html="$COOKIE_DIR/${user}_login.html"
    local tok
    tok=$(csrf_get "$OLD_BASE/login" "$login_html")
    [ -n "$tok" ] || fail "login $user: tidak dapat csrf token dari /login"
    curl -sf -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$OLD_BASE/login" \
        -d "_csrf=$tok" --data-urlencode "username=$user" \
        --data-urlencode "password=$pass" -o "$COOKIE_DIR/${user}_post.html" \
        || fail "login $user: POST /login gagal"
    # Token baru dibuat saat login; ambil dari halaman post-login bila ada.
    # Verifikasi login sukses: coba akses /dashboard.
    local dash="$COOKIE_DIR/${user}_dash.html"
    curl -sf -c "$COOKIE_JAR" -b "$COOKIE_JAR" "$OLD_BASE/dashboard" -o "$dash" \
        || fail "login $user: /dashboard tidak bisa diakses (login gagal?)"
    log "login OK: $user"
}

# post_form <cookie_user> <url> <data...> — POST form dengan csrf fresh (per-session,
# diambil dari GET /dashboard; token session berlaku untuk semua POST).
post_form() {
    local user="$1" url="$2"; shift 2
    COOKIE_JAR="$COOKIE_DIR/${user}.jar"
    local page="$COOKIE_DIR/${user}_page.html"
    local tok
    tok=$(csrf_get "$OLD_BASE/dashboard" "$page")
    [ -n "$tok" ] || fail "post_form: tidak dapat csrf dari /dashboard"
    curl -sf -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$url" \
        -d "_csrf=$tok" "$@" -o "$COOKIE_DIR/${user}_post.html" \
        || fail "post_form: POST $url gagal"
}

# api_post_json <cookie_user> <url> <json_data> — POST JSON dengan X-CSRF-Token header.
api_post_json() {
    local user="$1" url="$2" json="$3"
    COOKIE_JAR="$COOKIE_DIR/${user}.jar"
    local page="$COOKIE_DIR/${user}_apipage.html"
    local tok
    tok=$(csrf_get "$OLD_BASE/dashboard" "$page")
    [ -n "$tok" ] || fail "api_post_json: tidak dapat csrf dari /dashboard"
    curl -sf -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$url" \
        -H "Content-Type: application/json" \
        -H "X-CSRF-Token: $tok" \
        -d "$json" -o "$COOKIE_DIR/${user}_api.json" \
        || fail "api_post_json: POST $url gagal"
}

# api_upload <cookie_user> <url> <file> <type> <label> — multipart upload image.
api_upload() {
    local user="$1" url="$2" file="$3" type="$4" label="$5"
    COOKIE_JAR="$COOKIE_DIR/${user}.jar"
    local page="$COOKIE_DIR/${user}_uppage.html"
    local tok
    tok=$(csrf_get "$OLD_BASE/dashboard" "$page")
    [ -n "$tok" ] || fail "api_upload: tidak dapat csrf"
    curl -sf -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$url" \
        -H "X-CSRF-Token: $tok" \
        -F "image=@$file" -F "type=$type" -F "label=$label" \
        -o "$COOKIE_DIR/${user}_upload.json" \
        || fail "api_upload: POST $url gagal"
    grep -q '"success":true' "$COOKIE_DIR/${user}_upload.json" \
        || fail "api_upload: response bukan success: $(cat "$COOKIE_DIR/${user}_upload.json")"
}

# db_count <db_path> <table> — count baris (sqlite3 CLI).
db_count() {
    sqlite3 "$1" "SELECT COUNT(*) FROM $2;" 2>/dev/null || echo "0"
}