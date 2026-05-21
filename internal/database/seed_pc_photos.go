package database

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

func seedPCPhotos(db *DB) error {
	releaseURL := os.Getenv("PC_PHOTO_RELEASE_URL")
	githubToken := os.Getenv("GITHUB_TOKEN")
	if releaseURL == "" || githubToken == "" {
		return nil
	}

	var pcCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM pcs").Scan(&pcCount); err != nil {
		return fmt.Errorf("seedPCPhotos: failed to check pc count: %w", err)
	}
	if pcCount == 0 {
		return nil
	}

	var photoCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pcs WHERE photo_serial IS NOT NULL AND photo_serial != ''`).Scan(&photoCount); err != nil {
		return fmt.Errorf("seedPCPhotos: failed to check photo count: %w", err)
	}
	if photoCount > 0 {
		return nil
	}

	uploadPath := os.Getenv("UPLOAD_PATH")
	if uploadPath == "" {
		uploadPath = "uploads"
	}
	pcDir := filepath.Join(uploadPath, "pc")
	if err := os.MkdirAll(pcDir, 0755); err != nil {
		return fmt.Errorf("seedPCPhotos: failed to create pc dir: %w", err)
	}

	entries, err := downloadAndExtractPhotos(releaseURL, githubToken, pcDir)
	if err != nil {
		fmt.Printf("WARN: PC photo seeding skipped: %v\n", err)
		return nil
	}

	if len(entries) == 0 {
		fmt.Println("WARN: PC photo seeding skipped: no matching photo files found in zip")
		return nil
	}

	serialMap := map[int]string{}
	frontMap := map[int]string{}
	allNums := map[int]bool{}
	for _, e := range entries {
		allNums[e.pcNum] = true
		if e.dbCol == "photo_serial" {
			serialMap[e.pcNum] = e.fileName
		} else {
			frontMap[e.pcNum] = e.fileName
		}
	}

	updated := 0
	for num := range allNums {
		serial := serialMap[num]
		front := frontMap[num]
		result, err := db.Exec(`UPDATE pcs SET photo_serial=COALESCE(NULLIF(?, ''), photo_serial), photo_front=COALESCE(NULLIF(?, ''), photo_front) WHERE pc_number=?`, serial, front, num)
		if err != nil {
			return fmt.Errorf("seedPCPhotos: failed to update pc %d: %w", num, err)
		}
		if n, _ := result.RowsAffected(); n > 0 {
			updated++
		}
	}

	fmt.Printf("Seeded PC photos: %d files extracted, %d PCs updated\n", len(entries), updated)
	return nil
}

type photoEntry struct {
	pcNum    int
	dbCol    string
	fileName string
}

func downloadAndExtractPhotos(releaseURL, githubToken, pcDir string) ([]photoEntry, error) {
	tmpFile := filepath.Join(os.TempDir(), "pc_photos_"+strconv.FormatInt(time.Now().UnixNano(), 36)+".zip")
	defer os.Remove(tmpFile)

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", releaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+githubToken)
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned %d (check PC_PHOTO_RELEASE_URL and GITHUB_TOKEN)", resp.StatusCode)
	}

	out, err := os.Create(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return nil, fmt.Errorf("failed to save zip: %w", err)
	}
	out.Close()

	re := regexp.MustCompile(`^(\d+)_(sn|full)\.jpeg$`)
	var entries []photoEntry

	reader, err := zip.OpenReader(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}
	defer reader.Close()

	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		matches := re.FindStringSubmatch(base)
		if matches == nil {
			continue
		}
		pcNum, _ := strconv.Atoi(matches[1])
		dbCol := "photo_serial"
		if matches[2] == "full" {
			dbCol = "photo_front"
		}

		dstPath := filepath.Join(pcDir, base)
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open %s in zip: %w", f.Name, err)
		}
		dst, err := os.Create(dstPath)
		if err != nil {
			rc.Close()
			return nil, fmt.Errorf("failed to create %s: %w", dstPath, err)
		}
		if _, err := io.Copy(dst, rc); err != nil {
			rc.Close()
			dst.Close()
			return nil, fmt.Errorf("failed to write %s: %w", dstPath, err)
		}
		rc.Close()
		dst.Close()

		entries = append(entries, photoEntry{
			pcNum:    pcNum,
			dbCol:    dbCol,
			fileName: base,
		})
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no files matching pattern pc{N}_{sn,full}.jpeg in zip")
	}

	return entries, nil
}
