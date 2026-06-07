document.addEventListener('DOMContentLoaded', function() {
    var tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
    var tooltipList = tooltipTriggerList.map(function (tooltipTriggerEl) {
        return new bootstrap.Tooltip(tooltipTriggerEl);
    });

    const alerts = document.querySelectorAll('.alert.alert-dismissible');
    alerts.forEach(alert => {
        setTimeout(() => {
            const bsAlert = new bootstrap.Alert(alert);
            bsAlert.close();
        }, 5000);
    });

    document.querySelectorAll('.availability-select').forEach(sel => {
        sel.addEventListener('change', function() {
            updateAvailability(this);
        });
    });

    document.querySelectorAll('[data-default-date]').forEach(el => {
        el.valueAsDate = new Date();
    });
});

// --- Delete confirmation modal ---
var deleteState = {};

window.confirmDelete = function(opts) {
    var message = opts.message || 'Apakah Anda yakin ingin menghapus data ini?';
    var requirePrefix = opts.requirePrefix || false;
    var prefix = opts.prefix || '';
    var type = opts.type || '';
    var formId = opts.formId || '';

    document.getElementById('deleteConfirmMessage').textContent = message;
    document.getElementById('deleteConfirmBatchSection').classList.add('d-none');
    document.getElementById('deleteConfirmTitle').innerHTML = '<i class="bi bi-exclamation-triangle text-danger me-1"></i> Konfirmasi Hapus';

    var inputSection = document.getElementById('deleteConfirmInputSection');
    var input = document.getElementById('deleteConfirmInput');
    var errEl = document.getElementById('deleteConfirmError');
    var label = document.getElementById('deleteConfirmLabel');

    if (requirePrefix && prefix) {
        label.innerHTML = 'Ketik &quot;' + prefix + '&quot; untuk mengkonfirmasi penghapusan ' + type + '.';
        input.value = '';
        input.classList.remove('is-invalid');
        errEl.classList.add('d-none');
        errEl.textContent = '';
        inputSection.classList.remove('d-none');
    } else {
        inputSection.classList.add('d-none');
    }

    deleteState = { requirePrefix: requirePrefix, prefix: prefix, formId: formId, batchItems: null };

    var modalEl = document.getElementById('deleteConfirmModal');
    var modal = bootstrap.Modal.getInstance(modalEl);
    if (!modal) modal = new bootstrap.Modal(modalEl);
    modal.show();
};

window.confirmBatchDelete = function(opts) {
    var items = opts.items || [];
    var url = opts.url || '';

    deleteState = { batchItems: items, batchUrl: url, requirePrefix: false, prefix: '', formId: '' };

    var titleEl = document.getElementById('deleteConfirmTitle');
    titleEl.innerHTML = '<i class="bi bi-exclamation-triangle text-danger me-1"></i> Konfirmasi Hapus Massal';

    document.getElementById('deleteConfirmMessage').textContent = '';
    document.getElementById('deleteConfirmInputSection').classList.add('d-none');

    var batchSection = document.getElementById('deleteConfirmBatchSection');
    batchSection.classList.remove('d-none');

    document.getElementById('batchCountLabel').textContent = items.length + ' item akan dihapus:';

    var listEl = document.getElementById('deleteConfirmBatchList');
    listEl.innerHTML = '';
    items.forEach(function(item) {
        var div = document.createElement('div');
        div.className = 'py-1 border-bottom border-light';
        div.innerHTML = '<i class="bi bi-record-circle text-danger me-1 small"></i> ' + item.label;
        listEl.appendChild(div);
    });

    var modalEl = document.getElementById('deleteConfirmModal');
    var modal = bootstrap.Modal.getInstance(modalEl);
    if (!modal) modal = new bootstrap.Modal(modalEl);
    modal.show();
};

document.addEventListener('DOMContentLoaded', function() {
    document.getElementById('deleteConfirmBtn')?.addEventListener('click', function() {
        if (deleteState.batchItems) {
            var btn = document.getElementById('deleteConfirmBtn');
            showLoading(btn);
            var ids = deleteState.batchItems.map(function(item) { return item.id; });
            fetchWithCSRF(deleteState.batchUrl, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ ids: ids })
            }).then(function(r) {
                if (r.ok) {
                    window.location.reload();
                } else {
                    return r.json().then(function(d) { throw new Error(d.error || 'Gagal menghapus'); }).catch(function() {
                        throw new Error('Gagal menghapus data');
                    });
                }
            }).catch(function(err) {
                hideLoading(btn);
                showToast(err.message, 'error');
                var modal = bootstrap.Modal.getInstance(document.getElementById('deleteConfirmModal'));
                if (modal) modal.hide();
            });
            return;
        }

        if (!deleteState.formId) return;

        if (deleteState.requirePrefix) {
            var input = document.getElementById('deleteConfirmInput').value;
            if (input !== deleteState.prefix) {
                var inputEl = document.getElementById('deleteConfirmInput');
                var errEl = document.getElementById('deleteConfirmError');
                inputEl.classList.add('is-invalid');
                errEl.innerHTML = 'Ketik &quot;' + deleteState.prefix + '&quot; dengan benar.';
                errEl.classList.remove('d-none');
                return;
            }
        }

        document.getElementById(deleteState.formId).submit();
    });
});

