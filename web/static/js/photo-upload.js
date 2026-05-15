let serialFileRef = null;
let frontFileRef = null;

async function heicToJpeg(file) {
    const ext = file.name.split('.').pop().toLowerCase();
    if (ext !== 'heic' && ext !== 'heif') return file;
    if (typeof HeicTo === 'undefined') return file;

    try {
        const blob = await HeicTo({
            blob: file,
            type: 'image/jpeg',
            quality: 0.88,
        });
        return new File([blob],
            file.name.replace(/\.(heic|heif)$/i, '.jpg'),
            { type: 'image/jpeg', lastModified: Date.now() }
        );
    } catch (e) {
        console.warn('HEIC conversion failed, uploading original:', e);
        return file;
    }
}

document.addEventListener('DOMContentLoaded', function () {
    setupFileHandlers();
});

function setupFileHandlers() {
    const serialCamera = document.getElementById('photo_serial_camera');
    const serialGallery = document.getElementById('photo_serial_gallery');

    if (serialCamera) {
        serialCamera.addEventListener('change', (e) => handleFileSelect(e.target.files[0], 'serial', 'camera'));
    }
    if (serialGallery) {
        serialGallery.addEventListener('change', (e) => handleFileSelect(e.target.files[0], 'serial', 'gallery'));
    }

    const frontCamera = document.getElementById('photo_front_camera');
    const frontGallery = document.getElementById('photo_front_gallery');

    if (frontCamera) {
        frontCamera.addEventListener('change', (e) => handleFileSelect(e.target.files[0], 'front', 'camera'));
    }
    if (frontGallery) {
        frontGallery.addEventListener('change', (e) => handleFileSelect(e.target.files[0], 'front', 'gallery'));
    }
}

async function handleFileSelect(file, type, source) {
    if (!file) return;

    const ext = file.name.split('.').pop().toLowerCase();
    const isHeic = ext === 'heic' || ext === 'heif';
    showLoadingState(type, isHeic);

    try {
        file = await heicToJpeg(file);
        const result = await uploadForProcessing(file, type);

        if (result.success) {
            showPreview(result.preview_url, type);
            storeFileReference(result.file_ref, type);
            clearOtherInput(type, source);
        } else {
            throw new Error(result.message);
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
    if (pcNumber) {
        formData.append('pc_number', pcNumber);
    }

    const response = await fetch('/api/upload-image', {
        method: 'POST',
        body: formData
    });

    return await response.json();
}

function showPreview(previewUrl, type) {
    const area = document.getElementById(`preview${type === 'serial' ? 'Serial' : 'Front'}`);
    if (area) {
        area.innerHTML = `
            <div class="border rounded-3 p-4 bg-light text-center">
                <img id="imagePreview${type === 'serial' ? 'Serial' : 'Front'}"
                     src="${previewUrl}"
                     alt="Preview"
                     class="img-fluid rounded shadow-sm"
                     style="max-width: 100%; max-height: 400px; object-fit: contain; display: block; margin: 0 auto;">
                <div class="mt-3">
                    <button type="button" class="btn btn-sm btn-outline-danger" onclick="clearImage('${type}')">
                        <i class="bi bi-x-circle me-1"></i> Hapus
                    </button>
                </div>
                <small class="text-muted d-block mt-2">
                    <i class="bi bi-check-circle text-success me-1"></i>
                    File berhasil diproses dan siap untuk disimpan
                </small>
            </div>
        `;
        area.classList.remove('d-none');
    }
}

function storeFileReference(fileRef, type) {
    const id = type === 'serial' ? 'serial_file_ref' : 'front_file_ref';
    if (type === 'serial') { serialFileRef = fileRef } else { frontFileRef = fileRef }

    let hiddenInput = document.getElementById(id);
    if (!hiddenInput) {
        hiddenInput = document.createElement('input');
        hiddenInput.type = 'hidden';
        hiddenInput.id = id;
        hiddenInput.name = id;
        document.querySelector('form').appendChild(hiddenInput);
    }
    hiddenInput.value = fileRef;
}

function showLoadingState(type, isHeic) {
    const area = document.getElementById(`preview${type === 'serial' ? 'Serial' : 'Front'}`);
    const msg = isHeic ? 'Mengkonversi HEIC ke JPEG...' : 'Memproses gambar...';
    if (area) {
        area.innerHTML = `
            <div class="text-center p-4">
                <div class="spinner-border text-primary" role="status">
                    <span class="visually-hidden">Loading...</span>
                </div>
                <div class="mt-2 small text-muted">${msg}</div>
            </div>
        `;
        area.classList.remove('d-none');
    }
}

function showError(type, message) {
    const area = document.getElementById(`preview${type === 'serial' ? 'Serial' : 'Front'}`);
    if (area) {
        area.innerHTML = `
            <div class="alert alert-danger text-center">
                <i class="bi bi-exclamation-triangle"></i>
                <div class="mt-1 small">${message}</div>
            </div>
        `;
        area.classList.remove('d-none');
    }
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
        const h = document.getElementById('serial_file_ref');
        if (h) h.remove();
    } else {
        frontFileRef = null;
        const h = document.getElementById('front_file_ref');
        if (h) h.remove();
    }

    const cameraInput = document.getElementById(`photo_${type}_camera`);
    const galleryInput = document.getElementById(`photo_${type}_gallery`);
    const previewArea = document.getElementById(`preview${type === 'serial' ? 'Serial' : 'Front'}`);

    if (cameraInput) cameraInput.value = '';
    if (galleryInput) galleryInput.value = '';
    if (previewArea) previewArea.classList.add('d-none');

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
