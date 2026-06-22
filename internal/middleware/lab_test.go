package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func setupMiddlewareTest(t *testing.T) (globalDB *database.DB, dbs map[string]*database.DB, labs map[string]config.LabConfig) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	globalDB, err := database.InitDB(t.TempDir()+"/middleware_test.db", "")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { globalDB.Close() })

	_, err = globalDB.Exec(`CREATE TABLE IF NOT EXISTS global_users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		full_name TEXT NOT NULL DEFAULT '',
		is_super_admin INTEGER NOT NULL DEFAULT 0,
		is_protected INTEGER NOT NULL DEFAULT 0,
		session_token TEXT DEFAULT '',
		password_is_default INTEGER NOT NULL DEFAULT 1,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("create global_users: %v", err)
	}
	_, err = globalDB.Exec(`CREATE TABLE IF NOT EXISTS lab_permissions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL REFERENCES global_users(id),
		lab_url_path TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'admin' CHECK(role IN ('admin')),
		is_main_account INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, lab_url_path)
	)`)
	if err != nil {
		t.Fatalf("create lab_permissions: %v", err)
	}

	// Create per-lab DB for middleware tests
	labDB, err := database.InitDB(t.TempDir()+"/lab_test.db", "")
	if err != nil {
		t.Fatalf("InitDB lab: %v", err)
	}
	t.Cleanup(func() { labDB.Close() })

	dbs = map[string]*database.DB{"test-lab": labDB}
	labs = map[string]config.LabConfig{"test-lab": {URLPath: "test-lab"}}
	return
}

func setSession(t *testing.T, r *gin.Engine, userID int, username string, isSuperAdmin bool, labs []string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/setsession", nil)
	// Create a handler that sets session values
	r.GET("/setsession", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("user_id", userID)
		s.Set("username", username)
		s.Set("is_super_admin", isSuperAdmin)
		s.Set("labs", labs)
		_ = s.Save()
		c.String(200, "ok")
	})
	r.ServeHTTP(w, req)
	return w
}

func TestLabURL(t *testing.T) {
	t.Run("with_lab", func(t *testing.T) {
		result := LabURL(createTestContext("test-lab"), "/dashboard")
		if result != "/test-lab/dashboard" {
			t.Errorf("expected /test-lab/dashboard, got %s", result)
		}
	})
}

func TestLabCookieName(t *testing.T) {
	t.Run("with_lab", func(t *testing.T) {
		name := LabCookieName("test-lab")
		if name != "inventaris_session_test-lab" {
			t.Errorf("expected inventaris_session_test-lab, got %s", name)
		}
	})
	t.Run("empty_lab", func(t *testing.T) {
		name := LabCookieName("")
		if name != "inventaris_session" {
			t.Errorf("expected inventaris_session, got %s", name)
		}
	})
}

// TestLabRoleInjector verifies role injection from lab_permissions.
func TestLabRoleInjector(t *testing.T) {
	globalDB, _, _ := setupMiddlewareTest(t)

	// Create a super admin user
	superHash := "$2a$10$dummy"
	globalDB.Exec(`INSERT INTO global_users (id, username, password, full_name, is_super_admin, is_protected, session_token)
		VALUES (1, 'superadmin', ?, 'Super Admin', 1, 0, '')`, superHash)

	t.Run("super_admin_gets_role_admin", func(t *testing.T) {
		w := httptest.NewRecorder()

		// Set session as super admin
		sessionRouter := gin.New()
		store := cookie.NewStore([]byte("test-secret"))
		sessionRouter.Use(sessions.Sessions("inventaris_session", store))

		// First request sets session
		sessionRouter.GET("/set", func(c *gin.Context) {
			s := sessions.Default(c)
			s.Set("user_id", 1)
			s.Set("username", "superadmin")
			s.Set("is_super_admin", true)
			_ = s.Save()
			c.String(200, "ok")
		})
		req, _ := http.NewRequest("GET", "/set", nil)
		sessionRouter.ServeHTTP(w, req)
		cookies := w.Result().Cookies()

		// Now test with LabRoleInjector — no per-lab DB needed for role injector
		testRouter := gin.New()
		store2 := cookie.NewStore([]byte("test-secret"))
		testRouter.Use(sessions.Sessions("inventaris_session", store2))
		testRouter.Use(GlobalDBInjector(globalDB))
		testRouter.Use(LabRoleInjector())

		testRouter.GET("/test", func(c *gin.Context) {
			role := c.GetString("role")
			if role != "admin" {
				t.Errorf("expected role 'admin' for super admin, got %q", role)
			}
			isa, _ := c.Get("is_super_admin")
			if isa != true {
				t.Error("expected is_super_admin=true")
			}
			c.String(200, role)
		})

		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("GET", "/test", nil)
		for _, c := range cookies {
			req2.AddCookie(c)
		}
		testRouter.ServeHTTP(w2, req2)
		if w2.Code != 200 {
			t.Errorf("expected 200, got %d", w2.Code)
		}
	})
}

// TestLabPermissionRequired verifies access control behavior.
func TestLabPermissionRequired(t *testing.T) {
	globalDB, dbs, _ := setupMiddlewareTest(t)

	// Create users: super admin, lab admin, no-permission user
	globalDB.Exec(`INSERT INTO global_users (id, username, password, full_name, is_super_admin, is_protected, session_token)
		VALUES (1, 'super', ?, 'Super', 1, 0, '')`, "$2a$10$dummy")
	globalDB.Exec(`INSERT INTO global_users (id, username, password, full_name, is_super_admin, is_protected, session_token)
		VALUES (2, 'labadmin', ?, 'LabAdmin', 0, 0, '')`, "$2a$10$dummy")
	globalDB.Exec(`INSERT INTO global_users (id, username, password, full_name, is_super_admin, is_protected, session_token)
		VALUES (3, 'nobody', ?, 'Nobody', 0, 0, '')`, "$2a$10$dummy")
	globalDB.Exec(`INSERT INTO lab_permissions (user_id, lab_url_path, role) VALUES (2, 'test-lab', 'admin')`)

	labLabs := map[string]config.LabConfig{"test-lab": {URLPath: "test-lab"}}

	t.Run("no_session_redirects_to_login", func(t *testing.T) {
		testRouter := gin.New()
		store := cookie.NewStore([]byte("test-secret"))
		testRouter.Use(sessions.Sessions("inventaris_session", store))
		testRouter.Use(GlobalDBInjector(globalDB))

		labGroup := testRouter.Group("/:lab")
		labGroup.Use(DBInjector(dbs, labLabs))
		labGroup.GET("/protected", func(c *gin.Context) {
			c.Set("lab", c.Param("lab"))
			c.Next()
		}, LabPermissionRequired(), func(c *gin.Context) {
			c.String(200, "ok")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test-lab/protected", nil)
		testRouter.ServeHTTP(w, req)
		// No session → redirect to /login
		if w.Code != 302 {
			t.Errorf("expected 302 redirect without session, got %d", w.Code)
		}
	})

	t.Run("super_admin_allowed", func(t *testing.T) {
		testRouter := gin.New()
		store := cookie.NewStore([]byte("test-secret"))
		testRouter.Use(sessions.Sessions("inventaris_session", store))
		testRouter.Use(GlobalDBInjector(globalDB))

		labGroup := testRouter.Group("/:lab")
		labGroup.Use(DBInjector(dbs, labLabs))

		// Register ALL routes BEFORE any ServeHTTP
		testRouter.GET("/setsess", func(c *gin.Context) {
			s := sessions.Default(c)
			s.Set("user_id", 1)
			s.Set("username", "super")
			s.Set("is_super_admin", true)
			s.Set("labs", []string{})
			_ = s.Save()
			c.String(200, "ok")
		})
		labGroup.GET("/protected", func(c *gin.Context) {
			c.Set("lab", c.Param("lab"))
			c.Next()
		}, LabPermissionRequired(), func(c *gin.Context) {
			c.String(200, "allowed")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/setsess", nil)
		testRouter.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("setsess returned %d", w.Code)
		}

		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("GET", "/test-lab/protected", nil)
		for _, ck := range w.Result().Cookies() {
			req2.AddCookie(ck)
		}
		testRouter.ServeHTTP(w2, req2)
		if w2.Code != 200 {
			t.Errorf("expected 200 for super admin, got %d", w2.Code)
		}
	})

	t.Run("user_with_permission_allowed", func(t *testing.T) {
		testRouter := gin.New()
		store := cookie.NewStore([]byte("test-secret"))
		testRouter.Use(sessions.Sessions("inventaris_session", store))
		testRouter.Use(GlobalDBInjector(globalDB))

		labGroup := testRouter.Group("/:lab")
		labGroup.Use(DBInjector(dbs, labLabs))

		// Register ALL routes BEFORE any ServeHTTP
		testRouter.GET("/setsess", func(c *gin.Context) {
			s := sessions.Default(c)
			s.Set("user_id", 2)
			s.Set("username", "labadmin")
			s.Set("is_super_admin", false)
			s.Set("labs", []string{"test-lab"})
			_ = s.Save()
			c.String(200, "ok")
		})
		labGroup.GET("/protected", func(c *gin.Context) {
			c.Set("lab", c.Param("lab"))
			c.Next()
		}, LabPermissionRequired(), func(c *gin.Context) {
			c.String(200, "allowed")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/setsess", nil)
		testRouter.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("setsess returned %d", w.Code)
		}

		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("GET", "/test-lab/protected", nil)
		for _, ck := range w.Result().Cookies() {
			req2.AddCookie(ck)
		}
		testRouter.ServeHTTP(w2, req2)
		if w2.Code != 200 {
			t.Errorf("expected 200 for permitted user, got %d", w2.Code)
		}
	})
}

// createTestContext creates a minimal gin context with lab set.
func createTestContext(lab string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("lab", lab)
	return c
}
