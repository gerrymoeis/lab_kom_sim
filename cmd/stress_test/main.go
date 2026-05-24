package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var tableRE = regexp.MustCompile(`href="/(?:logbook|schedules|software|devices|device-types|device-loans|device-usages|lost-items|admin/users)/(\d+)(?:/edit|/delete|")`)
var deviceIDRE = regexp.MustCompile(`href="/devices/(\d+)"`)

type config struct {
	url          string
	totalReqs    int
	workers      int
	mode         string
	readPct      int
	rampUp       time.Duration
	setupUsers   int
	verbose      bool
	deviceIDs    []int  // Changed from single deviceID to slice
}

type entityStore struct {
	mu      sync.Mutex
	base    int
	created int64
	deleted map[int]bool
}

func newEntityStore() *entityStore {
	return &entityStore{deleted: make(map[int]bool)}
}

func (s *entityStore) trackMax(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id > s.base {
		s.base = id
	}
}

func (s *entityStore) nextCreateID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.created++
	return s.base + int(s.created)
}

func (s *entityStore) pickEditID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.created == 0 {
		return s.base
	}
	offset := rand.Intn(int(s.created))
	return s.base + offset + 1
}

type worker struct {
	id      int
	client  *http.Client
	cfg     *config
	results chan<- result
}

type result struct {
	entity     string
	op         string
	statusCode int
	latency    time.Duration
}

func newWorker(id int, cfg *config, results chan<- result) *worker {
	jar, _ := cookiejar.New(nil)
	return &worker{
		id: id,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				DisableCompression:  true,
			},
		},
		cfg:     cfg,
		results: results,
	}
}

func (w *worker) login(username, password string) error {
	v := url.Values{}
	v.Set("username", username)
	v.Set("password", password)
	req, _ := http.NewRequest("POST", w.cfg.url+"/login", strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 302 {
		return fmt.Errorf("login returned %d", resp.StatusCode)
	}
	return nil
}

func (w *worker) do(method, path, body string) (*http.Response, error) {
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, w.cfg.url+path, reqBody)
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return w.client.Do(req)
}

func pk(format string, a ...any) string { return fmt.Sprintf(format, a...) }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func parseFlags() *config {
	c := &config{}
	flag.StringVar(&c.url, "url", "http://localhost:8080", "Target server URL")
	flag.IntVar(&c.totalReqs, "total-requests", 10000, "Total HTTP requests to send")
	flag.IntVar(&c.workers, "workers", 10, "Number of concurrent workers")
	flag.StringVar(&c.mode, "mode", "mix", "Test mode: read, write, mix")
	flag.IntVar(&c.readPct, "read-pct", 50, "Read percentage in mix mode")
	flag.DurationVar(&c.rampUp, "ramp-up", 5*time.Second, "Ramp-up duration")
	flag.IntVar(&c.setupUsers, "setup-users", 0, "Create N stress test users (default: workers)")
	flag.BoolVar(&c.verbose, "verbose", false, "Log each request")
	flag.Parse()
	return c
}

func bodyLogbookCreate(c int64, workers int) string {
	v := url.Values{}
	
	// Date variation: workers × 3 days (default: 30 days)
	// Range: 2026-05-01 to 2026-05-30
	dayVariations := workers * 3
	day := 1 + int(c%int64(dayVariations))
	v.Set("date", pk("2026-05-%02d", day))
	
	// Time variation: 48 combinations (12 hours × 4 minutes)
	hour := 6 + int(c%12)
	minute := int(c%4) * 15  // 0, 15, 30, 45
	v.Set("time_in", pk("%02d:%02d", hour, minute))
	
	// Time out: +2 hours from time_in
	outHour := hour + 2
	v.Set("time_out", pk("%02d:%02d", outHour, minute))
	
	// Unique student name
	v.Set("student_name", pk("STRESS_%d", c))
	
	// Unique NIM: 13 digits
	v.Set("nim", pk("%013d", c))
	
	// Purpose variation
	purposes := []string{
		"Praktikum", "Tugas", "Belajar", "Diskusi", 
		"Mengerjakan Tugas", "Browsing", "Coding", "Research",
	}
	v.Set("purpose", purposes[int(c)%len(purposes)])
	
	return v.Encode()
}

func bodyLogbookEdit(c int64) string {
	v := url.Values{}
	v.Set("date", "2026-05-23")
	v.Set("student_name", pk("STRESS_EDIT_%d", c))
	v.Set("nim", pk("STRESS%08dE", c))
	v.Set("time_in", "09:00")
	v.Set("time_out", "11:00")
	v.Set("purpose", pk("Stress Test Edit %d", c))
	return v.Encode()
}

