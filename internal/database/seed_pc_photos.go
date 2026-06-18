package database

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

func seedPCPhotos(db *DB, urlPath string) error {
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

	// Check if photo files already exist on disk (more reliable than DB-only check)
	uploadPath := os.Getenv("UPLOAD_PATH")
	if uploadPath == "" {
		uploadPath = "uploads"
	}
	pcDir := filepath.Join(uploadPath, urlPath, "pc")
	if fi, err := os.Stat(filepath.Join(pcDir, "1_serial.jpeg")); err == nil && fi.Size() > 0 {
		return nil
	}
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
		label := fmt.Sprintf("pc-%d", num)
		result, err := db.Exec(`UPDATE pcs SET photo_serial=COALESCE(NULLIF(?, ''), photo_serial), photo_front=COALESCE(NULLIF(?, ''), photo_front) WHERE label=?`, serial, front, label)
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
	tmpFile, err := downloadReleaseAsset(releaseURL, githubToken)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile)

	re := regexp.MustCompile(`^(\d+)_(sn|full|serial|front)\.jpeg$`)
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
		if matches[2] == "full" || matches[2] == "front" {
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
		return nil, fmt.Errorf("no files matching pattern {N}_{sn,full,serial,front}.jpeg in zip")
	}

	return entries, nil
}

func downloadReleaseAsset(releaseURL, token string) (string, error) {
	re := regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+)/releases/download/([^/]+)/([^/]+)$`)
	matches := re.FindStringSubmatch(releaseURL)
	if matches == nil {
		return "", fmt.Errorf("invalid GitHub release URL: expected https://github.com/owner/repo/releases/download/tag/filename")
	}
	owner, repo, tag, filename := matches[1], matches[2], matches[3], matches[4]

	client := &http.Client{Timeout: 300 * time.Second}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, repo, tag)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create API request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d (check PC_PHOTO_RELEASE_URL and GITHUB_TOKEN)", resp.StatusCode)
	}

	var release struct {
		Assets []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse release JSON: %w", err)
	}

	var assetID int
	for _, a := range release.Assets {
		if a.Name == filename {
			assetID = a.ID
			break
		}
	}
	if assetID == 0 {
		return "", fmt.Errorf("asset '%s' not found in release '%s/%s' tag '%s'", filename, owner, repo, tag)
	}

	dlURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/assets/%d", owner, repo, assetID)
	req, err = http.NewRequest("GET", dlURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/octet-stream")

	resp, err = client.Do(req)
	if err != nil {
		return "", fmt.Errorf("asset download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		return "", fmt.Errorf("asset download returned %d", resp.StatusCode)
	}

	tmpPath := filepath.Join(os.TempDir(), "pc_photos_"+strconv.FormatInt(time.Now().UnixNano(), 36)+".zip")
	out, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to save asset: %w", err)
	}

	return tmpPath, nil
}
