package tests

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/handlers"
	"inventaris-lab-kom/internal/server"
	"inventaris-lab-kom/internal/services"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

// ── SSOT: Seed user definitions (single source of truth) ──────

type seedUserDef struct {
	Username, Password, FullName string
	IsSuperAdmin                 bool
}
type seedPermDef struct {
	Username, LabPath, Role string
}

var seedUsers = []seedUserDef{
	{"admin", "admin123", "Administrator", true},
	{"rekan", "rekan123", "Rekan Administrator", true},
	{"labA_only", "test123", "Lab A Only", false},
	{"labB_only", "test123", "Lab B Only", false},
	{"no_perm_user", "test123", "No Permission", false},
	{"labA_dosen", "test123", "Lab A Dosen", false},
}
var seedPerms = []seedPermDef{
	{"labA_only", "lab-kom-mi", "admin"},
	{"labB_only", "vokasi", "admin"},
	{"labA_dosen", "lab-kom-mi", "admin"},
}

// ── Pre-computed bcrypt hashes ───────────────────────────────
// Computed once at init to avoid 576 bcrypt recomputations during test suite.
var seedPasswords map[string]string

func init() {
	seedPasswords = make(map[string]string, len(seedUsers))
	for _, u := range seedUsers {
		h, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.MinCost)
		if err != nil {
			panic("seedPasswords init: " + err.Error())
		}
		seedPasswords[u.Username] = string(h)
	}
}

// ── DRY helpers ───────────────────────────────────────────────

func bcryptHash(pw string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		panic("bcrypt: " + err.Error())
	}
	return string(h)
}

func noRedirectClient() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func seedGlobalUsers(db *database.DB) {
	for _, u := range seedUsers {
		db.Exec("INSERT OR IGNORE INTO global_users (username, password, full_name, is_super_admin) VALUES (?, ?, ?, ?)",
			u.Username, seedPasswords[u.Username], u.FullName, boolToInt(u.IsSuperAdmin))
	}
	for _, p := range seedPerms {
		var id int
		db.QueryRow("SELECT id FROM global_users WHERE username=?", p.Username).Scan(&id)
		db.Exec("INSERT OR IGNORE INTO lab_permissions (user_id, lab_url_path, role) VALUES (?, ?, ?)", id, p.LabPath, p.Role)
	}
}

var (
	sharedEnv     *TestEnvironment
	sharedCleanup func()
	sharedTmpDir  string
)

func TestMain(m *testing.M) {
	wd, _ := os.Getwd()
	projectRoot := findProjectRoot(wd)
	if err := os.Chdir(projectRoot); err != nil {
		panic("Chdir to project root " + projectRoot + ": " + err.Error())
	}
	godotenv.Load()
	if _, err := os.Stat(filepath.Join(projectRoot, ".env.reference")); err == nil {
		godotenv.Load(filepath.Join(projectRoot, ".env.reference"))
	}

	database.SetTestMode(true)

	code := func() int {
		env, _, tmpDir, err := createSharedEnvironment()
		if err != nil {
			panic("createSharedEnvironment: " + err.Error())
		}
		sharedEnv = env
		sharedTmpDir = tmpDir
		return m.Run()
	}()
	if sharedEnv != nil {
		sharedCleanup()
		sharedEnv.DB_A.Close()
		sharedEnv.DB_B.Close()
		sharedEnv.GlobalDB.Close()
		if sharedTmpDir != "" {
			os.RemoveAll(sharedTmpDir)
		}
	}
	os.Exit(code)
}

