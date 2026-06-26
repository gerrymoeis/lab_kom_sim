package middleware

import (
	"net/http"
	"strings"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Helper: redirect user based on session state when lab is not found
func redirectOnNoLab(c *gin.Context) {
	session := sessions.Default(c)
	if session.Get("user_id") == nil {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}
	isSuperAdmin, _ := session.Get("is_super_admin").(bool)
	isGlobalAdmin, _ := session.Get("is_global_admin").(bool)
	if isSuperAdmin || isGlobalAdmin {
		c.Redirect(http.StatusFound, "/admin/labs")
		c.Abort()
		return
	}
	labsRaw := session.Get("labs")
	if labsRaw != nil {
		if labs, ok := labsRaw.([]string); ok && len(labs) > 0 {
			c.Redirect(http.StatusFound, "/"+labs[0]+"/dashboard")
			c.Abort()
			return
		}
	}
	c.Redirect(http.StatusFound, "/login")
	c.Abort()
}

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
		c.Set("role", "admin")

		userID, _, isSuperAdmin, isGlobalAdmin, ok := GetCurrentUser(c)
		if !ok {
			c.Next()
			return
		}

		c.Set("is_super_admin", isSuperAdmin)
		c.Set("is_global_admin", isGlobalAdmin)
		if isSuperAdmin {
			c.Set("role", "admin")
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

		c.Next()
	}
}

// LabPermissionRequired checks if user has access to current lab
func LabPermissionRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _, isSuperAdmin, _, ok := GetCurrentUser(c)
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
			// Redirect to first allowed lab instead of 403
			if labsVal != nil {
				if labs, ok := labsVal.([]string); ok && len(labs) > 0 {
					c.Redirect(http.StatusFound, "/"+labs[0]+"/dashboard")
					c.Abort()
					return
				}
			}
			c.Redirect(http.StatusFound, "/login")
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
			redirectOnNoLab(c)
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
