package database

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"inventaris-lab-kom/internal/timeutil"
)

func seedPCPhotos(db *DB, uploadPath, urlPath string) error {
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

	pcDir := filepath.Join(uploadPath, urlPath, "pc")
	// Check if seed photos already exist using standardized naming (pc-{num}_{type}_{date}.jpeg)
	marker := filepath.Join(pcDir, ".seed_done")
	if fi, err := os.Stat(marker); err == nil && fi.Size() >= 0 {
		return nil
	}
	if err := os.MkdirAll(pcDir, 0755); err != nil {
		return fmt.Errorf("seedPCPhotos: failed to create pc dir: %w", err)
	}

	entries, err := downloadAndExtractPhotos(releaseURL, githubToken, pcDir)
	if err != nil {
		fmt.Printf("WARN: PC photo seeding skipped: %v. Periksa koneksi internet dan DNS server.\n", err)
		return nil
	}

	if len(entries) == 0 {
		fmt.Println("WARN: PC photo seeding skipped: no matching photo files found in zip")
		return nil
	}

	dateStr := timeutil.Now().Format("020106") // DDMMYY — same format as UploadImage

	serialMap := map[int]string{}
	frontMap := map[int]string{}
	allNums := map[int]bool{}

	// Rename extracted files to standardized naming: pc-{num}_{type}_{date}.jpeg
	for _, e := range entries {
		allNums[e.pcNum] = true
		oldPath := filepath.Join(pcDir, e.fileName)
		label := fmt.Sprintf("pc-%d", e.pcNum)
		newName := fmt.Sprintf("%s_%s_%s.jpeg", label, e.dbColSuffix, dateStr)
		newPath := filepath.Join(pcDir, newName)

		if err := os.Rename(oldPath, newPath); err != nil {
			// If rename fails (e.g., cross-device), copy+delete
			if cpErr := copyFile(oldPath, newPath); cpErr != nil {
				fmt.Printf("WARN: seedPCPhotos: failed to rename %s -> %s: %v\n", oldPath, newPath, err)
				continue
			}
			os.Remove(oldPath)
		}

		if e.dbCol == "photo_serial" {
			serialMap[e.pcNum] = newName
		} else {
			frontMap[e.pcNum] = newName
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

	// Write marker file
	os.WriteFile(marker, []byte("done"), 0644)

	fmt.Printf("Seeded PC photos: %d files extracted, %d PCs updated\n", len(entries), updated)
	return nil
}

type photoEntry struct {
	pcNum       int
	dbCol       string // "photo_serial" or "photo_front"
	dbColSuffix string // "serial" or "front"
	fileName    string
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
		dbColSuffix := "serial"
		if matches[2] == "full" || matches[2] == "front" {
			dbCol = "photo_front"
			dbColSuffix = "front"
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
			pcNum:       pcNum,
			dbCol:       dbCol,
			dbColSuffix: dbColSuffix,
			fileName:    base,
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

	client := &http.Client{
		Timeout: 300 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		},
	}

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

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = io.Copy(d, s)
	return err
}