func createSharedEnvironment() (*TestEnvironment, string, string, error) {
	tmpDir, err := os.MkdirTemp("", "simlabkom-test-shared-*")
	if err != nil {
		return nil, "", "", fmt.Errorf("MkdirTemp: %w", err)
	}
	uploadPath := filepath.Join(tmpDir, "uploads")
	dbPathA := filepath.Join(tmpDir, "testing_a.db")
	dbPathB := filepath.Join(tmpDir, "testing_b.db")
	globalDBPath := filepath.Join(tmpDir, "testing_global.db")

	labAURL := "lab-kom-mi"
	labAID := "MI-1"
	labBURL := "vokasi"
	labBID := "VOKASI-1"

	cfg := createTestConfig()
	cfg.UploadPath = uploadPath
	cfg.Labs = []config.LabConfig{
		{ID: labAID, Title: "Lab Kom MI", URLPath: labAURL, DBPath: dbPathA, UploadDir: filepath.Join(uploadPath, labAURL), Layout: config.GridLayout{ColsPerRow: []int{8, 8, 8, 8, 8}}},
		{ID: labBID, Title: "Vokasi", URLPath: labBURL, DBPath: dbPathB, UploadDir: filepath.Join(uploadPath, labBURL), Layout: config.GridLayout{ColsPerRow: []int{10, 8, 9, 9}, HasGap: true, GapPos: 4}},
	}

	dbA, err := database.InitDB(dbPathA, "")
	if err != nil {
		return nil, "", tmpDir, fmt.Errorf("InitDB lab A: %w", err)
	}
	if err := database.RunMigrations(dbA, false, labAID, labAURL, uploadPath, false); err != nil {
		return nil, "", tmpDir, fmt.Errorf("Migrate lab A: %w", err)
	}
	if err := database.SeedDefaultUser(dbA); err != nil {
		return nil, "", tmpDir, fmt.Errorf("Seed user lab A: %w", err)
	}
	dbA.Exec("UPDATE users SET session_token = NULL")

	dbB, err := database.InitDB(dbPathB, "")
	if err != nil {
		return nil, "", tmpDir, fmt.Errorf("InitDB lab B: %w", err)
	}
	if err := database.RunMigrations(dbB, false, labBID, labBURL, uploadPath, false); err != nil {
		return nil, "", tmpDir, fmt.Errorf("Migrate lab B: %w", err)
	}
	if err := database.SeedDefaultUser(dbB); err != nil {
		return nil, "", tmpDir, fmt.Errorf("Seed user lab B: %w", err)
	}
	dbB.Exec("UPDATE users SET session_token = NULL")

	globalDB, err := database.InitDB(globalDBPath, "")
	if err != nil {
		return nil, "", tmpDir, fmt.Errorf("InitDB global: %w", err)
	}
	if err := database.SetupGlobalDB(globalDB, cfg.Labs); err != nil {
		return nil, "", tmpDir, fmt.Errorf("Setup global DB: %w", err)
	}
	globalDB.Exec("UPDATE global_users SET session_token = ''")

	seedGlobalUsers(globalDB)
	dbs := map[string]*database.DB{labAURL: dbA, labBURL: dbB}
	router, cleanup, flushLogs, globalHandler := server.SetupRouter(dbs, globalDB, cfg, services.DummyNotifier{})
	sharedCleanup = cleanup

	ts := httptest.NewServer(router)

	noRedirect := func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	client := &http.Client{CheckRedirect: noRedirect}

	env := &TestEnvironment{
		LabA: &testLab{
			url: labAURL, id: labAID, prefix: "/" + labAURL,
			db: dbA, cfg: cfg.Labs[0], cookies: make(map[string]string),
			ts: ts, t: nil, client: &http.Client{CheckRedirect: noRedirect},
		},
		LabB: &testLab{
			url: labBURL, id: labBID, prefix: "/" + labBURL,
			db: dbB, cfg: cfg.Labs[1], cookies: make(map[string]string),
			ts: ts, t: nil, client: &http.Client{CheckRedirect: noRedirect},
		},
		TS:            ts,
		Client:        client,
		GlobalDB:      globalDB,
		DB_A:          dbA,
		DB_B:          dbB,
		Config:        cfg,
		FlushLogs:     flushLogs,
		GlobalHandler: globalHandler,
	}

	return env, uploadPath, tmpDir, nil
}

// TestConfigOverrides allows customising config for specific test scenarios.
type TestConfigOverrides struct {
	GeminiKey         string
	OpenRouterKey     string
	UploadPath        string
	GeminiBaseURL     string
	OpenRouterBaseURL string
}

func findProjectRoot(wd string) string {
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
		dir = parent
	}
}

type testLab struct {
	url    string
	id     string
	prefix string
	db     *database.DB
	cfg    config.LabConfig

	cookies map[string]string
	csrf    string
	ts      *httptest.Server
	t       *testing.T
	client  *http.Client
}

