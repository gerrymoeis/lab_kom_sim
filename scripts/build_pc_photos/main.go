package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"inventaris-lab-kom/internal/services"
)

type photoEntry struct {
	pcNumber string
	photoType string // "serial" or "front"
	sourcePath string
}

func main() {
	source := flag.String("source", "", "Folder berisi file .heic (contoh: pc1_sn.heic, pc1_full.heic)")
	output := flag.String("output", "photos.zip", "Path output file .zip")
	maxDim := flag.Int("max-dimension", 1280, "Resize maksimal dimensi (px)")
	keepTemp := flag.Bool("keep-temp", false, "Jangan hapus folder temp setelah selesai")
	flag.Parse()

	if *source == "" {
		fmt.Println("ERROR: --source wajib diisi")
		fmt.Println("Contoh: go run scripts/build_pc_photos/main.go --source=/path/to/heic/folder")
		os.Exit(1)
	}

	srcStat, err := os.Stat(*source)
	if err != nil || !srcStat.IsDir() {
		fmt.Printf("ERROR: source '%s' tidak ditemukan atau bukan folder\n", *source)
		os.Exit(1)
	}

	entries, err := os.ReadDir(*source)
	if err != nil {
		fmt.Printf("ERROR: gagal baca folder source: %v\n", err)
		os.Exit(1)
	}

	re := regexp.MustCompile(`^pc(\d+)_(sn|full)\.heic$`)
	var photos []photoEntry

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		matches := re.FindStringSubmatch(name)
		if matches == nil {
			fmt.Printf("  SKIP: %s (tidak cocok pola pc{N}_sn.heic / pc{N}_full.heic)\n", e.Name())
			continue
		}
		pt := "serial"
		if matches[2] == "full" {
			pt = "front"
		}
		photos = append(photos, photoEntry{
			pcNumber:   matches[1],
			photoType:  pt,
			sourcePath: filepath.Join(*source, e.Name()),
		})
	}

	if len(photos) == 0 {
		fmt.Println("Tidak ada file .heic yang cocok ditemukan.")
		os.Exit(0)
	}

	fmt.Printf("Ditemukan %d file foto:\n", len(photos))
	for _, p := range photos {
		fmt.Printf("  PC-%s → %s_%s.jpeg\n", p.pcNumber, p.pcNumber, p.photoType)
	}

	tmpDir, err := os.MkdirTemp("", "pc-photos-build-*")
	if err != nil {
		fmt.Printf("ERROR: gagal buat temp dir: %v\n", err)
		os.Exit(1)
	}
	if !*keepTemp {
		defer os.RemoveAll(tmpDir)
	}

	svc := services.NewImageService()

	for _, p := range photos {
		outName := fmt.Sprintf("%s_%s.jpeg", p.pcNumber, p.photoType)
		outPath := filepath.Join(tmpDir, outName)

		fmt.Printf("\n  Memproses %s → %s ...\n", filepath.Base(p.sourcePath), outName)
		if err := svc.CompressAndSave(p.sourcePath, outPath, *maxDim); err != nil {
			fmt.Printf("  ERROR: gagal konversi %s: %v\n", filepath.Base(p.sourcePath), err)
			os.Exit(1)
		}
		fmt.Printf("  ✅ %s selesai\n", outName)
	}

	zipFile, err := os.Create(*output)
	if err != nil {
		fmt.Printf("ERROR: gagal buat file zip: %v\n", err)
		os.Exit(1)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)

	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(tmpDir, path)
		f, err := zw.Create(relPath)
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(f, src)
		return err
	})
	if err != nil {
		fmt.Printf("ERROR: gagal buat zip: %v\n", err)
		os.Exit(1)
	}

	if err := zw.Close(); err != nil {
		fmt.Printf("ERROR: gagal close zip: %v\n", err)
		os.Exit(1)
	}

	zipInfo, _ := os.Stat(*output)
	fmt.Printf("\n✅ Selesai! %d foto dikonversi dan di-zip ke:\n   %s\n   Ukuran: %d bytes\n",
		len(photos), *output, zipInfo.Size())
}
