package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

func (h *Handler) fetchDeviceTypes() []models.DeviceType {
	dts, err := h.deviceTypeRepo.GetAllSimple()
	if err != nil { return nil }
	return dts
}

func (h *Handler) DeviceList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	switch c.DefaultQuery("tab", "list") {
	case "list":   h.deviceListTab(c, username, role)
	case "types":  h.deviceTypesTab(c, username, role)
	case "loans":  h.deviceLoansTab(c, username, role)
	case "usages": h.deviceUsagesTab(c, username, role)
	}
}

func (h *Handler) deviceListTab(c *gin.Context, username, role string) {
	filters := struct{ Search, Category string }{
		Search:   c.Query("search"),
		Category: c.Query("category"),
	}

	devices, err := h.deviceRepo.List(repository.DeviceFilters{
		Search:   filters.Search,
		Category: filters.Category,
	})
	if err != nil { h.errHTML(c, "Gagal mengambil data perangkat"); return }

	c.HTML(http.StatusOK, "device/list.html", gin.H{
		"title": "Manajemen Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"activeTab": "list", "devices": devices,
		"deviceTypes": h.fetchDeviceTypes(),
		"filters": gin.H{"search": filters.Search, "category": filters.Category},
	})
}

func (h *Handler) deviceTypesTab(c *gin.Context, username, role string) {
	search := c.Query("search")
	category := c.Query("category")

	dts, err := h.deviceTypeRepo.List(category)
	if err != nil { h.errHTML(c, "Gagal mengambil data jenis barang"); return }

	if search != "" {
		var filtered []models.DeviceType
		for _, dt := range dts {
			if strings.Contains(strings.ToLower(dt.Name), strings.ToLower(search)) ||
				strings.Contains(strings.ToLower(dt.Category), strings.ToLower(search)) {
				filtered = append(filtered, dt)
			}
		}
		dts = filtered
	}

	c.HTML(http.StatusOK, "device/list.html", gin.H{
		"title": "Jenis Barang", "currentPage": "devices",
		"username": username, "role": role,
		"activeTab": "types", "deviceTypes": dts,
	})
}

func (h *Handler) deviceLoansTab(c *gin.Context, username, role string) {
	loans, err := h.deviceRepo.ListLoans()
	if err != nil { h.errHTML(c, "Gagal mengambil data peminjaman"); return }

	c.HTML(http.StatusOK, "device/list.html", gin.H{
		"title": "Peminjaman", "currentPage": "devices",
		"username": username, "role": role,
		"activeTab": "loans", "loans": loans,
	})
}

func (h *Handler) deviceUsagesTab(c *gin.Context, username, role string) {
	usages, err := h.deviceRepo.ListUsages()
	if err != nil { h.errHTML(c, "Gagal mengambil data pemakaian"); return }

	c.HTML(http.StatusOK, "device/list.html", gin.H{
		"title": "Pemakaian", "currentPage": "devices",
		"username": username, "role": role,
		"activeTab": "usages", "usages": usages,
	})
}

func (h *Handler) DeviceCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	c.HTML(http.StatusOK, "device/create.html", gin.H{
		"title": "Tambah Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"deviceTypes": h.fetchDeviceTypes(),
	})
}

func (h *Handler) DeviceCreate(c *gin.Context) {
	var req CreateDeviceRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "device/create.html", gin.H{
			"title": "Tambah Perangkat", "error": "Lengkapi data yang diperlukan",
		})
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	_, _, err := h.deviceService.CreateDevice(services.CreateDeviceInput{
		DeviceTypeID: req.DeviceTypeID, Name: req.Name, Brand: req.Brand, Model: req.Model,
		SerialNumber: req.SerialNumber, ItemType: req.ItemType, ItemMode: req.ItemMode,
		Quantity: req.Quantity, Condition: req.Condition, Location: req.Location,
		PurchaseDate: req.PurchaseDate, Notes: req.Notes,
	}, uid, u, r, ip, ua)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "device/create.html", gin.H{
			"title": "Tambah Perangkat", "error": "Gagal menyimpan perangkat",
		})
		return
	}
	c.Redirect(http.StatusFound, "/devices")
}

func (h *Handler) DeviceDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id, _ := strconv.Atoi(c.Param("id"))
	data, err := h.deviceService.GetDetail(id)
	if err != nil {
		h.errHTML(c, "Perangkat tidak ditemukan")
		return
	}

	c.HTML(http.StatusOK, "device/detail.html", gin.H{
		"title": "Detail Perangkat", "currentPage": "devices",
		"username": username, "role": role, "device": data.Device,
		"deviceTypeName": data.DeviceTypeName, "loans": data.Loans, "usages": data.Usages,
	})
}

func (h *Handler) DeviceEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id, _ := strconv.Atoi(c.Param("id"))
	d, err := h.deviceRepo.GetByIDSimple(id)
	if err != nil { h.errHTML(c, "Perangkat tidak ditemukan"); return }

	c.HTML(http.StatusOK, "device/edit.html", gin.H{
		"title": "Edit Perangkat", "currentPage": "devices",
		"username": username, "role": role, "device": d,
		"deviceTypes": h.fetchDeviceTypes(),
	})
}

