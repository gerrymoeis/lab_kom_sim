package tests

import (
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

// TestConfigOverrides allows customising config for specific test scenarios.
type TestConfigOverrides struct {
	Android       bool
	GeminiKey     string
	OpenRouterKey string
	UploadPath    string
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
	token := l.extractCSRFToken(string(body))
	if token == "" {
		return false
	}
	l.csrf = token
	return true
}

type TestEnvironment struct {
	LabA, LabB   *testLab
	TS           *httptest.Server
	Client       *http.Client
	GlobalDB, DB_A, DB_B *database.DB
	Config       *config.Config
	FlushLogs    func()
	GlobalHandler *handlers.GlobalHandler
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
	return &config.Config{
		SessionSecret:    "test-secret-12345",
		UploadPath:       uploadPath,
		DefaultPageSize:  25,
		Android:          cfg.Android,
		GeminiAPIKey:     geminiKey,
		OpenRouterAPIKey: openRouterKey,
	}
}

func setupTestEnvironment(t *testing.T, overrides ...TestConfigOverrides) *TestEnvironment {
	t.Helper()

	cfgOverride := TestConfigOverrides{}
	if len(overrides) > 0 {
		cfgOverride = overrides[0]
	}

	wd, _ := os.Getwd()
	projectRoot := findProjectRoot(wd)
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("Chdir to project root %s: %v", projectRoot, err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	godotenv.Load()

	tmpDir, err := os.MkdirTemp("", "simlabkom-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	dbPathA := filepath.Join(tmpDir, "testing_a.db")
	dbPathB := filepath.Join(tmpDir, "testing_b.db")
	globalDBPath := filepath.Join(tmpDir, "testing_global.db")

	os.RemoveAll(filepath.Join(projectRoot, "uploads", "temp"))
	os.RemoveAll(filepath.Join(projectRoot, "uploads", "pc"))
	os.RemoveAll(filepath.Join(projectRoot, "uploads", "logbook"))

	for _, labURL := range []string{"lab-kom-mi", "vokasi"} {
		os.RemoveAll(filepath.Join(projectRoot, "uploads", labURL))
	}

	labAURL := "lab-kom-mi"
	labAID := "MI-1"
	labBURL := "vokasi"
	labBID := "VOKASI-1"

	cfg := createTestConfig(cfgOverride)
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

	bcryptHash := func(pw string) string {
		h, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
		return string(h)
	}
	globalDB.Exec("INSERT OR IGNORE INTO global_users (username, password, full_name, is_super_admin) VALUES (?, ?, ?, 1)", "admin", bcryptHash("admin123"), "Administrator")
	globalDB.Exec("INSERT OR IGNORE INTO global_users (username, password, full_name, is_super_admin) VALUES (?, ?, ?, 1)", "rekan", bcryptHash("rekan123"), "Rekan Administrator")
	globalDB.Exec("INSERT OR IGNORE INTO global_users (username, password, full_name, is_super_admin) VALUES (?, ?, ?, 0)", "labA_only", bcryptHash("test123"), "Lab A Only")
	globalDB.Exec("INSERT OR IGNORE INTO global_users (username, password, full_name, is_super_admin) VALUES (?, ?, ?, 0)", "labB_only", bcryptHash("test123"), "Lab B Only")
	globalDB.Exec("INSERT OR IGNORE INTO global_users (username, password, full_name, is_super_admin) VALUES (?, ?, ?, 0)", "no_perm_user", bcryptHash("test123"), "No Permission")
	globalDB.Exec("INSERT OR IGNORE INTO global_users (username, password, full_name, is_super_admin) VALUES (?, ?, ?, 0)", "labA_dosen", bcryptHash("test123"), "Lab A Dosen")
	var labAOnlyID, labBOnlyID, labADosenID int
	globalDB.QueryRow("SELECT id FROM global_users WHERE username='labA_only'").Scan(&labAOnlyID)
	globalDB.QueryRow("SELECT id FROM global_users WHERE username='labB_only'").Scan(&labBOnlyID)
	globalDB.QueryRow("SELECT id FROM global_users WHERE username='labA_dosen'").Scan(&labADosenID)
	globalDB.Exec("INSERT OR IGNORE INTO lab_permissions (user_id, lab_url_path, role) VALUES (?, ?, 'admin')", labAOnlyID, labAURL)
	globalDB.Exec("INSERT OR IGNORE INTO lab_permissions (user_id, lab_url_path, role) VALUES (?, ?, 'admin')", labBOnlyID, labBURL)
	globalDB.Exec("INSERT OR IGNORE INTO lab_permissions (user_id, lab_url_path, role) VALUES (?, ?, 'admin')", labADosenID, labAURL)
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
		os.RemoveAll(filepath.Join(projectRoot, "uploads", "temp"))
		os.RemoveAll(filepath.Join(projectRoot, "uploads", "pc"))
		os.RemoveAll(filepath.Join(projectRoot, "uploads", "logbook"))
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
