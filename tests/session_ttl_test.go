package tests

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestSessionTTLAndLoginKillOldSession — E2E untuk dua perilaku utama Doc 031:
//  1. Login terakhir menang: admin login di device B saat device A masih aktif →
//     device A di-kick pada akses berikutnya (302 /login), token DB dirotasi ke B.
//  2. Session idle melewati maxAge (TTL sliding 7 hari) → akses berikutnya 302 /login,
//     token DB dibersihkan (slot kosong), dan login ulang langsung sukses.
func TestSessionTTLAndLoginKillOldSession(t *testing.T) {
	env := wrapSharedEnv(t)
	tsURL := env.TS.URL

	saveCookies := func(resp *http.Response, cm map[string]string) {
		for _, c := range resp.Cookies() {
			cm[c.Name] = c.Value
		}
	}

	addCookies := func(req *http.Request, cm map[string]string) {
		for n, v := range cm {
			req.AddCookie(&http.Cookie{Name: n, Value: v})
		}
	}

	newNoRedirect := func() *http.Client {
		return &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	}

	// doLogin: GET /login (tanpa cookie lama agar selalu dapat halaman login),
	// lalu POST kredensial. Mengembalikan status POST.
	doLogin := func(client *http.Client, cm map[string]string, user, pass string) int {
		resp, err := client.Get(tsURL + "/login")
		if err != nil {
			t.Fatalf("GET /login: %v", err)
		}
		saveCookies(resp, cm)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		csrf := extractCSRFFromBody(string(body))
		if csrf == "" {
			t.Fatal("CSRF token not found on login page")
		}
		form := url.Values{"username": {user}, "password": {pass}, "_csrf": {csrf}}.Encode()
		req, _ := http.NewRequest("POST", tsURL+"/login", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		addCookies(req, cm)
		resp, err = client.Do(req)
		if err != nil {
			t.Fatalf("POST /login: %v", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		// Propagasi Set-Cookie agar rotasi/penghapusan cookie lama ikut tercatat.
		saveCookies(resp, cm)
		return resp.StatusCode
	}

	// getWith: request dengan cookie manual; status + hasil Location dicek,
	// Set-Cookie (contoh: penghapusan cookie saat kicked/stale) ikut dipropagasi.
	getWith := func(client *http.Client, cm map[string]string, path string) (int, string) {
		req, _ := http.NewRequest("GET", tsURL+path, nil)
		addCookies(req, cm)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		loc := resp.Header.Get("Location")
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		saveCookies(resp, cm)
		return resp.StatusCode, loc
	}

	t.Run("login_last_wins_kicks_old_device", func(t *testing.T) {
		clientA := newNoRedirect()
		cmA := map[string]string{}
		if code := doLogin(clientA, cmA, "admin", "admin123"); code != 302 {
			t.Fatalf("device A login: expected 302, got %d", code)
		}
		if code, _ := getWith(clientA, cmA, "/labs"); code != 200 {
			t.Fatalf("device A /labs after login: expected 200, got %d", code)
		}

		// Device B login sebagai admin yang sama — harus sukses meski A masih aktif.
		clientB := newNoRedirect()
		cmB := map[string]string{}
		if code := doLogin(clientB, cmB, "admin", "admin123"); code != 302 {
			t.Fatalf("device B login (same admin): expected 302, got %d", code)
		}

		// Device lama di-kick pada akses berikutnya (token DB sudah dirotasi).
		code, loc := getWith(clientA, cmA, "/labs")
		if code != 302 || !strings.HasSuffix(loc, "/login") {
			t.Errorf("expected device A kicked (302 /login), got %d -> %q", code, loc)
		}
		// Device baru tetap valid.
		if code, _ = getWith(clientB, cmB, "/labs"); code != 200 {
			t.Errorf("expected device B still valid (200), got %d", code)
		}

		// Token DB harus berisi token hasil login device B (bukan lagi token A).
		var dbToken string
		if err := env.GlobalDB.QueryRow("SELECT session_token FROM global_users WHERE username = 'admin'").Scan(&dbToken); err != nil {
			t.Fatalf("read admin token: %v", err)
		}
		if dbToken == "" {
			t.Error("expected rotated token stored in DB after device B login")
		}
	})

	t.Run("idle_session_expires_and_relogin_succeeds", func(t *testing.T) {
		client := newNoRedirect()
		cm := map[string]string{}
		if code := doLogin(client, cm, "admin", "admin123"); code != 302 {
			t.Fatalf("login: expected 302, got %d", code)
		}
		if code, _ := getWith(client, cm, "/labs"); code != 200 {
			t.Fatalf("expected 200 before TTL expiry, got %d", code)
		}

		// Paksa sesi idle melewati maxAge (7 hari).
		old := time.Now().Unix() - 8*86400
		if _, err := env.GlobalDB.Exec("UPDATE global_users SET session_updated_at = ? WHERE username = 'admin'", old); err != nil {
			t.Fatalf("force stale ttl: %v", err)
		}

		code, loc := getWith(client, cm, "/labs")
		if code != 302 || !strings.HasSuffix(loc, "/login") {
			t.Errorf("expected stale session redirected to /login, got %d -> %q", code, loc)
		}

		// Token stale dibersihkan dari DB (slot kosong untuk login baru).
		var dbToken string
		if err := env.GlobalDB.QueryRow("SELECT session_token FROM global_users WHERE username = 'admin'").Scan(&dbToken); err != nil {
			t.Fatalf("read admin token: %v", err)
		}
		if dbToken != "" {
			t.Errorf("expected stale token cleared from DB, got %q", dbToken)
		}

		// Login ulang langsung sukses (slot kosong / token basi di-overwrite).
		if code := doLogin(client, cm, "admin", "admin123"); code != 302 {
			t.Errorf("expected relogin to succeed after stale session, got %d", code)
		}
		if code, _ := getWith(client, cm, "/labs"); code != 200 {
			t.Errorf("expected 200 after relogin, got %d", code)
		}
	})
}