func (l *testLab) closeResp(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func (l *testLab) saveCookies(resp *http.Response) {
	for _, c := range resp.Cookies() {
		l.cookies[c.Name] = c.Value
	}
}

func (l *testLab) addCookies(req *http.Request) {
	for n, v := range l.cookies {
		req.AddCookie(&http.Cookie{Name: n, Value: v})
	}
}

func (l *testLab) extractCSRFToken(html string) string {
	prefix := `<meta name="csrf-token" content="`
	start := strings.Index(html, prefix)
	if start == -1 {
		return ""
	}
	start += len(prefix)
	end := strings.Index(html[start:], `"`)
	if end == -1 {
		return ""
	}
	return html[start : start+end]
}

func (l *testLab) login(username, password string) bool {
	req, _ := http.NewRequest("GET", l.ts.URL+"/login", nil)
	resp, err := l.client.Do(req)
	if err != nil {
		return false
	}
	l.saveCookies(resp)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	token := l.extractCSRFToken(string(body))
	if token == "" {
		return false
	}

	formData := "_csrf=" + url.QueryEscape(token) + "&username=" + url.QueryEscape(username) + "&password=" + url.QueryEscape(password)
	req, _ = http.NewRequest("POST", l.ts.URL+"/login", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	l.addCookies(req)
	resp, err = l.client.Do(req)
	if err != nil {
		return false
	}
	defer l.closeResp(resp)
	l.saveCookies(resp)
	l.csrf = token
	return resp.StatusCode == 302 && len(l.cookies) > 0
}

func (l *testLab) get(path string) (*http.Response, error) {
	req, _ := http.NewRequest("GET", l.ts.URL+l.prefix+path, nil)
	l.addCookies(req)
	return l.client.Do(req)
}

func (l *testLab) post(path, data string) (*http.Response, error) {
	if data == "" {
		data = "_csrf=" + url.QueryEscape(l.csrf)
	} else {
		data = data + "&_csrf=" + url.QueryEscape(l.csrf)
	}
	req, _ := http.NewRequest("POST", l.ts.URL+l.prefix+path, strings.NewReader(data))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	l.addCookies(req)
	return l.client.Do(req)
}

func (l *testLab) postJSON(path, data string) (*http.Response, error) {
	req, _ := http.NewRequest("POST", l.ts.URL+l.prefix+path, strings.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", l.csrf)
	l.addCookies(req)
	return l.client.Do(req)
}

func (l *testLab) getURL(url string) (*http.Response, error) {
	req, _ := http.NewRequest("GET", url, nil)
	l.addCookies(req)
	return l.client.Do(req)
}

func (l *testLab) getLocation(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	return resp.Header.Get("Location")
}

func (l *testLab) assertStatus(resp *http.Response, expected int) bool {
	if resp.StatusCode != expected {
		l.t.Errorf("FAIL: expected status %d, got %d", expected, resp.StatusCode)
		return false
	}
	return true
}

func (l *testLab) assertRedirect(resp *http.Response, expectedTo string) bool {
	loc := l.getLocation(resp)
	if loc != expectedTo {
		l.t.Errorf("FAIL: expected redirect to %q, got %q", expectedTo, loc)
		return false
	}
	return true
}

func (l *testLab) assertBodyContains(resp *http.Response, substr string) bool {
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), substr) {
		l.t.Errorf("FAIL: body does not contain %q", substr)
		return false
	}
	return true
}

func (l *testLab) getBody(resp *http.Response) string {
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return string(body)
}

func (l *testLab) refreshCSRF() bool {
	resp, err := l.get("/dashboard")
	if err != nil || resp.StatusCode != 200 {
		return false
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	bodyStr := string(body)
	if strings.Contains(bodyStr, "Error #") {
		fmt.Printf("  [DETECTED TEMPLATE ERROR in /dashboard] %s\n", extractErrorLine(bodyStr))
		return false
	}
	token := l.extractCSRFToken(bodyStr)
	if token == "" {
		return false
	}
	l.csrf = token
	return true
}

func extractErrorLine(s string) string {
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Error #") {
			return strings.TrimSpace(line)
		}
	}
	return s[:min(len(s), 200)]
}

type TestEnvironment struct {
	LabA, LabB           *testLab
	TS                   *httptest.Server
	Client               *http.Client
	GlobalDB, DB_A, DB_B *database.DB
	Config               *config.Config
	FlushLogs            func()
	GlobalHandler        *handlers.GlobalHandler
}

func createTestConfig(overrides ...TestConfigOverrides) *config.Config {
	cfg := TestConfigOverrides{}
	if len(overrides) > 0 {
		cfg = overrides[0]
	}
	uploadPath := cfg.UploadPath
	if uploadPath == "" {
		uploadPath = "uploads"
	}
	geminiKey := cfg.GeminiKey
	if geminiKey == "" {
		geminiKey = os.Getenv("GEMINI_API_KEY")
	}
	openRouterKey := cfg.OpenRouterKey
	if openRouterKey == "" {
		openRouterKey = os.Getenv("OPENROUTER_API_KEY")
	}
	geminiBaseURL := cfg.GeminiBaseURL
	if geminiBaseURL == "" {
		geminiBaseURL = os.Getenv("GEMINI_BASE_URL")
	}
	openRouterBaseURL := cfg.OpenRouterBaseURL
	if openRouterBaseURL == "" {
		openRouterBaseURL = os.Getenv("OPENROUTER_BASE_URL")
	}
	return &config.Config{
		SessionSecret:        "test-secret-12345",
		SessionMaxAgeSeconds: 604800,
		UploadPath:           uploadPath,
		DefaultPageSize:      25,
		GeminiAPIKey:         geminiKey,
		GeminiBaseURL:        geminiBaseURL,
		OpenRouterAPIKey:     openRouterKey,
		OpenRouterBaseURL:    openRouterBaseURL,
	}
}