func bodyScheduleCreate(c int64) string {
	v := url.Values{}
	v.Set("course_name", pk("STRESS Course %d", c))
	v.Set("lecturer", "Stress Lecturer")
	v.Set("day", "Senin")
	v.Set("class", "A")
	v.Set("time_start", "08:00")
	v.Set("time_end", "10:00")
	v.Set("notes", pk("Stress test schedule %d", c))
	return v.Encode()
}

func bodyScheduleEdit(c int64) string {
	v := url.Values{}
	v.Set("course_name", pk("STRESS Course Edit %d", c))
	v.Set("lecturer", "Stress Lecturer Edit")
	v.Set("day", "Selasa")
	v.Set("class", "B")
	v.Set("time_start", "10:00")
	v.Set("time_end", "12:00")
	return v.Encode()
}

func bodySoftwareCreate(c int64) string {
	v := url.Values{}
	v.Set("name", pk("STRESS Software %d", c))
	v.Set("category", "utility")
	v.Set("description", pk("Stress test software %d", c))
	return v.Encode()
}

func bodySoftwareEdit(c int64) string {
	v := url.Values{}
	v.Set("name", pk("STRESS Software Edit %d", c))
	v.Set("category", "development")
	v.Set("description", pk("Stress test software edited %d", c))
	return v.Encode()
}

func bodyPCEdit(c int64) string {
	v := url.Values{}
	v.Set("serial_number", pk("STRSN%08d", c))
	v.Set("operating_system", "Windows 11")
	v.Set("status", "normal")
	v.Set("device_type", "PC All-in-one")
	v.Set("brand_model", "Stress Brand Model")
	return v.Encode()
}

func bodyPCStatus(c int64) string {
	v := url.Values{}
	v.Set("status", "normal")
	return v.Encode()
}

func bodyDeviceTypeCreate(c int64) string {
	v := url.Values{}
	v.Set("name", pk("STRESS Type %d", c))
	v.Set("category", "peripheral")
	v.Set("item_type", "consumable")
	return v.Encode()
}

func bodyDeviceTypeEdit(c int64) string {
	v := url.Values{}
	v.Set("name", pk("STRESS Type Edit %d", c))
	v.Set("category", "network")
	v.Set("item_type", "consumable")
	return v.Encode()
}

func bodyDeviceLoanCreate(c int64, deviceIDs []int) string {
	// Rotate through devices to distribute load
	deviceID := deviceIDs[int(c)%len(deviceIDs)]
	
	v := url.Values{}
	v.Set("device_id", strconv.Itoa(deviceID))
	v.Set("borrower_name", pk("STRESS Borrower %d", c))
	v.Set("loan_date", "2026-05-23")
	v.Set("quantity", "1")
	return v.Encode()
}

func bodyDeviceLoanEdit(c int64) string {
	v := url.Values{}
	v.Set("borrower_name", pk("STRESS Borrower Edit %d", c))
	v.Set("loan_date", "2026-05-23")
	v.Set("expected_return_date", "2026-05-30")
	v.Set("status", "active")
	return v.Encode()
}

func bodyDeviceUsageCreate(c int64, deviceIDs []int) string {
	// Rotate through devices to distribute load
	deviceID := deviceIDs[int(c)%len(deviceIDs)]
	
	v := url.Values{}
	v.Set("device_id", strconv.Itoa(deviceID))
	v.Set("user_name", pk("STRESS User %d", c))
	v.Set("usage_date", "2026-05-23")
	v.Set("quantity", "1")
	return v.Encode()
}

func bodyDeviceUsageEdit(c int64) string {
	v := url.Values{}
	v.Set("user_name", pk("STRESS User Edit %d", c))
	v.Set("usage_date", "2026-05-23")
	v.Set("quantity", "1")
	return v.Encode()
}

func bodyLostItemCreate(c int64) string {
	v := url.Values{}
	v.Set("item_name", pk("STRESS Lost Item %d", c))
	v.Set("reported_by", pk("STRESS Reporter %d", c))
	v.Set("reported_date", "2026-05-23")
	v.Set("status", "hilang")
	return v.Encode()
}

func bodyLostItemEdit(c int64) string {
	v := url.Values{}
	v.Set("item_name", pk("STRESS Lost Item Edit %d", c))
	v.Set("reported_by", pk("STRESS Reporter Edit %d", c))
	v.Set("reported_date", "2026-05-23")
	v.Set("status", "ditemukan")
	return v.Encode()
}

func bodyProfileUpdate(c int64) string {
	v := url.Values{}
	v.Set("username", pk("stress_user_%d", c))
	v.Set("full_name", pk("Stress Tester %d", c))
	return v.Encode()
}

func bodyPasswordChange() string {
	v := url.Values{}
	v.Set("old_password", "Stress123!")
	v.Set("new_password", "Stress123!")
	v.Set("confirm_password", "Stress123!")
	return v.Encode()
}

type endpointDef struct {
	method string
	path   string
	body   string
	entity string
	op     string
}

