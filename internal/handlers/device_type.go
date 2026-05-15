package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"inventaris-lab-kom/internal/models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) DeviceTypeList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	search := c.Query("search")
	category := c.Query("category")

	query := `SELECT id, name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template, created_at FROM device_types WHERE 1=1`
	var args []interface{}
	if search != "" {
		query += ` AND (name LIKE ? OR category LIKE ?)`
		s := "%" + search + "%"
		args = append(args, s, s)
	}
	if category != "" {
		query += ` AND category = ?`
		args = append(args, category)
	}
	query += ` ORDER BY category, name`

	rows, err := h.db.Query(query, args...)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data jenis barang")
		return
	}
	defer rows.Close()

	var types []models.DeviceType
	for rows.Next() {
		var dt models.DeviceType
		var brand, model, prefix, location, notes sql.NullString
		if rows.Scan(&dt.ID, &dt.Name, &dt.Category, &brand, &model, &dt.ItemType, &dt.IsLoanable, &dt.IsConsumable, &prefix, &location, &notes, &dt.CreatedAt) != nil {
			continue
		}
		if brand.Valid { dt.Brand = brand.String }
		if model.Valid { dt.Model = model.String }
		if prefix.Valid { dt.AssetCodePrefix = prefix.String }
		if location.Valid { dt.DefaultLocation = location.String }
		if notes.Valid { dt.NotesTemplate = notes.String }
		types = append(types, dt)
	}

	c.HTML(http.StatusOK, "device_type/list.html", gin.H{
		"title": "Jenis Barang", "currentPage": "devices",
		"username": username, "role": role,
		"deviceTypes": types, "search": search, "category": category,
	})
}

func (h *Handler) DeviceTypeDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id := c.Param("id")
	var dt models.DeviceType
	var brand, model, prefix, location, notes sql.NullString
	err := h.db.QueryRow(`SELECT id, name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template, created_at FROM device_types WHERE id = ?`, id).
		Scan(&dt.ID, &dt.Name, &dt.Category, &brand, &model, &dt.ItemType, &dt.IsLoanable, &dt.IsConsumable, &prefix, &location, &notes, &dt.CreatedAt)
	if err != nil {
		h.errHTML(c, "Jenis barang tidak ditemukan")
		return
	}
	if brand.Valid { dt.Brand = brand.String }
	if model.Valid { dt.Model = model.String }
	if prefix.Valid { dt.AssetCodePrefix = prefix.String }
	if location.Valid { dt.DefaultLocation = location.String }
	if notes.Valid { dt.NotesTemplate = notes.String }

	c.HTML(http.StatusOK, "device_type/detail.html", gin.H{
		"title": "Detail Jenis Barang", "currentPage": "devices",
		"username": username, "role": role, "deviceType": dt,
	})
}

func (h *Handler) DeviceTypeCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	c.HTML(http.StatusOK, "device_type/create.html", gin.H{
		"title": "Tambah Jenis Barang", "currentPage": "devices",
		"username": username, "role": role,
	})
}

func (h *Handler) DeviceTypeCreate(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Akses ditolak"); return }

	name := c.PostForm("name")
	category := c.PostForm("category")
	brand := c.PostForm("brand")
	model := c.PostForm("model")
	itemType := c.PostForm("item_type")
	itemMode := c.PostForm("item_mode")
	prefix := c.PostForm("asset_code_prefix")
	location := c.PostForm("default_location")
	notes := c.PostForm("notes_template")

	if name == "" || category == "" || itemType == "" {
		c.HTML(http.StatusBadRequest, "device_type/create.html", gin.H{
			"title": "Tambah Jenis Barang", "error": "Nama, kategori, dan tipe item harus diisi",
		})
		return
	}

	result, err := h.db.Exec(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		name, category, brand, model, itemType, itemMode == "loanable", itemMode == "consumable", prefix, location, notes)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			c.HTML(http.StatusBadRequest, "device_type/create.html", gin.H{
				"title": "Tambah Jenis Barang", "error": "Nama jenis barang sudah ada",
			})
			return
		}
		h.logCreateError(c, "device_type", map[string]interface{}{"name": name}, err.Error())
		c.HTML(http.StatusInternalServerError, "device_type/create.html", gin.H{
			"title": "Tambah Jenis Barang", "error": "Gagal menyimpan data",
		})
		return
	}

	id, _ := result.LastInsertId()
	h.logCreate(c, "device_type", int(id), map[string]interface{}{
		"name": name, "category": category, "item_type": itemType,
	})
	c.Redirect(http.StatusFound, "/device-types")
}

func (h *Handler) DeviceTypeEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id := c.Param("id")
	rows := h.db.QueryRow(`SELECT id, name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template FROM device_types WHERE id = ?`, id)
	var dt models.DeviceType
	var brand, model, prefix, location, notes sql.NullString
	if err := rows.Scan(&dt.ID, &dt.Name, &dt.Category, &brand, &model, &dt.ItemType, &dt.IsLoanable, &dt.IsConsumable, &prefix, &location, &notes); err != nil {
		h.errHTML(c, "Jenis barang tidak ditemukan")
		return
	}
	if brand.Valid { dt.Brand = brand.String }
	if model.Valid { dt.Model = model.String }
	if prefix.Valid { dt.AssetCodePrefix = prefix.String }
	if location.Valid { dt.DefaultLocation = location.String }
	if notes.Valid { dt.NotesTemplate = notes.String }

	c.HTML(http.StatusOK, "device_type/edit.html", gin.H{
		"title": "Edit Jenis Barang", "currentPage": "devices",
		"username": username, "role": role, "deviceType": dt,
	})
}

func (h *Handler) DeviceTypeEdit(c *gin.Context) {
	id := c.Param("id")
	name := c.PostForm("name")
	category := c.PostForm("category")
	brand := c.PostForm("brand")
	model := c.PostForm("model")
	itemType := c.PostForm("item_type")
	itemMode := c.PostForm("item_mode")
	prefix := c.PostForm("asset_code_prefix")
	location := c.PostForm("default_location")
	notes := c.PostForm("notes_template")

	_, err := h.db.Exec(`UPDATE device_types SET name=?, category=?, brand=?, model=?, item_type=?, is_loanable=?, is_consumable=?, asset_code_prefix=?, default_location=?, notes_template=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		name, category, brand, model, itemType, itemMode == "loanable", itemMode == "consumable", prefix, location, notes, id)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			c.HTML(http.StatusBadRequest, "device_type/edit.html", gin.H{
				"title": "Edit Jenis Barang", "error": "Nama sudah digunakan",
			})
			return
		}
		h.logUpdateError(c, "device_type", 0, map[string]interface{}{"id": id}, err.Error())
		h.errHTML(c, "Gagal mengupdate jenis barang")
		return
	}

	h.logUpdate(c, "device_type", 0,
		map[string]interface{}{"id": id},
		map[string]interface{}{"name": name, "category": category},
	)
	c.Redirect(http.StatusFound, "/device-types")
}

func (h *Handler) DeviceTypeDelete(c *gin.Context) {
	id := c.Param("id")

	_, err := h.db.Exec("DELETE FROM device_types WHERE id = ?", id)
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") {
			h.redirectWithError(c, "/device-types", "Jenis barang masih digunakan oleh perangkat")
			return
		}
		h.logDeleteError(c, "device_type", 0, map[string]interface{}{"id": id}, err.Error())
		h.redirectWithError(c, "/device-types", "Gagal menghapus jenis barang")
		return
	}

	h.logDelete(c, "device_type", 0, map[string]interface{}{"id": id})
	c.Redirect(http.StatusFound, "/device-types")
}
