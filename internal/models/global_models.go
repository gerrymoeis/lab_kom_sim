package models

import "time"

type GlobalUser struct {
	ID                 int       `json:"id"`
	Username           string    `json:"username"`
	Password           string    `json:"-"`
	FullName           string    `json:"full_name"`
	IsSuperAdmin       bool      `json:"is_super_admin"`
	IsProtected        bool      `json:"is_protected"`
	SessionToken       string    `json:"-"`
	PasswordIsDefault  bool      `json:"password_is_default"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type LabPermission struct {
	ID            int       `json:"id"`
	UserID        int       `json:"user_id"`
	LabURLPath    string    `json:"lab_url_path"`
	Role          string    `json:"role"`
	IsMainAccount bool      `json:"is_main_account"`
	CreatedAt     time.Time `json:"created_at"`
}

type DefaultCredential struct {
	Username      string
	Password      string
	LabTitle      string
	IsSuperAdmin  bool
	IsMainAccount bool
}

type LabConfig struct {
	ID        int       `json:"id"`
	LabID     string    `json:"lab_id"`
	Title     string    `json:"title"`
	URLPath   string    `json:"url_path"`
	DBPath    string    `json:"db_path"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
