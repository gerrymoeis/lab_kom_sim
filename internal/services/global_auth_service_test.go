package services

import (
	"testing"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

func setupGlobalAuthTest(t *testing.T) (*GlobalAuthService, *repository.GlobalUserRepository, *database.DB) {
	t.Helper()
	db, err := database.InitDB(t.TempDir()+"/global_auth_test.db", "")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Create global tables manually (minimal — only what GlobalAuthService needs)
	tables := []string{
		`CREATE TABLE IF NOT EXISTS global_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			full_name TEXT NOT NULL DEFAULT '',
			is_super_admin INTEGER NOT NULL DEFAULT 0,
			is_protected INTEGER NOT NULL DEFAULT 0,
			is_global_admin INTEGER NOT NULL DEFAULT 0,
			session_token TEXT DEFAULT '',
			password_is_default INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS lab_permissions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES global_users(id),
			lab_url_path TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'admin' CHECK(role IN ('admin')),
			is_main_account INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, lab_url_path)
		)`,
	}
	for _, ddl := range tables {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	repo := repository.NewGlobalUserRepository(db)
	svc := NewGlobalAuthService(repo)
	return svc, repo, db
}

func seedTestUser(t *testing.T, db *database.DB, username, password, fullName string, isSuperAdmin, isProtected bool) int {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	sa := 0
	if isSuperAdmin {
		sa = 1
	}
	prot := 0
	if isProtected {
		prot = 1
	}
	res, err := db.Exec(`INSERT INTO global_users (username, password, full_name, is_super_admin, is_protected, session_token, password_is_default)
		VALUES (?, ?, ?, ?, ?, '', 0)`, username, string(hash), fullName, sa, prot)
	if err != nil {
		t.Fatalf("insert user %s: %v", username, err)
	}
	id, _ := res.LastInsertId()
	return int(id)
}

func seedPermission(t *testing.T, db *database.DB, userID int, labURLPath, role string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO lab_permissions (user_id, lab_url_path, role) VALUES (?, ?, ?)`, userID, labURLPath, role)
	if err != nil {
		t.Fatalf("insert permission: %v", err)
	}
}

// --- Tests ---

func TestGlobalAuthLogin(t *testing.T) {
	svc, _, db := setupGlobalAuthTest(t)
	seedTestUser(t, db, "testuser", "secret123", "Test User", false, false)

	t.Run("success", func(t *testing.T) {
		user, token, err := svc.Login("testuser", "secret123")
		if err != nil {
			t.Fatalf("Login: %v", err)
		}
		if user == nil {
			t.Fatal("expected non-nil user")
		}
		if user.Username != "testuser" {
			t.Errorf("expected username 'testuser', got %q", user.Username)
		}
		if token == "" {
			t.Error("expected non-empty session token")
		}
	})

	t.Run("fail_wrong_password", func(t *testing.T) {
		_, _, err := svc.Login("testuser", "wrongpass")
		if err != ErrInvalidCredentials {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("fail_nonexistent_user", func(t *testing.T) {
		_, _, err := svc.Login("nobody", "password")
		if err != ErrInvalidCredentials {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("fail_already_logged_in", func(t *testing.T) {
		// First login already created session_token from "success" subtest
		_, _, err := svc.Login("testuser", "secret123")
		if err != ErrAlreadyLoggedIn {
			t.Errorf("expected ErrAlreadyLoggedIn, got %v", err)
		}
	})
}

func TestGlobalAuthLogout(t *testing.T) {
	svc, repo, db := setupGlobalAuthTest(t)
	id := seedTestUser(t, db, "logoutuser", "pass", "Logout User", false, false)

	// Login first
	_, _, err := svc.Login("logoutuser", "pass")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	t.Run("clears_session_token", func(t *testing.T) {
		token, _ := repo.GetSessionToken(id)
		if token == "" {
			t.Fatal("expected session_token before logout")
		}

		svc.Logout(id)

		token, _ = repo.GetSessionToken(id)
		if token != "" {
			t.Error("expected empty session_token after logout")
		}
	})

	t.Run("can_login_again_after_logout", func(t *testing.T) {
		user, token, err := svc.Login("logoutuser", "pass")
		if err != nil {
			t.Fatalf("Login after logout: %v", err)
		}
		if token == "" {
			t.Error("expected non-empty token after re-login")
		}
		_ = user
	})
}

func TestGlobalAuthPermissions(t *testing.T) {
	svc, repo, db := setupGlobalAuthTest(t)
	id := seedTestUser(t, db, "permuser", "pass", "Permission User", false, false)

	t.Run("get_permissions_empty", func(t *testing.T) {
		perms, err := svc.GetPermissions(id)
		if err != nil {
			t.Fatalf("GetPermissions: %v", err)
		}
		if len(perms) != 0 {
			t.Errorf("expected 0 permissions, got %d", len(perms))
		}
	})

	t.Run("add_permission_then_get", func(t *testing.T) {
		repo.AddPermission(id, "lab-a", "admin")
		repo.AddPermission(id, "lab-b", "admin")

		perms, err := svc.GetPermissions(id)
		if err != nil {
			t.Fatalf("GetPermissions: %v", err)
		}
		if len(perms) != 2 {
			t.Errorf("expected 2 permissions, got %d", len(perms))
		}
	})
}

func TestGlobalAuthGetLabsForUser(t *testing.T) {
	svc, repo, db := setupGlobalAuthTest(t)

	allLabs := []config.LabConfig{
		{URLPath: "lab-a"},
		{URLPath: "lab-b"},
		{URLPath: "lab-c"},
	}

	t.Run("super_admin_gets_all_labs", func(t *testing.T) {
		id := seedTestUser(t, db, "super", "pass", "Super", true, false)
		paths := svc.GetLabsForUser(id, true, allLabs)
		if len(paths) != 3 {
			t.Errorf("expected 3 labs for super admin, got %d", len(paths))
		}
	})

	t.Run("regular_user_gets_only_permitted_labs", func(t *testing.T) {
		id := seedTestUser(t, db, "regular", "pass", "Regular", false, false)
		repo.AddPermission(id, "lab-a", "admin")
		repo.AddPermission(id, "lab-c", "admin")

		paths := svc.GetLabsForUser(id, false, allLabs)
		if len(paths) != 2 {
			t.Errorf("expected 2 labs, got %d", len(paths))
		}
		if paths[0] != "lab-a" || paths[1] != "lab-c" {
			t.Errorf("expected [lab-a lab-c], got %v", paths)
		}
	})

	t.Run("no_permissions_returns_empty_slice", func(t *testing.T) {
		paths := svc.GetLabsForUser(999, false, allLabs)
		if len(paths) != 0 {
			t.Errorf("expected empty slice for unknown user, got %v (len=%d)", paths, len(paths))
		}
	})
}
