package handlers

import (
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"inventaris-lab-kom/internal/repository"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

func (h *Handler) DeviceLoanList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := h.cfg.DefaultPageSize

	values, _ := url.ParseQuery(c.Request.URL.RawQuery)
	delete(values, "page")
	var query interface{} = ""
	if len(values) > 0 {
		query = template.URL("&" + values.Encode())
	}

	search := c.Query("search")
	status := c.Query("status")
	sortBy := c.Query("sort_by")

	loans, total, err := h.deviceLoanService.ListPaginated(repository.DeviceLoanFilters{
		Search: search,
		Status: status,
		SortBy: sortBy,
	}, page, pageSize)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data peminjaman")
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	c.HTML(http.StatusOK, "device_loan/list.html", gin.H{
		"title": "Peminjaman", "currentPage": "devices",
		"username": username, "role": role,
		"loans": loans,
		"filters": gin.H{"search": search, "status": status, "sort_by": sortBy},
		"startRow": (page-1)*pageSize + 1,
		"page": page, "totalPages": totalPages, "totalItems": total,
		"query": query,
	})
}

func (h *Handler) DeviceLoanCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}
	devices, err := h.deviceLoanService.GetLoanableDevices()
	if err != nil {
		devices = nil
	}
	deviceID, _ := strconv.Atoi(c.DefaultQuery("device_id", "0"))
	c.HTML(http.StatusOK, "device_loan/create.html", gin.H{
		"title": "Tambah Peminjaman", "currentPage": "devices",
		"username": username, "role": role,
		"devices": devices, "preselectDeviceID": deviceID,
	})
}

func (h *Handler) DeviceLoanCreate(c *gin.Context) {
	var req CreateDeviceLoanRequest
	if err := c.ShouldBind(&req); err != nil {
		_, username, role, _ := h.user(c)
		c.HTML(http.StatusBadRequest, "device_loan/create.html", gin.H{
			"title": "Tambah Peminjaman", "currentPage": "devices",
			"username": username, "role": role, "error": "Lengkapi data yang diperlukan",
		})
		return
	}

	deviceID, _ := strconv.Atoi(req.DeviceID)
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	_, err := h.deviceLoanService.CreateLoan(services.CreateLoanInput{
		DeviceID:     deviceID,
		BorrowerName: req.BorrowerName,
		BorrowerType: req.BorrowerType,
		LoanDate:     req.LoanDate,
		ReturnDate:   req.ReturnDate,
		Purpose:      req.Purpose,
	}, uid, u, r, ip, ua)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "device_loan/create.html", gin.H{
			"title": "Tambah Peminjaman", "currentPage": "devices",
			"username": u, "role": r, "error": "Gagal menyimpan peminjaman",
		})
		return
	}
	c.Redirect(http.StatusFound, "/devices?tab=loans")
}

func (h *Handler) DeviceLoanDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	loan, err := h.deviceLoanService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Peminjaman tidak ditemukan")
		return
	}

	extensions, _ := h.deviceLoanService.GetExtensionsByLoanID(id)

	c.HTML(http.StatusOK, "device_loan/detail.html", gin.H{
		"title": "Detail Peminjaman", "currentPage": "devices",
		"username": username, "role": role,
		"loan":       loan,
		"assetCode":  loan.DeviceAssetCode,
		"extensions": extensions,
	})
}

func (h *Handler) DeviceLoanEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	loan, err := h.deviceLoanService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Peminjaman tidak ditemukan")
		return
	}

	c.HTML(http.StatusOK, "device_loan/edit.html", gin.H{
		"title": "Edit Peminjaman", "currentPage": "devices",
		"username": username, "role": role,
		"loan": loan,
		"assetCode": loan.DeviceAssetCode,
		"deviceTypeName": loan.DeviceTypeName,
	})
}

func (h *Handler) DeviceLoanEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req EditDeviceLoanRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	var actualReturnDate *time.Time
	if req.Status == "returned" && req.ActualReturnDate != "" {
		if t, err := time.Parse("2006-01-02", req.ActualReturnDate); err == nil {
			actualReturnDate = &t
		}
	}

	if err := h.deviceLoanService.UpdateReturn(id, actualReturnDate, req.Notes, uid, u, r, ip, ua); err != nil {
		h.errHTML(c, "Gagal mengupdate peminjaman")
		return
	}
	c.Redirect(http.StatusFound, "/devices?tab=loans")
}

func (h *Handler) DeviceLoanDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceLoanService.DeleteLoan(id, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/devices?tab=loans", "Gagal menghapus peminjaman")
		return
	}
	c.Redirect(http.StatusFound, "/devices?tab=loans")
}

func (h *Handler) DeviceLoanExtend(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	newReturnDate := c.PostForm("return_date")
	if newReturnDate == "" {
		h.errJSON(c, http.StatusBadRequest, "Tanggal kembali baru harus diisi")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceLoanService.ExtendLoan(id, newReturnDate, uid, u, r, ip, ua); err != nil {
		h.errJSON(c, http.StatusInternalServerError, "Gagal memperpanjang peminjaman")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
