package handlers

import (
	"database/sql"
	"net/http"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"

	"github.com/gin-gonic/gin"
)

// DeviceList renders list of all devices
func (h *Handler) DeviceList(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	rows, err := h.db.Query(`
		SELECT id, name, category, brand, condition, location, purchase_date, notes, created_at
		FROM devices
		ORDER BY category, name
	`)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data perangkat",
		})
		return
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var device models.Device
		err := rows.Scan(&device.ID, &device.Name, &device.Category, &device.Brand,
			&device.Condition, &device.Location, &device.PurchaseDate, &device.Notes, &device.CreatedAt)
		if err != nil {
			continue
		}
		devices = append(devices, device)
	}

	c.HTML(http.StatusOK, "device/list.html", gin.H{
		"title":    "Daftar Perangkat - Sistem Inventaris Lab",
		"username": username,
		"role":     role,
		"devices":  devices,
	})
}

// DeviceCreatePage renders device creation form
func (h *Handler) DeviceCreatePage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	c.HTML(http.StatusOK, "device/create.html", gin.H{
		"title":    "Tambah Perangkat Baru - Sistem Inventaris Lab",
		"username": username,
		"role":     role,
	})
}

// DeviceCreate handles device creation
func (h *Handler) DeviceCreate(c *gin.Context) {
	name := c.PostForm("name")
	category := c.PostForm("category")
	brand := c.PostForm("brand")
	condition := c.PostForm("condition")
	location := c.PostForm("location")
	purchaseDate := c.PostForm("purchase_date")
	notes := c.PostForm("notes")

	if name == "" || category == "" {
		c.HTML(http.StatusBadRequest, "device/create.html", gin.H{
			"title": "Tambah Perangkat Baru",
			"error": "Nama dan kategori harus diisi",
		})
		return
	}

	var purchaseDatePtr *string
	if purchaseDate != "" {
		purchaseDatePtr = &purchaseDate
	}

	_, err := h.db.Exec(`
		INSERT INTO devices (name, category, brand, condition, location, purchase_date, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, name, category, brand, condition, location, purchaseDatePtr, notes)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "device/create.html", gin.H{
			"title": "Tambah Perangkat Baru",
			"error": "Gagal menyimpan data perangkat",
		})
		return
	}

	c.Redirect(http.StatusFound, "/devices")
}

// DeviceEditPage renders device edit form
func (h *Handler) DeviceEditPage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	id := c.Param("id")
	var device models.Device
	var purchaseDateStr sql.NullString

	err := h.db.QueryRow(`
		SELECT id, name, category, brand, condition, location, purchase_date, notes
		FROM devices WHERE id = ?
	`, id).Scan(&device.ID, &device.Name, &device.Category, &device.Brand,
		&device.Condition, &device.Location, &purchaseDateStr, &device.Notes)

	if err == sql.ErrNoRows {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Perangkat Tidak Ditemukan",
			"message": "Perangkat yang Anda cari tidak ditemukan",
		})
		return
	}

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data perangkat",
		})
		return
	}

	var purchaseDateFormatted string
	if purchaseDateStr.Valid {
		purchaseDateFormatted = purchaseDateStr.String
	}

	c.HTML(http.StatusOK, "device/edit.html", gin.H{
		"title":        "Edit Perangkat - Sistem Inventaris Lab",
		"username":     username,
		"role":         role,
		"device":       device,
		"purchaseDate": purchaseDateFormatted,
	})
}

// DeviceEdit handles device update
func (h *Handler) DeviceEdit(c *gin.Context) {
	id := c.Param("id")
	name := c.PostForm("name")
	category := c.PostForm("category")
	brand := c.PostForm("brand")
	condition := c.PostForm("condition")
	location := c.PostForm("location")
	purchaseDate := c.PostForm("purchase_date")
	notes := c.PostForm("notes")

	var purchaseDatePtr *string
	if purchaseDate != "" {
		purchaseDatePtr = &purchaseDate
	}

	_, err := h.db.Exec(`
		UPDATE devices 
		SET name = ?, category = ?, brand = ?, condition = ?, location = ?,
		    purchase_date = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, name, category, brand, condition, location, purchaseDatePtr, notes, id)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengupdate data perangkat",
		})
		return
	}

	c.Redirect(http.StatusFound, "/devices")
}

// DeviceDelete handles device deletion
func (h *Handler) DeviceDelete(c *gin.Context) {
	id := c.Param("id")

	_, err := h.db.Exec("DELETE FROM devices WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menghapus perangkat",
		})
		return
	}

	c.Redirect(http.StatusFound, "/devices")
}