func clearSessions() {
	sharedEnv.GlobalDB.Exec("UPDATE global_users SET session_token = '', session_updated_at = 0")
	for _, db := range sharedEnv.GlobalHandler.LabsDB {
		db.Exec("UPDATE users SET session_token = NULL")
	}
}

func resetGlobalState() {
	// Per-lab DB sessions (separate databases, can't be in global transaction)
	for _, db := range sharedEnv.GlobalHandler.LabsDB {
		db.Exec("UPDATE users SET session_token = NULL")
	}

	// Single transaction for all global DB operations → 1 fsync instead of 18-20
	tx, err := sharedEnv.GlobalDB.Begin()
	if err != nil {
		panic("resetGlobalState: begin tx: " + err.Error())
	}

	// Clear global sessions inside transaction
	tx.Exec("UPDATE global_users SET session_token = '', session_updated_at = 0")

	// UPSERT for each seed user — uses pre-computed hashes (avoid 576 bcrypt computations)
	for _, u := range seedUsers {
		tx.Exec(
			`INSERT INTO global_users (username, password, full_name, is_super_admin)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(username) DO UPDATE SET
				 password = excluded.password,
				 full_name = excluded.full_name,
				 is_super_admin = excluded.is_super_admin`,
			u.Username, seedPasswords[u.Username], u.FullName, boolToInt(u.IsSuperAdmin),
		)
	}

	// Reset permissions for seed users only (don't touch main accounts created by SetupGlobalDB)
	for _, p := range seedPerms {
		var id int
		tx.QueryRow("SELECT id FROM global_users WHERE username=?", p.Username).Scan(&id)
		tx.Exec("DELETE FROM lab_permissions WHERE user_id = ?", id)
		tx.Exec(
			"INSERT INTO lab_permissions (user_id, lab_url_path, role) VALUES (?, ?, ?)",
			id, p.LabPath, p.Role,
		)
	}

	// Restore main accounts if their username was changed (e.g., lab-kom-mi → lab-kom-mi-changed)
	for _, lab := range sharedEnv.Config.Labs {
		var count int
		tx.QueryRow("SELECT COUNT(*) FROM global_users WHERE username = ?", lab.URLPath).Scan(&count)
		if count == 0 {
			changedUsername := lab.URLPath + "-changed"
			tx.Exec(
				"UPDATE global_users SET username = ?, is_super_admin = 1 WHERE username = ?",
				lab.URLPath, changedUsername,
			)
		}
	}

	if err := tx.Commit(); err != nil {
		panic("resetGlobalState: commit tx: " + err.Error())
	}
}

func wrapSharedEnv(t *testing.T) *TestEnvironment {
	t.Helper()
	resetGlobalState()
	noRedirect := func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	client := &http.Client{CheckRedirect: noRedirect}
	labA := &testLab{
		url: sharedEnv.LabA.url, id: sharedEnv.LabA.id, prefix: sharedEnv.LabA.prefix,
		db: sharedEnv.DB_A, cfg: sharedEnv.Config.Labs[0],
		cookies: make(map[string]string),
		ts:      sharedEnv.TS, t: t, client: client,
	}
	labB := &testLab{
		url: sharedEnv.LabB.url, id: sharedEnv.LabB.id, prefix: sharedEnv.LabB.prefix,
		db: sharedEnv.DB_B, cfg: sharedEnv.Config.Labs[1],
		cookies: make(map[string]string),
		ts:      sharedEnv.TS, t: t, client: client,
	}
	return &TestEnvironment{
		LabA:          labA,
		LabB:          labB,
		TS:            sharedEnv.TS,
		Client:        client,
		GlobalDB:      sharedEnv.GlobalDB,
		DB_A:          sharedEnv.DB_A,
		DB_B:          sharedEnv.DB_B,
		Config:        sharedEnv.Config,
		FlushLogs:     sharedEnv.FlushLogs,
		GlobalHandler: sharedEnv.GlobalHandler,
	}
}

