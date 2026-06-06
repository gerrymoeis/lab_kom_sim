// Shared image upload utility
// Dual-mode: ANDROID_MODE=true → client-side compress, ANDROID_MODE=false → server-side compress

async function heicToJpeg(file) {
    var ext = file.name.split('.').pop().toLowerCase();
    if (ext !== 'heic' && ext !== 'heif') return file;
    if (typeof HeicTo === 'undefined') return file;
    try {
        var blob = await HeicTo({ blob: file, type: 'image/jpeg', quality: 0.88 });
        return new File([blob], file.name.replace(/\.(heic|heif)$/i, '.jpg'),
            { type: 'image/jpeg', lastModified: Date.now() });
    } catch (e) {
        return file;
    }
}

// compressImage resizes and re-encodes image to JPEG via canvas
async function compressImage(file, maxDimension, quality) {
    return new Promise(function (resolve, reject) {
        var img = new Image();
        img.onload = function () {
            var width = img.width;
            var height = img.height;
            if (width > maxDimension || height > maxDimension) {
                var ratio = Math.min(maxDimension / width, maxDimension / height);
                width = Math.round(width * ratio);
                height = Math.round(height * ratio);
            }
            var canvas = document.createElement('canvas');
            canvas.width = width;
            canvas.height = height;
            var ctx = canvas.getContext('2d');
            ctx.drawImage(img, 0, 0, width, height);
            canvas.toBlob(function (blob) {
                if (!blob) { reject(new Error('Gagal kompresi gambar')); return; }
                var fileName = file.name.replace(/\.[^.]+$/, '.jpg');
                resolve(new File([blob], fileName, { type: 'image/jpeg', lastModified: Date.now() }));
            }, 'image/jpeg', quality);
        };
        img.onerror = function () { reject(new Error('Gagal memuat gambar')); };
        img.src = URL.createObjectURL(file);
    });
}

// getMaxDim returns the max dimension based on photo type
function getMaxDim(type) {
    return type === 'front' ? 1920 : 1280;
}

// --- PC-specific: serial & front photo handling ---

var serialFileRef = null;
var frontFileRef = null;
var serialPreviewUrl = null;
var frontPreviewUrl = null;

document.addEventListener('DOMContentLoaded', function () {
    setupFileHandlers();
});

function setupFileHandlers() {
    var pairs = [
        { camera: 'photo_serial_camera', gallery: 'photo_serial_gallery', type: 'serial' },
        { camera: 'photo_front_camera', gallery: 'photo_front_gallery', type: 'front' },
        { camera: 'photo_logbook_camera', gallery: 'photo_logbook_gallery', type: 'logbook' }
    ];
    for (var i = 0; i < pairs.length; i++) {
        var p = pairs[i];
        var cam = document.getElementById(p.camera);
        var gal = document.getElementById(p.gallery);
        if (cam) cam.addEventListener('change', function (p) { return function (e) { handleFileSelect(e.target.files[0], p.type, 'camera'); }; }(p));
        if (gal) gal.addEventListener('change', function (p) { return function (e) { handleFileSelect(e.target.files[0], p.type, 'gallery'); }; }(p));
    }
}

async function handleFileSelect(file, type, source) {
    if (!file) return;
    if (file.size > 5 * 1024 * 1024) {
        showError(type, 'Ukuran file maksimal 5MB');
        return;
    }
    showLoadingState(type);

    try {
        file = await heicToJpeg(file);

        if (window.ANDROID_MODE) {
            file = await compressImage(file, getMaxDim(type), 0.75);
        }

        // Local preview
        var previewUrl = URL.createObjectURL(file);
        if (type === 'serial') {
            if (serialPreviewUrl) URL.revokeObjectURL(serialPreviewUrl);
            serialPreviewUrl = previewUrl;
        } else {
            if (frontPreviewUrl) URL.revokeObjectURL(frontPreviewUrl);
            frontPreviewUrl = previewUrl;
        }
        showLocalPreview(previewUrl, type);
        clearOtherInput(type, source);

        // Upload to server
        var result = await uploadForProcessing(file, type);
        if (result.success) {
            storeFileReference(result.file_ref, type);
            // Clear file input agar tidak dikirim ulang saat form submit
            document.getElementById('photo_' + type + '_' + source).value = '';
        }
    } catch (error) {
        console.error('Photo upload error:', error.message);
        showError(type, error.message);
    }
}

