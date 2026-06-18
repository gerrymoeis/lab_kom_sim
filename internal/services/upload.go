package services

import "path/filepath"

func UploadPath(baseUploadPath, lab, subDir, filename string) string {
	return filepath.Join(baseUploadPath, lab, subDir, filename)
}

func UploadDir(baseUploadPath, lab, subDir string) string {
	return filepath.Join(baseUploadPath, lab, subDir)
}