func (w *worker) pickEndpoint(counter int64, stores map[string]*entityStore) endpointDef {
	cfg := w.cfg
	readEndpoints := []endpointDef{
		{"GET", "/dashboard", "", "dashboard", "read"},
		{"GET", "/logbook?page=1&size=50", "", "logbook", "read"},
		{"GET", "/pc?page=1&size=50", "", "pc", "read"},
		{"GET", "/schedules", "", "schedules", "read"},
		{"GET", "/software", "", "software", "read"},
		{"GET", "/devices?tab=list", "", "devices", "read"},
		{"GET", "/device-types", "", "device-types", "read"},
		{"GET", "/devices?tab=loans", "", "device-loans", "read"},
		{"GET", "/devices?tab=usages", "", "device-usages", "read"},
		{"GET", "/lost-items", "", "lost-items", "read"},
		{"GET", "/admin/users", "", "users", "read"},
		{"GET", "/admin/activity-logs", "", "activity-logs", "read"},
		{"GET", "/profile", "", "profile", "read"},
	}

	createEndpoints := []endpointDef{
		{"POST", "/logbook/create", bodyLogbookCreate(counter, cfg.workers), "logbook", "create"},
		{"POST", "/schedules/create", bodyScheduleCreate(counter), "schedules", "create"},
		{"POST", "/software/create", bodySoftwareCreate(counter), "software", "create"},
		{"POST", "/device-types/create", bodyDeviceTypeCreate(counter), "device-types", "create"},
		{"POST", "/device-loans/create", bodyDeviceLoanCreate(counter, cfg.deviceIDs), "device-loans", "create"},
		{"POST", "/device-usages/create", bodyDeviceUsageCreate(counter, cfg.deviceIDs), "device-usages", "create"},
		{"POST", "/lost-items/create", bodyLostItemCreate(counter), "lost-items", "create"},
		{"POST", pk("/pc/%d/edit", int(counter%40)+1), bodyPCEdit(counter), "pc", "update"},
		{"POST", pk("/api/pc/%d/status", int(counter%40)+1), bodyPCStatus(counter), "pc", "create"},
	}

	updateEndpoints := []endpointDef{
		{"POST", pk("/pc/%d/edit", int(counter%40)+1), bodyPCEdit(counter), "pc", "update"},
		{"POST", pk("/api/pc/%d/status", int(counter%40)+1), bodyPCStatus(counter), "pc", "update"},
		{"POST", "/profile", bodyProfileUpdate(counter), "profile", "update"},
		{"POST", "/profile/password", bodyPasswordChange(), "profile", "update"},
	}

	if id := stores["logbook"].pickEditID(); id > 0 {
		updateEndpoints = append(updateEndpoints,
			endpointDef{"POST", pk("/logbook/%d/edit", id), bodyLogbookEdit(counter), "logbook", "update"})
	}
	if id := stores["schedules"].pickEditID(); id > 0 {
		updateEndpoints = append(updateEndpoints,
			endpointDef{"POST", pk("/schedules/%d/edit", id), bodyScheduleEdit(counter), "schedules", "update"})
	}
	if id := stores["software"].pickEditID(); id > 0 {
		updateEndpoints = append(updateEndpoints,
			endpointDef{"POST", pk("/software/%d/edit", id), bodySoftwareEdit(counter), "software", "update"})
	}
	if id := stores["device-types"].pickEditID(); id > 0 {
		updateEndpoints = append(updateEndpoints,
			endpointDef{"POST", pk("/device-types/%d/edit", id), bodyDeviceTypeEdit(counter), "device-types", "update"})
	}
	if id := stores["device-loans"].pickEditID(); id > 0 {
		updateEndpoints = append(updateEndpoints,
			endpointDef{"POST", pk("/device-loans/%d/edit", id), bodyDeviceLoanEdit(counter), "device-loans", "update"})
	}
	if id := stores["device-usages"].pickEditID(); id > 0 {
		updateEndpoints = append(updateEndpoints,
			endpointDef{"POST", pk("/device-usages/%d/edit", id), bodyDeviceUsageEdit(counter), "device-usages", "update"})
	}
	if id := stores["lost-items"].pickEditID(); id > 0 {
		updateEndpoints = append(updateEndpoints,
			endpointDef{"POST", pk("/lost-items/%d/edit", id), bodyLostItemEdit(counter), "lost-items", "update"})
	}

	deleteEndpoints := []endpointDef{}
	if id := stores["logbook"].pickEditID(); id > 0 {
		deleteEndpoints = append(deleteEndpoints,
			endpointDef{"POST", pk("/logbook/%d/delete", id), "", "logbook", "delete"})
	}
	if id := stores["schedules"].pickEditID(); id > 0 {
		deleteEndpoints = append(deleteEndpoints,
			endpointDef{"POST", pk("/schedules/%d/delete", id), "", "schedules", "delete"})
	}
	if id := stores["software"].pickEditID(); id > 0 {
		deleteEndpoints = append(deleteEndpoints,
			endpointDef{"POST", pk("/software/%d/delete", id), "", "software", "delete"})
	}
	if id := stores["device-types"].pickEditID(); id > 0 {
		deleteEndpoints = append(deleteEndpoints,
			endpointDef{"POST", pk("/device-types/%d/delete", id), "", "device-types", "delete"})
	}
	if id := stores["device-loans"].pickEditID(); id > 0 {
		deleteEndpoints = append(deleteEndpoints,
			endpointDef{"POST", pk("/device-loans/%d/delete", id), "", "device-loans", "delete"})
	}
	if id := stores["device-usages"].pickEditID(); id > 0 {
		deleteEndpoints = append(deleteEndpoints,
			endpointDef{"POST", pk("/device-usages/%d/delete", id), "", "device-usages", "delete"})
	}
	if id := stores["lost-items"].pickEditID(); id > 0 {
		deleteEndpoints = append(deleteEndpoints,
			endpointDef{"POST", pk("/lost-items/%d/delete", id), "", "lost-items", "delete"})
	}
	deleteEndpoints = append(deleteEndpoints,
		endpointDef{"POST", pk("/pc/%d/delete", int(counter%40)+1), "", "pc", "delete"})

	var candidates []endpointDef
	switch cfg.mode {
	case "read":
		candidates = readEndpoints
	case "write":
		candidates = append(append(createEndpoints, updateEndpoints...), deleteEndpoints...)
	default:
		if rand.Intn(100) < cfg.readPct {
			candidates = readEndpoints
		} else {
			r := rand.Intn(100)
			switch {
			case r < 50:
				candidates = createEndpoints
			case r < 80:
				candidates = updateEndpoints
			default:
				candidates = deleteEndpoints
			}
		}
	}
	return candidates[rand.Intn(len(candidates))]
}

