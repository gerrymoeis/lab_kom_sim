// Shared image upload utility
// Handles HEIC conversion, instant local preview (URL.createObjectURL), API upload

async function heicToJpeg(file) {
    const ext = file.name.split('.').pop().toLowerCase();
    if (ext !== 'heic' && ext !== 'heif') return file;
    if (typeof HeicTo === 'undefined') return file;
    try {
        const blob = await HeicTo({ blob: file, type: 'image/jpeg', quality: 0.88 });
        return new File([blob], file.name.replace(/\.(heic|heif)$/i, '.jpg'),
            { type: 'image/jpeg', lastModified: Date.now() });
    } catch (e) {
        console.warn('HEIC conversion failed, uploading original:', e);
        return file;
    }
}

// --- PC-specific: serial & front photo handling ---

let serialFileRef = null;
let frontFileRef = null;
let serialPreviewUrl = null;
let frontPreviewUrl = null;

document.addEventListener('DOMContentLoaded', function () {
    setupFileHandlers();
});

function setupFileHandlers() {
    const pairs = [
        { camera: 'photo_serial_camera', gallery: 'photo_serial_gallery', type: 'serial' },
        { camera: 'photo_front_camera', gallery: 'photo_front_gallery', type: 'front' }
    ];
    for (const p of pairs) {
        const cam = document.getElementById(p.camera);
        const gal = document.getElementById(p.gallery);
        if (cam) cam.addEventListener('change', (e) => handleFileSelect(e.target.files[0], p.type, 'camera'));
        if (gal) gal.addEventListener('change', (e) => handleFileSelect(e.target.files[0], p.type, 'gallery'));
    }
}

async function handleFileSelect(file, type, source) {
    if (!file) return;
    showLoadingState(type);

    try {
        file = await heicToJpeg(file);

        // Instant local preview (works for all formats including converted HEIC)
        const previewUrl = URL.createObjectURL(file);
        if (type === 'serial') {
            if (serialPreviewUrl) URL.revokeObjectURL(serialPreviewUrl);
            serialPreviewUrl = previewUrl;
        } else {
            if (frontPreviewUrl) URL.revokeObjectURL(frontPreviewUrl);
            frontPreviewUrl = previewUrl;
        }
        showLocalPreview(previewUrl, type);
        clearOtherInput(type, source);

        // Background upload to server for processing + file_ref
        const result = await uploadForProcessing(file, type);
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
    const formData = new FormData();
    formData.append('image', file);
    formData.append('type', type);

    const pcNumberInput = document.querySelector('input[name="pc_number"]');
    const pcNumber = pcNumberInput ? pcNumberInput.value : window.location.pathname.split('/')[2];
    if (pcNumber) formData.append('pc_number', pcNumber);

    const response = await fetch('/api/upload-image', { method: 'POST', body: formData });
    return await response.json();
}

function showLocalPreview(url, type) {
    const img = document.getElementById(type === 'serial' ? 'imagePreviewSerial' : 'imagePreviewFront');
    const area = document.getElementById(`preview${type === 'serial' ? 'Serial' : 'Front'}`);
    if (img) img.src = url;
    if (area) area.classList.remove('d-none');
}

function storeFileReference(fileRef, type) {
    const id = type === 'serial' ? 'serial_file_ref' : 'front_file_ref';
    if (type === 'serial') serialFileRef = fileRef; else frontFileRef = fileRef;

    let hiddenInput = document.getElementById(id);
    if (!hiddenInput) {
        hiddenInput = document.createElement('input');
        hiddenInput.type = 'hidden';
        hiddenInput.id = id;
        hiddenInput.name = id;
        const form = document.querySelector('form');
        if (form) form.appendChild(hiddenInput);
    }
    hiddenInput.value = fileRef;
}

function showLoadingState(type) {
    const area = document.getElementById(`preview${type === 'serial' ? 'Serial' : 'Front'}`);
    const loader = document.getElementById(`loading${type === 'serial' ? 'Serial' : 'Front'}`);
    if (loader) loader.classList.remove('d-none');
    if (area) area.classList.remove('d-none');
}

function showError(type, message) {
    const area = document.getElementById(`preview${type === 'serial' ? 'Serial' : 'Front'}`);
    const errEl = document.getElementById(`error${type === 'serial' ? 'Serial' : 'Front'}`);
    const loader = document.getElementById(`loading${type === 'serial' ? 'Serial' : 'Front'}`);
    if (errEl) { errEl.textContent = message; errEl.classList.remove('d-none'); }
    if (loader) loader.classList.add('d-none');
    if (area) area.classList.remove('d-none');
    console.error('Photo upload error:', message);
}

function clearOtherInput(type, source) {
    if (type === 'serial') {
        const other = document.getElementById(source === 'camera' ? 'photo_serial_gallery' : 'photo_serial_camera');
        if (other) other.value = '';
    } else {
        const other = document.getElementById(source === 'camera' ? 'photo_front_gallery' : 'photo_front_camera');
        if (other) other.value = '';
    }
}

async function clearImage(type) {
    const fileRef = type === 'serial' ? serialFileRef : frontFileRef;
    if (type === 'serial') {
        serialFileRef = null;
        if (serialPreviewUrl) { URL.revokeObjectURL(serialPreviewUrl); serialPreviewUrl = null; }
    } else {
        frontFileRef = null;
        if (frontPreviewUrl) { URL.revokeObjectURL(frontPreviewUrl); frontPreviewUrl = null; }
    }

    const h = document.getElementById(type === 'serial' ? 'serial_file_ref' : 'front_file_ref');
    if (h) h.remove();

    const cameraInput = document.getElementById(`photo_${type}_camera`);
    const galleryInput = document.getElementById(`photo_${type}_gallery`);
    const previewArea = document.getElementById(`preview${type === 'serial' ? 'Serial' : 'Front'}`);
    const img = document.getElementById(type === 'serial' ? 'imagePreviewSerial' : 'imagePreviewFront');
    const loader = document.getElementById(`loading${type === 'serial' ? 'Serial' : 'Front'}`);
    const errEl = document.getElementById(`error${type === 'serial' ? 'Serial' : 'Front'}`);

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
