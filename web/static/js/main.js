// Main JavaScript for Inventaris Lab Kom

// Initialize tooltips
document.addEventListener('DOMContentLoaded', function() {
    // Bootstrap tooltips
    var tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
    var tooltipList = tooltipTriggerList.map(function (tooltipTriggerEl) {
        return new bootstrap.Tooltip(tooltipTriggerEl);
    });

    // Auto-dismiss alerts after 5 seconds
    const alerts = document.querySelectorAll('.alert:not(.alert-permanent)');
    alerts.forEach(alert => {
        setTimeout(() => {
            const bsAlert = new bootstrap.Alert(alert);
            bsAlert.close();
        }, 5000);
    });
});

// Confirm delete actions
function confirmDelete(message) {
    return confirm(message || 'Apakah Anda yakin ingin menghapus data ini?');
}

// Show loading spinner
function showLoading(button) {
    const originalText = button.innerHTML;
    button.disabled = true;
    button.setAttribute('aria-busy', 'true');
    button.innerHTML = '<span class="spinner-border spinner-border-sm me-2" role="status" aria-hidden="true"></span>Loading...';
    button.dataset.originalText = originalText;
}

// Hide loading spinner
function hideLoading(button) {
    button.disabled = false;
    button.removeAttribute('aria-busy');
    button.innerHTML = button.dataset.originalText;
}

// Format date to Indonesian format
function formatDate(dateString) {
    if (!dateString) return '-';
    const date = new Date(dateString);
    const options = { year: 'numeric', month: 'long', day: 'numeric' };
    return date.toLocaleDateString('id-ID', options);
}

// Format time
function formatTime(timeString) {
    if (!timeString) return '-';
    return timeString;
}

// Copy to clipboard
function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(() => {
        alert('Berhasil disalin ke clipboard!');
    }).catch(err => {
        console.error('Gagal menyalin:', err);
    });
}

// Print page
function printPage() {
    window.print();
}

// Export table to CSV
function exportTableToCSV(tableId, filename) {
    const table = document.getElementById(tableId);
    if (!table) return;

    let csv = [];
    const rows = table.querySelectorAll('tr');

    for (let i = 0; i < rows.length; i++) {
        const row = [];
        const cols = rows[i].querySelectorAll('td, th');

        for (let j = 0; j < cols.length; j++) {
            row.push(cols[j].innerText);
        }

        csv.push(row.join(','));
    }

    downloadCSV(csv.join('\n'), filename);
}

function downloadCSV(csv, filename) {
    const csvFile = new Blob([csv], { type: 'text/csv' });
    const downloadLink = document.createElement('a');
    downloadLink.download = filename;
    downloadLink.href = window.URL.createObjectURL(csvFile);
    downloadLink.style.display = 'none';
    document.body.appendChild(downloadLink);
    downloadLink.click();
    document.body.removeChild(downloadLink);
}

// Search/Filter table
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

// Validate form
function validateForm(formId) {
    const form = document.getElementById(formId);
    if (!form) return false;

    if (!form.checkValidity()) {
        form.classList.add('was-validated');
        return false;
    }

    return true;
}

// Enhanced Form Validation System
const FormValidator = {
    // Initialize validation for all forms
    init: function() {
        document.addEventListener('DOMContentLoaded', () => {
            const forms = document.querySelectorAll('form[data-validate="true"]');
            forms.forEach(form => {
                this.setupFormValidation(form);
            });
        });
    },
    
    // Setup validation for a specific form
    setupFormValidation: function(form) {
        // Prevent default HTML5 validation bubbles
        form.setAttribute('novalidate', 'novalidate');
        
        // Add real-time validation on input
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
        
        // Validate on submit
        form.addEventListener('submit', (e) => {
            if (!this.validateFormFields(form)) {
                e.preventDefault();
                e.stopPropagation();
                
                // Focus on first invalid field
                const firstInvalid = form.querySelector('.is-invalid');
                if (firstInvalid) {
                    firstInvalid.focus();
                }
                
                // Show error toast
                showToast('Mohon perbaiki kesalahan pada form', 'error');
            } else {
                // Show loading state on submit button
                const submitBtn = form.querySelector('button[type="submit"]');
                if (submitBtn) {
                    showLoading(submitBtn);
                }
            }
        });
    },
    
    // Validate all fields in a form
    validateFormFields: function(form) {
        let isValid = true;
        const inputs = form.querySelectorAll('input, select, textarea');
        
        inputs.forEach(input => {
            if (!this.validateField(input)) {
                isValid = false;
            }
        });
        
        return isValid;
    },
    
    // Validate a single field
    validateField: function(field) {
        // Skip if field is disabled or readonly
        if (field.disabled || field.readOnly) {
            return true;
        }
        
        // Remove previous validation state
        field.classList.remove('is-valid', 'is-invalid');
        this.clearFieldError(field);
        
        // Check required
        if (field.hasAttribute('required') && !field.value.trim()) {
            this.setFieldError(field, 'Field ini wajib diisi');
            return false;
        }
        
        // Check minlength
        if (field.hasAttribute('minlength')) {
            const minLength = parseInt(field.getAttribute('minlength'));
            if (field.value.length > 0 && field.value.length < minLength) {
                this.setFieldError(field, `Minimal ${minLength} karakter`);
                return false;
            }
        }
        
        // Check maxlength
        if (field.hasAttribute('maxlength')) {
            const maxLength = parseInt(field.getAttribute('maxlength'));
            if (field.value.length > maxLength) {
                this.setFieldError(field, `Maksimal ${maxLength} karakter`);
                return false;
            }
        }
        
        // Check pattern
        if (field.hasAttribute('pattern') && field.value) {
            const pattern = new RegExp(field.getAttribute('pattern'));
            if (!pattern.test(field.value)) {
                const patternMsg = field.getAttribute('data-pattern-message') || 'Format tidak valid';
                this.setFieldError(field, patternMsg);
                return false;
            }
        }
        
        // Check email
        if (field.type === 'email' && field.value) {
            const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
            if (!emailPattern.test(field.value)) {
                this.setFieldError(field, 'Format email tidak valid');
                return false;
            }
        }
        
        // Check number
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
        
        // Check custom validation
        if (field.hasAttribute('data-validate-match')) {
            const matchFieldId = field.getAttribute('data-validate-match');
            const matchField = document.getElementById(matchFieldId);
            if (matchField && field.value !== matchField.value) {
                this.setFieldError(field, 'Password tidak cocok');
                return false;
            }
        }
        
        // Field is valid
        if (field.value.trim()) {
            field.classList.add('is-valid');
        }
        return true;
    },
    
    // Set field error
    setFieldError: function(field, message) {
        field.classList.add('is-invalid');
        
        // Create or update error message
        let feedback = field.parentElement.querySelector('.invalid-feedback');
        if (!feedback) {
            feedback = document.createElement('div');
            feedback.className = 'invalid-feedback';
            field.parentElement.appendChild(feedback);
        }
        feedback.textContent = message;
    },
    
    // Clear field error
    clearFieldError: function(field) {
        const feedback = field.parentElement.querySelector('.invalid-feedback');
        if (feedback) {
            feedback.remove();
        }
    }
};

// Initialize form validation system
FormValidator.init();

// Show toast notification
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