func discoverIDs(url string, client *http.Client) map[string]*entityStore {
	stores := map[string]*entityStore{
		"logbook":       newEntityStore(),
		"schedules":     newEntityStore(),
		"software":      newEntityStore(),
		"devices":       newEntityStore(),
		"device-types":  newEntityStore(),
		"device-loans":  newEntityStore(),
		"device-usages": newEntityStore(),
		"lost-items":    newEntityStore(),
		"users":         newEntityStore(),
	}

	patterns := []struct {
		entity  string
		path    string
		pattern string
	}{
		{"logbook", "/logbook?page=1&size=500", `href="/logbook/(\d+)/edit`},
		{"schedules", "/schedules", `href="/schedules/(\d+)/edit`},
		{"software", "/software", `href="/software/(\d+)/edit|href="/software/(\d+)"`},
		{"devices", "/devices?tab=list", `href="/devices/(\d+)(?:/edit|")`},
		{"device-types", "/device-types", `href="/device-types/(\d+)/edit`},
		{"device-loans", "/devices?tab=loans", `(?s)href="/device-loans/(\d+)/edit`},
		{"device-usages", "/devices?tab=usages", `(?s)href="/device-usages/(\d+)/edit`},
		{"lost-items", "/lost-items", `href="/lost-items/(\d+)/edit`},
		{"users", "/admin/users", `href="/admin/users/(\d+)/delete`},
	}

	for _, p := range patterns {
		if p.path == "" {
			continue
		}
		req, _ := http.NewRequest("GET", url+p.path, nil)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		re := regexp.MustCompile(p.pattern)
		matches := re.FindAllStringSubmatch(string(body), -1)
		seen := map[int]bool{}
		for _, m := range matches {
			for _, g := range m[1:] {
				if g == "" {
					continue
				}
				if id, err := strconv.Atoi(g); err == nil && !seen[id] {
					stores[p.entity].trackMax(id)
					seen[id] = true
				}
			}
		}
	}

	return stores
}

func clientLogin(client *http.Client, baseURL, username, password string) {
	v := url.Values{}
	v.Set("username", username)
	v.Set("password", password)
	req, _ := http.NewRequest("POST", baseURL+"/login", strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Login as %s failed: %v", username, err)
	}
	resp.Body.Close()
	if resp.StatusCode != 302 {
		log.Fatalf("Login as %s returned %d", username, resp.StatusCode)
	}
}