async function uploadForProcessing(file, type) {
    var formData = new FormData();
    formData.append('image', file);
    formData.append('type', type);

    var labelInput = document.querySelector('input[name="label"]');
    var label = labelInput ? labelInput.value : window.location.pathname.split('/')[2];
    if (label) { formData.append('label', label); }

    var response = await fetchWithCSRF('/api/upload-image', {
        method: 'POST',
        body: formData
    });
    var json = await response.json();
    return json;
}

function resolveSuffix(type) {
    if (type === 'serial') return 'Serial';
    if (type === 'front') return 'Front';
    return 'Logbook';
}

function showLocalPreview(url, type) {
    var sfx = resolveSuffix(type);
    var img = document.getElementById('imagePreview' + sfx);
    var area = document.getElementById('preview' + sfx);
    var loader = document.getElementById('loading' + sfx);
    if (img) { img.src = url; img.style.display = ''; }
    if (area) area.classList.remove('d-none');
    if (loader) loader.classList.add('d-none');
}

function storeFileReference(fileRef, type) {
    var sfx = resolveSuffix(type);
    var id = (type === 'serial' ? 'serial_file_ref' : type === 'front' ? 'front_file_ref' : 'logbook_file_ref');
    if (type === 'serial') serialFileRef = fileRef;
    else if (type === 'front') frontFileRef = fileRef;

    var hiddenInput = document.getElementById(id);
    if (!hiddenInput) {
        hiddenInput = document.createElement('input');
        hiddenInput.type = 'hidden';
        hiddenInput.id = id;
        hiddenInput.name = id;
        var form = document.querySelector('form');
        if (form) form.appendChild(hiddenInput);
    }
    hiddenInput.value = fileRef;

    var submitBtn = document.getElementById('submitBtn');
    if (submitBtn) submitBtn.disabled = false;
}

function showLoadingState(type) {
    var sfx = resolveSuffix(type);
    var area = document.getElementById('preview' + sfx);
    var loader = document.getElementById('loading' + sfx);
    var img = document.getElementById('imagePreview' + sfx);
    if (area) area.classList.remove('d-none');
    if (loader) loader.classList.remove('d-none');
    if (img) img.style.display = 'none';
}

function showError(type, message) {
    var sfx = resolveSuffix(type);
    var area = document.getElementById('preview' + sfx);
    var errEl = document.getElementById('error' + sfx);
    var loader = document.getElementById('loading' + sfx);
    if (errEl) { errEl.textContent = message; errEl.classList.remove('d-none'); }
    if (loader) loader.classList.add('d-none');
    if (area) area.classList.remove('d-none');
}

function clearOtherInput(type, source) {
    var other = document.getElementById('photo_' + type + '_' + (source === 'camera' ? 'gallery' : 'camera'));
    if (other) other.value = '';
}

async function clearImage(type) {
    var fileRef = type === 'serial' ? serialFileRef : type === 'front' ? frontFileRef : null;
    if (type === 'serial') { serialFileRef = null; if (serialPreviewUrl) { URL.revokeObjectURL(serialPreviewUrl); serialPreviewUrl = null; } }
    else if (type === 'front') { frontFileRef = null; if (frontPreviewUrl) { URL.revokeObjectURL(frontPreviewUrl); frontPreviewUrl = null; } }

    var h = document.getElementById(type === 'serial' ? 'serial_file_ref' : type === 'front' ? 'front_file_ref' : 'logbook_file_ref');
    if (h) h.remove();

    var cameraInput = document.getElementById('photo_' + type + '_camera');
    var galleryInput = document.getElementById('photo_' + type + '_gallery');
    var sfx = resolveSuffix(type);
    var previewArea = document.getElementById('preview' + sfx);
    var img = document.getElementById('imagePreview' + sfx);
    var loader = document.getElementById('loading' + sfx);
    var errEl = document.getElementById('error' + sfx);

    if (cameraInput) cameraInput.value = '';
    if (galleryInput) galleryInput.value = '';
    if (img) img.src = '';
    if (previewArea) previewArea.classList.add('d-none');
    if (loader) loader.classList.add('d-none');
    if (errEl) errEl.classList.add('d-none');

    var submitBtn = document.getElementById('submitBtn');
    if (submitBtn) submitBtn.disabled = true;

    if (fileRef) {
        try {
            await fetchWithCSRF('/api/delete-temp-file', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ file_ref: fileRef })
            });
        } catch (error) { }
    }
}

// Cleanup blob URLs on page unload
window.addEventListener('beforeunload', function () {
    if (serialPreviewUrl) URL.revokeObjectURL(serialPreviewUrl);
    if (frontPreviewUrl) URL.revokeObjectURL(frontPreviewUrl);
});
