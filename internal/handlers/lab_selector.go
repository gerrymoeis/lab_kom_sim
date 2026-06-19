package handlers

import (
	"net/http"

	"inventaris-lab-kom/internal/config"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func (h *Handler) LabSelector(c *gin.Context) {
	session := sessions.Default(c)
	if session.Get("user_id") == nil {
		c.Redirect(http.StatusFound, "/")
		return
	}

	username, _ := session.Get("username").(string)
	fullName, _ := session.Get("full_name").(string)
	isSuperAdmin, _ := session.Get("is_super_admin").(bool)

	var labs []config.LabConfig
	if isSuperAdmin {
		labs = h.cfg.Labs
	} else {
		allowedRaw := session.Get("labs")
		if allowedRaw == nil {
			c.Redirect(http.StatusFound, "/")
			return
		}
		allowed, _ := allowedRaw.([]string)
		allowedSet := make(map[string]bool, len(allowed))
		for _, l := range allowed {
			allowedSet[l] = true
		}
		for _, lab := range h.cfg.Labs {
			if allowedSet[lab.URLPath] {
				labs = append(labs, lab)
			}
		}
	}

	h.renderTemplate(c, http.StatusOK, "lab_selector.html", gin.H{
		"title":        "Pilih Laboratorium",
		"username":     username,
		"fullName":     fullName,
		"isSuperAdmin": isSuperAdmin,
		"labs":         labs,
	})
}