function showLoading(button) {
    const originalText = button.innerHTML;
    button.disabled = true;
    button.setAttribute('aria-busy', 'true');
    button.innerHTML = '<span class="spinner-border spinner-border-sm me-2" role="status" aria-hidden="true"></span>Loading...';
    button.dataset.originalText = originalText;
}

function hideLoading(button) {
    button.disabled = false;
    button.removeAttribute('aria-busy');
    button.innerHTML = button.dataset.originalText;
}

function filterTable(inputId, tableId) {
    const input = document.getElementById(inputId);
    const filter = input.value.toUpperCase();
    const table = document.getElementById(tableId);
    const tr = table.getElementsByTagName('tr');

    for (let i = 1; i < tr.length; i++) {
        let found = false;
        const td = tr[i].getElementsByTagName('td');
        
        for (let j = 0; j < td.length; j++) {
            if (td[j]) {
                const txtValue = td[j].textContent || td[j].innerText;
                if (txtValue.toUpperCase().indexOf(filter) > -1) {
                    found = true;
                    break;
                }
            }
        }
        
        tr[i].style.display = found ? '' : 'none';
    }
}

function updateAvailability(selectEl) {
    const usageId = selectEl.dataset.usageId;
    const formData = new FormData();
    formData.append('is_available', selectEl.value);

    fetch('/device-usages/' + usageId + '/availability', {
        method: 'POST',
        headers: { 'X-CSRF-Token': document.querySelector('meta[name="csrf-token"]')?.getAttribute('content') || '' },
        body: formData
    }).then(r => r.json()).then(d => {
        if (!d.success) alert('Error: ' + (d.error || 'Gagal'));
    }).catch(() => alert('Gagal menyimpan'));
}