func (h *Handler) DeviceEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req EditDeviceRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceService.UpdateDevice(id, services.UpdateDeviceInput{
		DeviceTypeID: req.DeviceTypeID, Name: req.Name, Brand: req.Brand, Model: req.Model,
		SerialNumber: req.SerialNumber, ItemType: req.ItemType, ItemMode: req.ItemMode,
		QuantityTotal: req.QuantityTotal, QuantityAvailable: req.QuantityAvailable,
		Condition: req.Condition, Location: req.Location,
		PurchaseDate: req.PurchaseDate, Notes: req.Notes,
	}, uid, u, r, ip, ua); err != nil {
		h.errHTML(c, "Gagal mengupdate perangkat")
		return
	}
	c.Redirect(http.StatusFound, "/devices")
}

func (h *Handler) DeviceDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceService.DeleteDevice(id, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/devices", "Gagal menghapus perangkat")
		return
	}
	c.Redirect(http.StatusFound, "/devices")
}

func (h *Handler) GetNextAssetCode(c *gin.Context) {
	prefix := c.Query("prefix")
	code := h.deviceRepo.GetNextAssetCode(prefix)
	c.JSON(http.StatusOK, gin.H{"next_code": code})
}

func (h *Handler) DeviceExport(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat export data"); return }

	svc := services.NewExcelService()
	yn := map[bool]string{true: "Ya", false: "Tidak"}

	devices, _ := h.deviceRepo.ExportAll()
	dData := make([][]interface{}, 0, len(devices))
	for _, d := range devices {
		dData = append(dData, []interface{}{d.AssetCode, d.Name, d.Category, d.Brand, d.Model, d.SerialNumber, d.ItemType, d.QuantityTotal, d.QuantityAvailable, d.Condition, d.Location})
	}

	dtRows, _ := h.deviceRepo.ExportDeviceTypes()
	tData := make([][]interface{}, 0, len(dtRows))
	for _, dt := range dtRows {
		pref := dt.AssetCodePrefix; if pref == "" { pref = "-" }
		loc := dt.DefaultLocation; if loc == "" { loc = "-" }
		tData = append(tData, []interface{}{dt.Name, dt.Category, dt.Brand, dt.Model, dt.ItemType, yn[dt.IsLoanable], yn[dt.IsConsumable], pref, loc})
	}

	loans, _ := h.deviceRepo.ExportLoans()
	lData := make([][]interface{}, 0, len(loans))
	for _, l := range loans {
		fd := func(t *time.Time) string { if t != nil { return t.Format("2006-01-02") }; return "-" }
		lData = append(lData, []interface{}{l.DeviceAssetCode, l.DeviceName, l.BorrowerName, l.BorrowerType, l.LoanDate.Format("2006-01-02"), fd(l.ExpectedReturnDate), fd(l.ActualReturnDate), l.Quantity, l.Status, l.Purpose})
	}

	usages, _ := h.deviceRepo.ExportUsages()
	uData := make([][]interface{}, 0, len(usages))
	for _, u := range usages {
		uData = append(uData, []interface{}{u.DeviceAssetCode, u.DeviceName, u.UserName, u.UserType, u.UsageDate.Format("2006-01-02"), u.Quantity, u.Purpose})
	}

	f, _ := svc.GenerateMultiSheetExcel([]services.ExcelExportConfig{
		{
			SheetName: "Perangkat",
			Headers:   []string{"Kode Aset", "Nama", "Kategori", "Brand", "Model", "Serial Number", "Tipe Item", "Total", "Tersedia", "Kondisi", "Lokasi"},
			Data:      dData,
			ColumnWidths: map[string]float64{"A": 14, "B": 28, "C": 18, "D": 16, "E": 20, "F": 18, "G": 14, "H": 10, "I": 12, "J": 14, "K": 22},
		},
		{
			SheetName: "Jenis Barang",
			Headers:   []string{"Nama", "Kategori", "Brand", "Model", "Tipe Item", "Bisa Dipinjam", "Habis Pakai", "Prefix Aset", "Lokasi Default"},
			Data:      tData,
			ColumnWidths: map[string]float64{"A": 24, "B": 16, "C": 16, "D": 20, "E": 14, "F": 14, "G": 14, "H": 14, "I": 22},
		},
		{
			SheetName: "Peminjaman",
			Headers:   []string{"Kode Aset", "Nama", "Peminjam", "Tipe", "Tgl Pinjam", "Tgl Kembali (Rencana)", "Tgl Kembali (Aktual)", "Qty", "Status", "Tujuan"},
			Data:      lData,
			ColumnWidths: map[string]float64{"A": 14, "B": 26, "C": 24, "D": 14, "E": 16, "F": 22, "G": 22, "H": 8, "I": 14, "J": 28},
		},
		{
			SheetName: "Pemakaian",
			Headers:   []string{"Kode Aset", "Nama", "Pengguna", "Tipe", "Tanggal", "Qty", "Tujuan"},
			Data:      uData,
			ColumnWidths: map[string]float64{"A": 14, "B": 26, "C": 24, "D": 14, "E": 16, "F": 8, "G": 28},
		},
	})
	defer f.Close()

	fn := svc.GenerateFilename("devices_export")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+fn)
	f.Write(c.Writer)
}
