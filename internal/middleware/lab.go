package middleware

import (
	"net/http"
	"strings"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// GlobalDBInjector injects the global database into context
func GlobalDBInjector(globalDB *database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("globalDB", globalDB)
		c.Next()
	}
}

// LabRoleInjector reads lab_permissions and sets role in context.
// Also auto-syncs global user to per-lab users table.
func LabRoleInjector() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("role", "user")

		userID, username, isSuperAdmin, ok := GetCurrentUser(c)
		if !ok {
			c.Next()
			return
		}

		c.Set("is_super_admin", isSuperAdmin)
		if isSuperAdmin {
			c.Set("role", "admin")
			autoSyncUser(c, userID, username, "")
			c.Next()
			return
		}

		lab := c.GetString("lab")
		if lab == "" {
			c.Next()
			return
		}

		gdb, exists := c.Get("globalDB")
		if !exists {
			c.Next()
			return
		}
		globalDB := gdb.(*database.DB)

		var role string
		err := globalDB.QueryRow(
			`SELECT role FROM lab_permissions WHERE user_id = ? AND lab_url_path = ?`,
			userID, lab).Scan(&role)
		if err == nil {
			c.Set("role", role)
		}

		autoSyncUser(c, userID, username, "")

		c.Next()
	}
}

// autoSyncUser ensures the global user exists in the current per-lab users table.
func autoSyncUser(c *gin.Context, userID int, username, fullName string) {
	if fullName == "" {
		session := sessions.Default(c)
		if fn, ok := session.Get("full_name").(string); ok {
			fullName = fn
		} else {
			fullName = username
		}
	}

	dbVal, hasDB := c.Get("db")
	if !hasDB {
		return
	}
	db, ok := dbVal.(*database.DB)
	if !ok {
		return
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", userID).Scan(&count)
	if count == 0 {
		db.Exec(`INSERT INTO users (id, username, password, full_name, role, is_protected, is_super_admin)
			VALUES (?, ?, '', ?, 'user', 0, 0)`, userID, username, fullName)
	}
}

// LabPermissionRequired checks if user has access to current lab
func LabPermissionRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _, isSuperAdmin, ok := GetCurrentUser(c)
		if !ok {
			LabRedirect(c, http.StatusFound, "/login")
			c.Abort()
			return
		}

		lab := c.GetString("lab")
		if lab == "" {
			c.AbortWithStatus(404)
			return
		}

		if isSuperAdmin {
			c.Next()
			return
		}

		// Check session cache first
		labsVal := sessions.Default(c).Get("labs")
		if labsVal != nil {
			if labs, ok := labsVal.([]string); ok {
				for _, l := range labs {
					if l == lab {
						c.Next()
						return
					}
				}
			}
		}

		// Fallback: query global DB
		gdb := c.MustGet("globalDB").(*database.DB)
		var exists int
		_ = gdb.QueryRow(
			`SELECT COUNT(*) FROM lab_permissions WHERE user_id = ? AND lab_url_path = ?`,
			userID, lab).Scan(&exists)
		if exists == 0 {
			c.HTML(http.StatusForbidden, "error.html", gin.H{
				"title":   "Akses Ditolak",
				"message": "Anda tidak memiliki akses ke laboratorium ini.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func DBInjector(dbs map[string]*database.DB, labs map[string]config.LabConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		lab := c.Param("lab")
		if lab == "" {
			lab = c.GetString("lab")
		}
		if _, ok := dbs[lab]; !ok {
			c.AbortWithStatus(404)
			return
		}
		c.Set("db", dbs[lab])
		if lc, ok := labs[lab]; ok {
			c.Set("labConfig", lc)
		}
		c.Set("lab", lab)
		c.Next()
	}
}

func LabURL(c *gin.Context, path string) string {
	lab := c.GetString("lab")
	if lab == "" {
		return path
	}
	if path == "" {
		return "/" + lab
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "/" + lab + path
}

func LabRedirect(c *gin.Context, code int, path string) {
	c.Redirect(code, LabURL(c, path))
}

func LabCookieName(lab string) string {
	if lab == "" {
		return "inventaris_session"
	}
	return "inventaris_session_" + lab
}