const FormValidator = {
    init: function() {
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', () => this._setupForms());
        } else {
            this._setupForms();
        }
    },

    _setupForms: function() {
        document.querySelectorAll('form[data-validate="true"]').forEach(form => {
            this.setupFormValidation(form);
        });
    },

    setupFormValidation: function(form) {
        form.setAttribute('novalidate', 'novalidate');
        
        const inputs = form.querySelectorAll('input, select, textarea');
        inputs.forEach(input => {
            input.addEventListener('blur', () => {
                this.validateField(input);
            });
            
            input.addEventListener('input', () => {
                if (input.classList.contains('is-invalid')) {
                    this.validateField(input);
                }
            });
        });
        
        form.addEventListener('submit', (e) => {
            if (!this.validateFormFields(form)) {
                e.preventDefault();
                e.stopPropagation();
                
                const firstInvalid = form.querySelector('.is-invalid');
                if (firstInvalid) {
                    firstInvalid.focus();
                }
                
                showToast('Mohon perbaiki kesalahan pada form', 'error');
            } else {
                const submitBtn = form.querySelector('button[type="submit"]');
                if (submitBtn) {
                    showLoading(submitBtn);
                }
            }
        });
    },
    
    validateFormFields: function(form) {
        let isValid = true;
        form.querySelectorAll('input, select, textarea').forEach(input => {
            if (!this.validateField(input)) {
                isValid = false;
            }
        });
        return isValid;
    },
    
    validateField: function(field) {
        if (field.disabled || field.readOnly) {
            return true;
        }
        
        field.classList.remove('is-valid', 'is-invalid');
        this.clearFieldError(field);
        
        if (field.hasAttribute('required') && !field.value.trim()) {
            this.setFieldError(field, 'Field ini wajib diisi');
            return false;
        }
        
        if (field.hasAttribute('minlength')) {
            const minLength = parseInt(field.getAttribute('minlength'));
            if (field.value.length > 0 && field.value.length < minLength) {
                this.setFieldError(field, `Minimal ${minLength} karakter`);
                return false;
            }
        }
        
        if (field.hasAttribute('maxlength')) {
            const maxLength = parseInt(field.getAttribute('maxlength'));
            if (field.value.length > maxLength) {
                this.setFieldError(field, `Maksimal ${maxLength} karakter`);
                return false;
            }
        }
        
        if (field.hasAttribute('pattern') && field.value) {
            const pattern = new RegExp(field.getAttribute('pattern'));
            if (!pattern.test(field.value)) {
                const patternMsg = field.getAttribute('data-pattern-message') || 'Format tidak valid';
                this.setFieldError(field, patternMsg);
                return false;
            }
        }
        
        if (field.type === 'email' && field.value) {
            const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
            if (!emailPattern.test(field.value)) {
                this.setFieldError(field, 'Format email tidak valid');
                return false;
            }
        }
        
        if (field.type === 'number' && field.value) {
            const value = parseFloat(field.value);
            
            if (field.hasAttribute('min')) {
                const min = parseFloat(field.getAttribute('min'));
                if (value < min) {
                    this.setFieldError(field, `Nilai minimal ${min}`);
                    return false;
                }
            }
            
            if (field.hasAttribute('max')) {
                const max = parseFloat(field.getAttribute('max'));
                if (value > max) {
                    this.setFieldError(field, `Nilai maksimal ${max}`);
                    return false;
                }
            }
        }
        
        if (field.hasAttribute('data-validate-match')) {
            const matchFieldId = field.getAttribute('data-validate-match');
            const matchField = document.getElementById(matchFieldId);
            if (matchField && field.value !== matchField.value) {
                this.setFieldError(field, 'Password tidak cocok');
                return false;
            }
        }
        
        if (field.value.trim()) {
            field.classList.add('is-valid');
        }
        return true;
    },
    
    setFieldError: function(field, message) {
        field.classList.add('is-invalid');
        
        let feedback = field.parentElement.querySelector('.invalid-feedback');
        if (!feedback) {
            feedback = document.createElement('div');
            feedback.className = 'invalid-feedback';
            field.parentElement.appendChild(feedback);
        }
        feedback.textContent = message;
    },
    
    clearFieldError: function(field) {
        const feedback = field.parentElement.querySelector('.invalid-feedback');
        if (feedback) {
            feedback.remove();
        }
    }
};

FormValidator.init();