func setupTestEnvironment(t *testing.T, overrides ...TestConfigOverrides) *TestEnvironment {
	t.Helper()

	cfgOverride := TestConfigOverrides{}
	if len(overrides) > 0 {
		cfgOverride = overrides[0]
	}

	tmpDir, err := os.MkdirTemp("", "simlabkom-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	dbPathA := filepath.Join(tmpDir, "testing_a.db")
	dbPathB := filepath.Join(tmpDir, "testing_b.db")
	globalDBPath := filepath.Join(tmpDir, "testing_global.db")

	labAURL := "lab-kom-mi"
	labAID := "MI-1"
	labBURL := "vokasi"
	labBID := "VOKASI-1"

	cfg := createTestConfig(cfgOverride)
	if cfgOverride.UploadPath == "" {
		cfg.UploadPath = filepath.Join(tmpDir, "uploads")
	}
	cfg.Labs = []config.LabConfig{
		{ID: labAID, Title: "Lab Kom MI", URLPath: labAURL, DBPath: dbPathA, UploadDir: filepath.Join(cfg.UploadPath, labAURL), Layout: config.GridLayout{ColsPerRow: []int{8, 8, 8, 8, 8}}},
		{ID: labBID, Title: "Vokasi", URLPath: labBURL, DBPath: dbPathB, UploadDir: filepath.Join(cfg.UploadPath, labBURL), Layout: config.GridLayout{ColsPerRow: []int{10, 8, 9, 9}, HasGap: true, GapPos: 4}},
	}

	dbA, err := database.InitDB(dbPathA, "")
	if err != nil {
		t.Fatalf("InitDB lab A: %v", err)
	}
	if err := database.RunMigrations(dbA, false, labAID, labAURL, cfg.UploadPath, false); err != nil {
		t.Fatalf("Migrate lab A: %v", err)
	}
	if err := database.SeedDefaultUser(dbA); err != nil {
		t.Errorf("Seed user lab A: %v", err)
	}
	dbA.Exec("UPDATE users SET session_token = NULL")

	dbB, err := database.InitDB(dbPathB, "")
	if err != nil {
		t.Fatalf("InitDB lab B: %v", err)
	}
	if err := database.RunMigrations(dbB, false, labBID, labBURL, cfg.UploadPath, false); err != nil {
		t.Fatalf("Migrate lab B: %v", err)
	}
	if err := database.SeedDefaultUser(dbB); err != nil {
		t.Errorf("Seed user lab B: %v", err)
	}
	dbB.Exec("UPDATE users SET session_token = NULL")

	globalDB, err := database.InitDB(globalDBPath, "")
	if err != nil {
		t.Fatalf("InitDB global: %v", err)
	}
	if err := database.SetupGlobalDB(globalDB, cfg.Labs); err != nil {
		t.Fatalf("Setup global DB: %v", err)
	}
	globalDB.Exec("UPDATE global_users SET session_token = ''")

	seedGlobalUsers(globalDB)
	dbs := map[string]*database.DB{labAURL: dbA, labBURL: dbB}
	router, cleanup, flushLogs, globalHandler := server.SetupRouter(dbs, globalDB, cfg, services.DummyNotifier{})
	t.Cleanup(func() {
		cleanup()
		// Close dynamically-added lab DBs
		for urlPath, db := range globalHandler.LabsDB {
			if _, exists := dbs[urlPath]; !exists {
				db.Close()
			}
		}
		if _, exists := globalHandler.LabsDB[labAURL]; exists {
			dbA.Close()
		}
		if _, exists := globalHandler.LabsDB[labBURL]; exists {
			dbB.Close()
		}
		globalDB.Close()
		_ = os.RemoveAll(tmpDir)
	})

	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	noRedirect := func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	client := &http.Client{CheckRedirect: noRedirect}

	labA := &testLab{
		url: labAURL, id: labAID, prefix: "/" + labAURL,
		db: dbA, cfg: cfg.Labs[0], cookies: make(map[string]string),
		ts: ts, t: t, client: client,
	}
	labB := &testLab{
		url: labBURL, id: labBID, prefix: "/" + labBURL,
		db: dbB, cfg: cfg.Labs[1], cookies: make(map[string]string),
		ts: ts, t: t, client: client,
	}

	return &TestEnvironment{
		LabA: labA, LabB: labB,
		TS: ts, Client: client,
		GlobalDB: globalDB, DB_A: dbA, DB_B: dbB,
		Config: cfg, FlushLogs: flushLogs,
		GlobalHandler: globalHandler,
	}
}
