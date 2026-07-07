package services

import (
	"log"
	"os"
	"path/filepath"
)

// PromoteFile memindahkan file dari temp/ ke subdirektori permanen.
// Jika oldFilename tidak kosong dan berbeda dari ref, old file akan dihapus.
// Return filename (ref) jika sukses, atau ("", nil) jika ref invalid/kosong.
func PromoteFile(uploadPath, lab, fileRef, subDir, oldFilename string) (string, error) {
	ref := filepath.Base(fileRef)
	if ref == "" || ref == "." || ref == "/" || ref == "\\" {
		return "", nil
	}

	src := filepath.Join(uploadPath, lab, "temp", ref)
	dstDir := filepath.Join(uploadPath, lab, subDir)
	dst := filepath.Join(dstDir, ref)

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		log.Printf("WARN: PromoteFile: failed to create dir for %s/%s: %v", subDir, ref, err)
		return "", err
	}

	if err := CopyFile(src, dst); err != nil {
		log.Printf("WARN: PromoteFile: copy failed %s -> %s: %v", src, dst, err)
		return "", err
	}

	os.Remove(src)

	if oldFilename != "" && oldFilename != ref {
		oldPath := filepath.Join(dstDir, oldFilename)
		os.Remove(oldPath)
	}

	return ref, nil
}

// DeleteFile menghapus file dari subdirektori permanen.
func DeleteFile(uploadPath, lab, subDir, filename string) error {
	if filename == "" {
		return nil
	}
	ref := filepath.Base(filename)
	if ref == "" || ref == "." || ref == "/" || ref == "\\" {
		return nil
	}
	return os.Remove(filepath.Join(uploadPath, lab, subDir, ref))
}

// DeleteTempFile menghapus file dari direktori temp.
func DeleteTempFile(uploadPath, lab, fileRef string) error {
	ref := filepath.Base(fileRef)
	if ref == "" || ref == "." || ref == "/" || ref == "\\" {
		return nil
	}
	return os.Remove(filepath.Join(uploadPath, lab, "temp", ref))
}
