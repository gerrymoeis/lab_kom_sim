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
        console.warn('HEIC conversion failed, uploading original:', e);
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
        { camera: 'photo_front_camera', gallery: 'photo_front_gallery', type: 'front' }
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
    showLoadingState(type);

    try {
        // HEIC conversion (no-op on ANDROID=false since heic-to.js is not loaded)
        file = await heicToJpeg(file);

        if (window.ANDROID_MODE) {
            // Client-side: compress before upload
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
        } else {
            console.warn('Server upload warning:', result.message);
        }
    } catch (error) {
        showError(type, error.message);
    }
}

async function uploadForProcessing(file, type) {
    var formData = new FormData();
    formData.append('image', file);
    formData.append('type', type);

    var pcNumberInput = document.querySelector('input[name="pc_number"]');
    var pcNumber = pcNumberInput ? pcNumberInput.value : window.location.pathname.split('/')[2];
    if (pcNumber) formData.append('pc_number', pcNumber);

    var response = await fetch('/api/upload-image', { method: 'POST', body: formData });
    return await response.json();
}

function showLocalPreview(url, type) {
    var img = document.getElementById(type === 'serial' ? 'imagePreviewSerial' : 'imagePreviewFront');
    var area = document.getElementById('preview' + (type === 'serial' ? 'Serial' : 'Front'));
    if (img) img.src = url;
    if (area) area.classList.remove('d-none');
}

function storeFileReference(fileRef, type) {
    var id = type === 'serial' ? 'serial_file_ref' : 'front_file_ref';
    if (type === 'serial') serialFileRef = fileRef; else frontFileRef = fileRef;

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
}

function showLoadingState(type) {
    var area = document.getElementById('preview' + (type === 'serial' ? 'Serial' : 'Front'));
    var loader = document.getElementById('loading' + (type === 'serial' ? 'Serial' : 'Front'));
    if (loader) loader.classList.remove('d-none');
    if (area) area.classList.remove('d-none');
}

function showError(type, message) {
    var area = document.getElementById('preview' + (type === 'serial' ? 'Serial' : 'Front'));
    var errEl = document.getElementById('error' + (type === 'serial' ? 'Serial' : 'Front'));
    var loader = document.getElementById('loading' + (type === 'serial' ? 'Serial' : 'Front'));
    if (errEl) { errEl.textContent = message; errEl.classList.remove('d-none'); }
    if (loader) loader.classList.add('d-none');
    if (area) area.classList.remove('d-none');
    console.error('Photo upload error:', message);
}

function clearOtherInput(type, source) {
    if (type === 'serial') {
        var other = document.getElementById(source === 'camera' ? 'photo_serial_gallery' : 'photo_serial_camera');
        if (other) other.value = '';
    } else {
        var other = document.getElementById(source === 'camera' ? 'photo_front_gallery' : 'photo_front_camera');
        if (other) other.value = '';
    }
}

async function clearImage(type) {
    var fileRef = type === 'serial' ? serialFileRef : frontFileRef;
    if (type === 'serial') {
        serialFileRef = null;
        if (serialPreviewUrl) { URL.revokeObjectURL(serialPreviewUrl); serialPreviewUrl = null; }
    } else {
        frontFileRef = null;
        if (frontPreviewUrl) { URL.revokeObjectURL(frontPreviewUrl); frontPreviewUrl = null; }
    }

    var h = document.getElementById(type === 'serial' ? 'serial_file_ref' : 'front_file_ref');
    if (h) h.remove();

    var cameraInput = document.getElementById('photo_' + type + '_camera');
    var galleryInput = document.getElementById('photo_' + type + '_gallery');
    var previewArea = document.getElementById('preview' + (type === 'serial' ? 'Serial' : 'Front'));
    var img = document.getElementById(type === 'serial' ? 'imagePreviewSerial' : 'imagePreviewFront');
    var loader = document.getElementById('loading' + (type === 'serial' ? 'Serial' : 'Front'));
    var errEl = document.getElementById('error' + (type === 'serial' ? 'Serial' : 'Front'));

    if (cameraInput) cameraInput.value = '';
    if (galleryInput) galleryInput.value = '';
    if (img) img.src = '';
    if (previewArea) previewArea.classList.add('d-none');
    if (loader) loader.classList.add('d-none');
    if (errEl) errEl.classList.add('d-none');

    if (fileRef) {
        try {
            await fetch('/api/delete-temp-file', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
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
