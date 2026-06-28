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
	pcLabel   string // "pc-33", "pc-cadangan-1", "pc-dosen"
	photoType string // "serial" or "front"
	sourcePath string
}

func main() {
	source := flag.String("source", "", "Folder berisi file .heic (contoh: pc1_sn.heic, pc1_full.heic)")
	output := flag.String("output", "photos.zip", "Path output file .zip")
	maxDim := flag.Int("max-dimension", 1280, "Resize maksimal dimensi (px)")
	prefix := flag.String("prefix", "", "Prefix untuk nama file di ZIP (contoh: mi-1)")
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

	reStandard := regexp.MustCompile(`^pc(\d+)_(sn|full)\.(heic|jpg|jpeg)$`)
	reSpecial := regexp.MustCompile(`^(pc-[\w-]+)_(sn|full)\.(heic|jpg|jpeg)$`)
	var photos []photoEntry

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())

		var pcLabel string
		var photoType string

		if m := reStandard.FindStringSubmatch(name); m != nil {
			pcLabel = "pc-" + m[1]
			if m[2] == "full" {
				photoType = "front"
			} else {
				photoType = "serial"
			}
		} else if m := reSpecial.FindStringSubmatch(name); m != nil {
			pcLabel = m[1]
			if m[2] == "full" {
				photoType = "front"
			} else {
				photoType = "serial"
			}
		} else {
			fmt.Printf("  SKIP: %s (tidak cocok pola pc{N}_{sn|full}.heic atau pc-{label}_{sn|full}.heic)\n", e.Name())
			continue
		}

		photos = append(photos, photoEntry{
			pcLabel:    pcLabel,
			photoType:  photoType,
			sourcePath: filepath.Join(*source, e.Name()),
		})
	}

	if len(photos) == 0 {
		fmt.Println("Tidak ada file .heic yang cocok ditemukan.")
		os.Exit(0)
	}

	fmt.Printf("Ditemukan %d file foto:\n", len(photos))
	pre := ""
	if *prefix != "" {
		pre = *prefix + "-"
	}
	for _, p := range photos {
		fmt.Printf("  %s → %s%s_%s.jpeg\n", p.pcLabel, pre, p.pcLabel, p.photoType)
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
		outName := p.pcLabel + "_" + p.photoType + ".jpeg"
		if *prefix != "" {
			outName = *prefix + "-" + outName
		}
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
