package cleanup

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func Run() {
	root := "."

	targets := []struct {
		path  string
		isDir bool
		desc  string
	}{
		{path: "inventaris_lab.db", desc: "old single-lab DB (root)"},
		{path: "inventaris_lab.db-shm", desc: "old single-lab DB WAL (root)"},
		{path: "inventaris_lab.db-wal", desc: "old single-lab DB WAL (root)"},
		{path: "testsum.exe", desc: "pre-built test binary (root)"},
		{path: "dist", isDir: true, desc: "public build output"},
		{path: "bin", isDir: true, desc: "pre-built binaries"},
	}

	for _, t := range targets {
		fullPath := filepath.Join(root, t.path)
		if t.isDir {
			if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
				if err := os.RemoveAll(fullPath); err != nil {
					log.Printf("Cleanup: failed to remove %s [%s]: %v", t.path, t.desc, err)
				} else {
					log.Printf("Cleanup: removed %s [%s]", t.path, t.desc)
				}
			}
		} else {
			if err := os.Remove(fullPath); err == nil {
				log.Printf("Cleanup: removed %s [%s]", t.path, t.desc)
			} else if !os.IsNotExist(err) {
				log.Printf("Cleanup: failed to remove %s [%s]: %v", t.path, t.desc, err)
			}
		}
	}

	cleanPlaceholderDir(root)
	cleanEmptyBackupDirs(root)
	cleanTempUploads(root)
	cleanScriptsTestsum(root)
}

func cleanPlaceholderDir(root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "# Absolute path") && e.IsDir() {
			fullPath := filepath.Join(root, e.Name())
			if err := os.RemoveAll(fullPath); err != nil {
				log.Printf("Cleanup: failed to remove placeholder dir %s: %v", e.Name(), err)
			} else {
				log.Printf("Cleanup: removed placeholder dir [%s]", e.Name())
			}
			return
		}
	}
}

func cleanEmptyBackupDirs(root string) {
	backupRoot := filepath.Join(root, "backups")
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirPath := filepath.Join(backupRoot, e.Name())
		empty, _ := isEmptyDir(dirPath)
		if empty {
			if err := os.Remove(dirPath); err != nil {
				log.Printf("Cleanup: failed to remove empty backup dir %s: %v", e.Name(), err)
			} else {
				log.Printf("Cleanup: removed empty backup dir [backups/%s]", e.Name())
			}
		}
	}
}

func cleanTempUploads(root string) {
	uploadRoot := filepath.Join(root, "uploads")
	entries, err := os.ReadDir(uploadRoot)
	if err != nil {
		return
	}
	for _, lab := range entries {
		if !lab.IsDir() {
			continue
		}
		tempDir := filepath.Join(uploadRoot, lab.Name(), "temp")
		if info, err := os.Stat(tempDir); err == nil && info.IsDir() {
			if err := os.RemoveAll(tempDir); err != nil {
				log.Printf("Cleanup: failed to remove temp uploads [%s]: %v", filepath.Join("uploads", lab.Name(), "temp"), err)
			} else {
				log.Printf("Cleanup: removed temp uploads [uploads/%s/temp]", lab.Name())
			}
		}
	}
}

func cleanScriptsTestsum(root string) {
	path := filepath.Join(root, "scripts", "testsum", "testsum.exe")
	if err := os.Remove(path); err == nil {
		log.Printf("Cleanup: removed [scripts/testsum/testsum.exe]")
	} else if !os.IsNotExist(err) {
		log.Printf("Cleanup: failed to remove scripts/testsum/testsum.exe: %v", err)
	}
}

func isEmptyDir(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	return err != nil, nil
}
