package handlers

import (
	"net/http"
	"strconv"

	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

func (h *Handler) DeviceLoanList(c *gin.Context) {
	c.Redirect(http.StatusFound, "/devices?tab=loans")
}

func (h *Handler) DeviceLoanCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	c.HTML(http.StatusOK, "device_loan/create.html", gin.H{
		"title": "Tambah Peminjaman", "currentPage": "devices",
		"username": username, "role": role,
	})
}

func (h *Handler) DeviceLoanCreate(c *gin.Context) {
	var req CreateDeviceLoanRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "device_loan/create.html", gin.H{
			"title": "Tambah Peminjaman", "error": "Lengkapi data yang diperlukan",
		})
		return
	}

	deviceID, _ := strconv.Atoi(req.DeviceID)
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	_, err := h.deviceLoanService.CreateLoan(services.CreateLoanInput{
		DeviceID: deviceID, BorrowerName: req.BorrowerName, BorrowerType: req.BorrowerType,
		LoanDate: req.LoanDate, ExpectedReturnDate: req.ExpectedReturnDate,
		Quantity: req.Quantity, Purpose: req.Purpose,
	}, uid, u, r, ip, ua)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "device_loan/create.html", gin.H{
			"title": "Tambah Peminjaman", "error": "Gagal menyimpan peminjaman",
		})
		return
	}
	c.Redirect(http.StatusFound, "/devices?tab=loans")
}

func (h *Handler) DeviceLoanEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id, _ := strconv.Atoi(c.Param("id"))
	loan, err := h.deviceLoanService.GetByID(id)
	if err != nil { h.errHTML(c, "Peminjaman tidak ditemukan"); return }

	c.HTML(http.StatusOK, "device_loan/edit.html", gin.H{
		"title": "Edit Peminjaman", "currentPage": "devices",
		"username": username, "role": role, "loan": loan,
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

	if err := h.deviceLoanService.UpdateLoan(id, services.UpdateLoanInput{
		BorrowerName: req.BorrowerName, BorrowerType: req.BorrowerType,
		LoanDate: req.LoanDate, ExpectedReturnDate: req.ExpectedReturnDate,
		ActualReturnDate: req.ActualReturnDate, Status: req.Status,
		Purpose: req.Purpose, Notes: req.Notes,
	}, uid, u, r, ip, ua); err != nil {
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