func setupStressUsers(cfg *config) (*http.Client, []int) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	v := url.Values{}
	v.Set("username", "rekan")
	v.Set("password", "rekan123")
	req, _ := http.NewRequest("POST", cfg.url+"/login", strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Setup: login as rekan failed: %v", err)
	}
	resp.Body.Close()
	log.Printf("Setup: logged in as rekan")

	existing := map[int]bool{}
	req, _ = http.NewRequest("GET", cfg.url+"/admin/users", nil)
	resp, err = client.Do(req)
	if err == nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		re := regexp.MustCompile(`stress_(\d+)`)
		for _, m := range re.FindAllStringSubmatch(string(body), -1) {
			if id, _ := strconv.Atoi(m[1]); id > 0 {
				existing[id] = true
			}
		}
	}

	created := 0
	for i := 1; i <= cfg.setupUsers; i++ {
		if existing[i] {
			continue
		}
		v := url.Values{}
		v.Set("username", pk("stress_%d", i))
		v.Set("password", "Stress123!")
		v.Set("full_name", pk("Stress Tester %d", i))
		v.Set("role", "admin")
		req, _ := http.NewRequest("POST", cfg.url+"/admin/users/create", strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Setup: failed to create user %d: %v", i, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 302 {
			created++
		}
	}
	log.Printf("Setup: created %d stress users (total needed: %d)", created, cfg.setupUsers)

	// Step 1: Get or create device_type (required for device foreign key)
	deviceTypeID := 0
	var body []byte
	
	// First, try to find existing STRESS_DEVICE_TYPE
	req, _ = http.NewRequest("GET", cfg.url+"/device-types", nil)
	resp, err = client.Do(req)
	if err == nil {
		body, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		
		// Look for STRESS_DEVICE_TYPE in the response
		if strings.Contains(string(body), "STRESS_DEVICE_TYPE") {
			// Try to extract ID near STRESS_DEVICE_TYPE
			re := regexp.MustCompile(`STRESS_DEVICE_TYPE.*?href="/device-types/(\d+)`)
			if m := re.FindStringSubmatch(string(body)); len(m) > 1 {
				deviceTypeID, _ = strconv.Atoi(m[1])
				log.Printf("Setup: found existing device type ID=%d", deviceTypeID)
			}
		}
	}
	
	// If not found, create new device type
	if deviceTypeID == 0 {
		v = url.Values{}
		v.Set("name", "STRESS_DEVICE_TYPE")
		v.Set("category", "peripheral")
		v.Set("item_type", "individual")
		req, _ = http.NewRequest("POST", cfg.url+"/device-types/create", strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err = client.Do(req)
		if err != nil {
			log.Fatalf("Setup: failed to create device type: %v", err)
		}
		body, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 302 {
			log.Fatalf("Setup: device type creation failed with status %d: %s", resp.StatusCode, string(body))
		}
		log.Printf("Setup: created device type")
		
		// Get the ID from redirect location or search again
		time.Sleep(500 * time.Millisecond)
		req, _ = http.NewRequest("GET", cfg.url+"/device-types", nil)
		resp, err = client.Do(req)
		if err != nil {
			log.Fatalf("Setup: failed to get device types: %v", err)
		}
		body, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		
		// Try multiple patterns
		patterns := []*regexp.Regexp{
			regexp.MustCompile(`STRESS_DEVICE_TYPE.*?href="/device-types/(\d+)`),
			regexp.MustCompile(`href="/device-types/(\d+)/edit"`),
			regexp.MustCompile(`href="/device-types/(\d+)"`),
		}
		
		for _, pattern := range patterns {
			matches := pattern.FindAllStringSubmatch(string(body), -1)
			if len(matches) > 0 {
				// Get the last match (most recent)
				m := matches[len(matches)-1]
				if len(m) > 1 {
					deviceTypeID, _ = strconv.Atoi(m[1])
					log.Printf("Setup: found device type ID=%d", deviceTypeID)
					break
				}
			}
		}
	}
	
	if deviceTypeID == 0 {
		log.Fatalf("Setup: could not find or create device type ID")
	}

	// Step 2: Create multiple devices (workers × 2)
	deviceCount := cfg.workers * 2
	quantityPerDevice := 50
	deviceIDs := []int{}
	
	log.Printf("Setup: creating %d devices with %d quantity each (total: %d)", 
		deviceCount, quantityPerDevice, deviceCount*quantityPerDevice)
	
	for i := 1; i <= deviceCount; i++ {
		v = url.Values{}
		v.Set("device_type_id", strconv.Itoa(deviceTypeID))
		v.Set("name", pk("STRESS_DEVICE_%d", i))
		v.Set("brand", "Stress Brand")
		v.Set("quantity_total", strconv.Itoa(quantityPerDevice))
		v.Set("item_type", "individual")
		v.Set("item_mode", "loanable")
		v.Set("condition", "baik")
		
		req, _ = http.NewRequest("POST", cfg.url+"/devices/create", strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err = client.Do(req)
		if err != nil {
			log.Fatalf("Setup: failed to create device %d: %v", i, err)
		}
		body, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 302 {
			log.Fatalf("Setup: device %d creation failed with status %d: %s", 
				i, resp.StatusCode, string(body))
		}
		
		// Try to get device ID from redirect Location header
		var deviceID int
		var found bool
		
		if location := resp.Header.Get("Location"); location != "" {
			// Try to extract ID from redirect URL
			// Possible formats: /devices/123, /devices?id=123
			re := regexp.MustCompile(`/devices/(\d+)`)
			if m := re.FindStringSubmatch(location); len(m) > 1 {
				deviceID, _ = strconv.Atoi(m[1])
				deviceIDs = append(deviceIDs, deviceID)
				found = true
				log.Printf("Setup: device %d created with ID=%d (from redirect)", i, deviceID)
			}
		}
		
		// Fallback: Search for device by name with EXACT match
		if !found {
			deviceName := pk("STRESS_DEVICE_%d", i)
			
			for retry := 0; retry < 5; retry++ {
				time.Sleep(500 * time.Millisecond)  // Increased delay for DB commit
				
				// Search ALL devices (no filter) to ensure we get the new device
				req, _ = http.NewRequest("GET", cfg.url+"/devices?tab=list", nil)
				resp, err = client.Do(req)
				if err != nil {
					log.Printf("Setup: retry %d - failed to get devices list: %v", retry+1, err)
					continue
				}
				body, _ = io.ReadAll(resp.Body)
				resp.Body.Close()
				
				// Strategy: Find exact device name in HTML, then extract ID from nearby href
				// Look for pattern: >STRESS_DEVICE_1< ... href="/devices/123"
				
				// Pattern 1: Exact name match with ID in same row
				pattern1 := regexp.MustCompile(`>` + deviceName + `<.*?href="/devices/(\d+)`)
				if m := pattern1.FindStringSubmatch(string(body)); len(m) > 1 {
					deviceID, _ = strconv.Atoi(m[1])
					deviceIDs = append(deviceIDs, deviceID)
					found = true
					log.Printf("Setup: device %d found with ID=%d (exact name match)", i, deviceID)
					break
				}
				
				// Pattern 2: href before name (reverse order)
				pattern2 := regexp.MustCompile(`href="/devices/(\d+)[^>]*>.*?>` + deviceName + `<`)
				if m := pattern2.FindStringSubmatch(string(body)); len(m) > 1 {
					deviceID, _ = strconv.Atoi(m[1])
					deviceIDs = append(deviceIDs, deviceID)
					found = true
					log.Printf("Setup: device %d found with ID=%d (reverse pattern)", i, deviceID)
					break
				}
				
				// Pattern 3: Look for device name in table cell, then find ID in same row
				// <tr>...<td>STRESS_DEVICE_1</td>...<a href="/devices/123/edit">
				pattern3 := regexp.MustCompile(`<tr[^>]*>.*?` + deviceName + `.*?href="/devices/(\d+)`)
				if m := pattern3.FindStringSubmatch(string(body)); len(m) > 1 {
					deviceID, _ = strconv.Atoi(m[1])
					deviceIDs = append(deviceIDs, deviceID)
					found = true
					log.Printf("Setup: device %d found with ID=%d (table row pattern)", i, deviceID)
					break
				}
				
				// Debug on last retry
				if retry == 4 {
					log.Printf("Setup: device %d (%s) not found after 5 retries", i, deviceName)
					log.Printf("Setup: Response length: %d bytes", len(body))
					
					// Check if device name exists in response
					if strings.Contains(string(body), deviceName) {
						log.Printf("Setup: Device name '%s' FOUND in response but ID extraction failed", deviceName)
						// Print context around device name
						idx := strings.Index(string(body), deviceName)
						start := max(0, idx-200)
						end := min(len(body), idx+200)
						log.Printf("Setup: Context around device name:\n%s", string(body)[start:end])
					} else {
						log.Printf("Setup: Device name '%s' NOT FOUND in response", deviceName)
						// Print sample of device list to see structure
						log.Printf("Setup: Response preview (first 1000 chars):\n%s", string(body)[:min(1000, len(body))])
					}
				}
			}
		}
		
		if !found {
			log.Fatalf("Setup: could not find device %d ID after 5 retries", i)
		}
		
		if i%5 == 0 || i == deviceCount {
			log.Printf("Setup: created %d/%d devices", i, deviceCount)
		}
	}
	
	log.Printf("Setup: successfully created %d devices (IDs: %d-%d)", 
		len(deviceIDs), deviceIDs[0], deviceIDs[len(deviceIDs)-1])
	
	// Debug: print all device IDs
	log.Printf("Setup: device IDs detail: %v", deviceIDs)

	// Don't logout - keep session for discovery phase
	log.Printf("Setup: keeping rekan session for discovery")

	return client, deviceIDs
}

func runWorkers(cfg *config, stores map[string]*entityStore) []result {
	results := make(chan result, cfg.totalReqs)
	workers := make([]*worker, cfg.workers)
	var wg sync.WaitGroup

	for i := 0; i < cfg.workers; i++ {
		w := newWorker(i+1, cfg, results)
		username := pk("stress_%d", (i % cfg.setupUsers) + 1)
		var loggedIn bool
		for retry := 0; retry < 10; retry++ {
			if err := w.login(username, "Stress123!"); err == nil {
				loggedIn = true
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		if !loggedIn {
			log.Fatalf("Worker %d login as %s failed after retries", i+1, username)
		}
		if cfg.verbose {
			log.Printf("Worker %d: logged in as %s", i+1, username)
		}
		workers[i] = w
	}
	log.Printf("All %d workers logged in", cfg.workers)

	var sent int64

	for i := 0; i < cfg.workers; i++ {
		wg.Add(1)
		go func(w *worker) {
			defer wg.Done()
			for {
				n := atomic.AddInt64(&sent, 1)
				if int(n) > cfg.totalReqs {
					return
				}
				ep := w.pickEndpoint(n, stores)
				start := time.Now()
				resp, err := w.do(ep.method, ep.path, ep.body)
				latency := time.Since(start)

				r := result{entity: ep.entity, op: ep.op, latency: latency}
				if err != nil {
					r.statusCode = 0
				} else {
					r.statusCode = resp.StatusCode
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}

				if cfg.verbose && n <= 20 {
					log.Printf("[%d] %s %s → %d (%v)", n, ep.method, ep.path, r.statusCode, latency)
				}
				
				// Log CREATE failures for debugging
				if ep.op == "create" && (r.statusCode < 200 || r.statusCode >= 400) && n <= 50 {
					debugInfo := ""
					if ep.entity == "device-loans" || ep.entity == "device-usages" {
						// Extract device_id from body
						if strings.Contains(ep.body, "device_id=") {
							parts := strings.Split(ep.body, "&")
							for _, part := range parts {
								if strings.HasPrefix(part, "device_id=") {
									debugInfo = fmt.Sprintf(" [%s]", part)
									break
								}
							}
						}
					}
					log.Printf("[%d] CREATE FAILED: %s %s\nBody: %s%s\n→ Status: %d (%v)", 
						n, ep.method, ep.path, ep.body, debugInfo, r.statusCode, latency)
				}
				
				results <- r
			}
		}(workers[i])
	}

	wg.Wait()
	close(results)

	out := make([]result, 0, cfg.totalReqs)
	for r := range results {
		out = append(out, r)
	}
	return out
}

func printReport(cfg *config, results []result, testDuration time.Duration) {
	if len(results) == 0 {
		fmt.Println("No results collected")
		return
	}

	durations := make([]time.Duration, len(results))
	total2xx, total3xx, total4xx, total5xx, total0 := 0, 0, 0, 0, 0
	countByOp := map[string]int{}
	okByOp := map[string]int{}
	countByEntity := map[string]int{}
	okByEntity := map[string]int{}
	latByOp := map[string][]time.Duration{}
	latByEntity := map[string][]time.Duration{}
	opEntityOK := map[string]map[string]int{}
	opEntityTotal := map[string]map[string]int{}

	for i, r := range results {
		durations[i] = r.latency
		countByOp[r.op]++
		countByEntity[r.entity]++
		latByOp[r.op] = append(latByOp[r.op], r.latency)
		latByEntity[r.entity] = append(latByEntity[r.entity], r.latency)
		if opEntityOK[r.op] == nil {
			opEntityOK[r.op] = map[string]int{}
			opEntityTotal[r.op] = map[string]int{}
		}
		opEntityTotal[r.op][r.entity]++

		switch {
		case r.statusCode >= 200 && r.statusCode < 300:
			total2xx++
			okByOp[r.op]++
			okByEntity[r.entity]++
			opEntityOK[r.op][r.entity]++
		case r.statusCode >= 300 && r.statusCode < 400:
			total3xx++
			okByOp[r.op]++
			okByEntity[r.entity]++
			opEntityOK[r.op][r.entity]++
		case r.statusCode >= 400 && r.statusCode < 500:
			total4xx++
		case r.statusCode >= 500:
			total5xx++
		default:
			total0++
		}
	}

	duration := testDuration.Round(time.Millisecond)

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

	p := func(pct int) time.Duration {
		if len(durations) == 0 {
			return 0
		}
		idx := int(float64(len(durations)) * float64(pct) / 100.0)
		if idx >= len(durations) {
			idx = len(durations) - 1
		}
		return durations[idx]
	}

	throughput := float64(len(results)) / duration.Seconds()

	fmt.Println()
	fmt.Println("=== STRESS TEST REPORT ===")
	fmt.Printf("Target:       %s\n", cfg.url)
	fmt.Printf("Total:        %d requests\n", len(results))
	fmt.Printf("Workers:      %d\n", cfg.workers)
	fmt.Printf("Mode:         %s (read %d%%)\n", cfg.mode, cfg.readPct)
	fmt.Printf("Duration:     %v\n", duration)
	fmt.Printf("Throughput:   %.0f req/s\n", throughput)
	fmt.Println()

	fmt.Println("BY STATUS CODE:")
	fmt.Printf("  2xx: %d (%.1f%%)\n", total2xx, float64(total2xx)/float64(len(results))*100)
	fmt.Printf("  3xx: %d (%.1f%%)\n", total3xx, float64(total3xx)/float64(len(results))*100)
	fmt.Printf("  4xx: %d (%.1f%%)\n", total4xx, float64(total4xx)/float64(len(results))*100)
	fmt.Printf("  5xx: %d (%.1f%%)\n", total5xx, float64(total5xx)/float64(len(results))*100)
	fmt.Println()

	fmt.Println("BY OPERATION:")
	ops := []string{"create", "read", "update", "delete"}
	for _, op := range ops {
		t := countByOp[op]
		if t == 0 {
			continue
		}
		ok := okByOp[op]
		var p50, p95, p99 time.Duration
		if lats := latByOp[op]; len(lats) > 0 {
			sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
			p50 = percentile(lats, 50)
			p95 = percentile(lats, 95)
			p99 = percentile(lats, 99)
		}
		fmt.Printf("  %-8s %5d  OK=%-5d FAIL=%-4d (%.1f%%)  p50=%-8v p95=%-8v p99=%-8v\n",
			strings.ToUpper(op[:1])+op[1:], t, ok, t-ok, float64(ok)/float64(t)*100, p50, p95, p99)
	}
	fmt.Println()

	fmt.Println("BY ENTITY:")
	entities := make([]string, 0, len(countByEntity))
	for e := range countByEntity {
		entities = append(entities, e)
	}
	sort.Strings(entities)
	for _, e := range entities {
		t := countByEntity[e]
		ok := okByEntity[e]
		var p50, p95 time.Duration
		if lats := latByEntity[e]; len(lats) > 0 {
			sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
			p50 = percentile(lats, 50)
			p95 = percentile(lats, 95)
		}
		fmt.Printf("  %-14s %5d req  OK=%-5d FAIL=%-4d (%.1f%%)  p50=%-8v p95=%-8v\n",
			e, t, ok, t-ok, float64(ok)/float64(t)*100, p50, p95)
	}
	fmt.Println()

	fmt.Println("OVERALL LATENCY:")
	if len(durations) > 0 {
		fmt.Printf("  p50: %v   p75: %v   p90: %v   p95: %v   p99: %v   max: %v\n",
			p(50), p(75), p(90), p(95), p(99), durations[len(durations)-1])
	}
	fmt.Println()

	fmt.Println("CRUD DETAIL (per entity, by operation):")
	for _, e := range entities {
		fmt.Printf("  %s:\n", e)
		for _, op := range ops {
			ot := opEntityTotal[op][e]
			if ot == 0 {
				continue
			}
			oOK := opEntityOK[op][e]
			var lat time.Duration
			key := e + "_" + op
			if lats := latByEntity[e]; len(lats) > 0 && len(results) > 0 {
				if latsByOp := latByOp[op]; len(latsByOp) > 0 {
					sorted := make([]time.Duration, len(latsByOp))
					copy(sorted, latsByOp)
					sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
					lat = percentile(sorted, 50)
					_ = key
				}
			}
			_ = lat
			fmt.Printf("    %-8s %5d req  OK=%-4d  FAIL=%-3d\n", op, ot, oOK, ot-oOK)
		}
	}
}

func percentile(sorted []time.Duration, pct int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)) * float64(pct) / 100.0)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func main() {
	cfg := parseFlags()
	log.SetFlags(0)

	if cfg.setupUsers == 0 {
		cfg.setupUsers = cfg.workers
	}

	var discoveryClient *http.Client
	if cfg.setupUsers > 0 {
		var deviceIDs []int
		discoveryClient, deviceIDs = setupStressUsers(cfg)
		cfg.deviceIDs = deviceIDs
		
		// Validate deviceIDs
		if len(deviceIDs) == 0 {
			log.Fatalf("Setup: no devices created")
		}
		log.Printf("Setup: validated %d devices", len(deviceIDs))
		
		time.Sleep(1 * time.Second)
	} else {
		jar, _ := cookiejar.New(nil)
		discoveryClient = &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		clientLogin(discoveryClient, cfg.url, "rekan", "rekan123")
	}

	log.Printf("Discovery: reading existing data...")
	stores := discoverIDs(cfg.url, discoveryClient)

	log.Printf("Discovered IDs:")
	for name, s := range stores {
		s.mu.Lock()
		log.Printf("  %s: base=%d, created=%d", name, s.base, s.created)
		s.mu.Unlock()
	}

	testStart := time.Now()
	results := runWorkers(cfg, stores)
	printReport(cfg, results, time.Since(testStart))
}