const BatchSelector = {
    activeTable: null,
    options: {},

    enable: function(table, opts) {
        if (this.activeTable) this.disable();
        this.activeTable = table;
        this.options = opts || {};

        table.querySelectorAll('.batch-col').forEach(function(el) {
            el.classList.remove('d-none');
        });

        var actionCols = table.querySelectorAll('.batch-action-col');
        if (actionCols.length) {
            table.querySelectorAll('tr').forEach(function(tr) {
                actionCols.forEach(function(ac) {
                    var idx = ac.cellIndex;
                    if (tr.children[idx]) tr.children[idx].classList.add('d-none');
                });
            });
        }

        this._showToolbar(table);

        var selectAll = table.querySelector('.batch-select-all');
        if (selectAll) {
            selectAll.addEventListener('change', this._onSelectAll.bind(this));
        }

        table.querySelectorAll('.batch-check').forEach(function(cb) {
            cb.addEventListener('change', this._updateToolbar.bind(this));
        }.bind(this));

        this._updateToolbar();
    },

    disable: function() {
        if (!this.activeTable) return;
        var table = this.activeTable;

        table.querySelectorAll('.batch-col').forEach(function(el) {
            el.classList.add('d-none');
        });

        var actionCols = table.querySelectorAll('.batch-action-col');
        if (actionCols.length) {
            table.querySelectorAll('tr').forEach(function(tr) {
                actionCols.forEach(function(ac) {
                    var idx = ac.cellIndex;
                    if (tr.children[idx]) tr.children[idx].classList.remove('d-none');
                });
            });
        }

        this._hideToolbar();

        table.querySelectorAll('.batch-check, .batch-select-all').forEach(function(cb) {
            cb.checked = false;
        });

        this.activeTable = null;
    },

    toggle: function(table, opts) {
        if (this.activeTable === table) {
            this.disable();
        } else {
            this.enable(table, opts);
        }
    },

    _onSelectAll: function(e) {
        var checked = e.target.checked;
        this.activeTable.querySelectorAll('.batch-check').forEach(function(cb) {
            cb.checked = checked;
        });
        this._updateToolbar();
    },

    _updateToolbar: function() {
        var table = this.activeTable;
        if (!table) return;
        var selected = table.querySelectorAll('.batch-check:checked').length;
        var total = table.querySelectorAll('.batch-check').length;
        var toolbar = document.getElementById('batchToolbar');
        if (!toolbar) return;

        var countEl = toolbar.querySelector('.batch-selected-count');
        if (countEl) countEl.textContent = selected;

        var deleteBtn = toolbar.querySelector('.batch-delete-btn');
        if (deleteBtn) deleteBtn.disabled = selected === 0;

        var selectAll = table.querySelector('.batch-select-all');
        if (selectAll) {
            selectAll.checked = selected > 0 && selected === total;
            selectAll.indeterminate = selected > 0 && selected < total;
        }
    },

    _showToolbar: function(table) {
        var existing = document.getElementById('batchToolbar');
        if (existing) {
            existing.classList.remove('d-none');
            return;
        }
        var toolbar = document.createElement('div');
        toolbar.id = 'batchToolbar';
        toolbar.className = 'd-flex align-items-center gap-2 mb-2 p-2 bg-light border rounded';
        toolbar.style.flexWrap = 'wrap';
        toolbar.innerHTML =
            '<span class="fw-semibold me-2"><i class="bi bi-check-square"></i> <span class="batch-selected-count">0</span> terpilih</span>' +
            '<button type="button" class="btn btn-sm btn-outline-secondary batch-select-all-btn"><i class="bi bi-check-all"></i> Pilih Semua</button>' +
            '<button type="button" class="btn btn-sm btn-outline-secondary batch-deselect-all-btn"><i class="bi bi-x"></i> Batal Semua</button>' +
            '<button type="button" class="btn btn-sm btn-danger batch-delete-btn" disabled><i class="bi bi-trash"></i> Hapus Terpilih</button>' +
            '<button type="button" class="btn btn-sm btn-link text-decoration-none batch-cancel-btn ms-auto">Batal Pilih</button>';

        table.parentElement.insertBefore(toolbar, table);

        var self = this;
        toolbar.querySelector('.batch-select-all-btn').addEventListener('click', function() {
            self.activeTable.querySelectorAll('.batch-check').forEach(function(cb) { cb.checked = true; });
            self._updateToolbar();
        });
        toolbar.querySelector('.batch-deselect-all-btn').addEventListener('click', function() {
            self.activeTable.querySelectorAll('.batch-check').forEach(function(cb) { cb.checked = false; });
            self._updateToolbar();
        });
        toolbar.querySelector('.batch-delete-btn').addEventListener('click', function() {
            self._deleteSelected();
        });
        toolbar.querySelector('.batch-cancel-btn').addEventListener('click', function() {
            self.disable();
        });
    },

    _hideToolbar: function() {
        var toolbar = document.getElementById('batchToolbar');
        if (toolbar) toolbar.classList.add('d-none');
    },

    _deleteSelected: function() {
        var table = this.activeTable;
        if (!table) return;
        var items = [];
        table.querySelectorAll('.batch-check:checked').forEach(function(cb) {
            var tr = cb.closest('tr');
            var id = tr.getAttribute('data-batch-id');
            if (!id) id = cb.value;
            var label = tr.getAttribute('data-batch-label') || id;
            items.push({ id: id, label: label });
        });
        if (items.length === 0) return;

        var url = table.getAttribute('data-batch-url') || this.options.batchUrl;
        if (!url) {
            showToast('URL batch delete tidak ditemukan', 'error');
            return;
        }

        confirmBatchDelete({ items: items, url: url });
    }
};

function showToast(message, type = 'info') {
    const toastContainer = document.getElementById('toastContainer');
    if (!toastContainer) {
        const container = document.createElement('div');
        container.id = 'toastContainer';
        container.className = 'toast-container position-fixed top-0 end-0 p-3';
        document.body.appendChild(container);
    }

    const toastId = 'toast-' + Date.now();
    const bgClass = type === 'success' ? 'bg-success' : type === 'error' ? 'bg-danger' : 'bg-info';
    
    const toastHTML = `
        <div id="${toastId}" class="toast ${bgClass} text-white" role="alert">
            <div class="toast-header ${bgClass} text-white">
                <strong class="me-auto">Notifikasi</strong>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="toast"></button>
            </div>
            <div class="toast-body">
                ${message}
            </div>
        </div>
    `;

    document.getElementById('toastContainer').insertAdjacentHTML('beforeend', toastHTML);
    const toastElement = document.getElementById(toastId);
    const toast = new bootstrap.Toast(toastElement);
    toast.show();

    toastElement.addEventListener('hidden.bs.toast', () => {
        toastElement.remove();
    });
}
