package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
	"inventaris-lab-kom/internal/timeutil"
)

type PCStatusInfo struct {
	Status, BadgeClass, Icon, Color, VisLabel string
}

type PlacementInfo struct {
	Placement, BadgeClass, Icon, VisLabel string
}

var pcStatusMap = map[string]PCStatusInfo{
	"normal":  {"normal", "success", "bi-check-circle-fill", "text-success", "Normal"},
	"warning": {"warning", "warning", "bi-exclamation-triangle-fill", "text-warning", "Warning"},
	"broken":  {"broken", "danger", "bi-x-circle-fill", "text-danger", "Rusak"},
}

var pcPlacementMap = map[string]PlacementInfo{
	"dipakai":  {"dipakai", "primary", "bi-check-lg", "Dipakai"},
	"cadangan": {"cadangan", "secondary", "bi-box-seam", "Cadangan"},
}

func getPCStatusInfo(status string) PCStatusInfo {
	if s, ok := pcStatusMap[status]; ok {
		return s
	}
	return pcStatusMap["normal"]
}

func getPCPlacementInfo(placement string) PlacementInfo {
	if p, ok := pcPlacementMap[placement]; ok {
		return p
	}
	return pcPlacementMap["dipakai"]
}

func loadPublicFuncMap() template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"iterate": func(count int) []int {
			r := make([]int, count)
			for i := 0; i < count; i++ {
				r[i] = i
			}
			return r
		},
		"pcStatusInfo":    func(status string) PCStatusInfo { return getPCStatusInfo(status) },
		"pcPlacementInfo": func(placement string) PlacementInfo { return getPCPlacementInfo(placement) },
		"isSpecialLabel": func(label, placement string) bool {
			if placement != "dipakai" { return false }
			if len(label) < 4 || !strings.HasPrefix(label, "pc-") { return false }
			for _, c := range label[3:] {
				if c >= '0' && c <= '9' { continue }
				return true
			}
			return false
		},
		"formatPCLabel": func(pc models.PC) string {
			if pc.Label != "" {
				return pc.Label
			}
			return "-"
		},
		"localTime": func(t interface{}) interface{} {
			switch v := t.(type) {
			case time.Time:
				if v.IsZero() {
					return v
				}
				return v.In(timeutil.Location())
			case *time.Time:
				if v == nil || v.IsZero() {
					return v
				}
				return v.In(timeutil.Location())
			}
			return t
		},
		"tzCode": func() string { return timeutil.Code() },
	}
}

func loadPublicTemplates(templatesDir string, funcMap template.FuncMap) (*template.Template, error) {
	templ := template.New("").Funcs(funcMap)
	err := filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".html" {
			return nil
		}
		relPath, _ := filepath.Rel(templatesDir, path)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = templ.New(filepath.ToSlash(relPath)).Parse(string(content))
		return err
	})
	return templ, err
}

func writeJSON(path string, v interface{}) error {
	os.MkdirAll(filepath.Dir(path), 0755)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()
		os.MkdirAll(filepath.Dir(target), 0755)
		dstFile, err := os.Create(target)
		if err != nil {
			return err
		}
		defer dstFile.Close()
		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

func buildDashboardGrid(pcs []models.PC, colsPerRow []int) ([][]models.PC, []models.PC, models.PC, models.PC, models.PC, []models.PC, map[string]int, int) {
	statusCounts := make(map[string]int)
	var spareCount int
	for _, pc := range pcs {
		if pc.Placement == "cadangan" {
			spareCount++
		} else {
			statusCounts[pc.Status]++
		}
	}

	gridRowCount := len(colsPerRow)
	grid := make([][]models.PC, gridRowCount)
	for i := range grid {
		grid[i] = make([]models.PC, colsPerRow[i])
	}

	var extraPCs []models.PC
	var pcLecturer, pcLaboran, pcCCTV models.PC
	var specialPCs []models.PC
	for _, pc := range pcs {
		if pc.Placement == "cadangan" {
			continue
		}
		maxCol := colsPerRow[pc.Row-1]
		if pc.Row >= 1 && pc.Row <= gridRowCount && pc.Column >= 1 && pc.Column <= maxCol {
			grid[pc.Row-1][pc.Column-1] = pc
		} else if pc.Label != "" && isNumericLabel(pc.Label) {
			extraPCs = append(extraPCs, pc)
		} else if strings.EqualFold(pc.Label, "pc-dosen") {
			pcLecturer = pc
		} else if strings.EqualFold(pc.Label, "pc-laboran") {
			pcLaboran = pc
		} else if strings.EqualFold(pc.Label, "pc-cctv") {
			pcCCTV = pc
		} else {
			specialPCs = append(specialPCs, pc)
		}
	}

	return grid, extraPCs, pcLecturer, pcLaboran, pcCCTV, specialPCs, statusCounts, spareCount
}

func isNumericLabel(label string) bool {
	if len(label) < 4 || !strings.HasPrefix(label, "pc-") {
		return false
	}
	for _, c := range label[3:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func buildSoftwareGrid(pcList []repository.PCInstallStatus) [][]repository.PCInstallStatus {
	maxRow, maxCol := 0, 0
	for _, p := range pcList {
		if p.Row > maxRow {
			maxRow = p.Row
		}
		if p.Column > maxCol {
			maxCol = p.Column
		}
	}
	if maxRow < 1 || maxCol < 1 {
		return nil
	}
	grid := make([][]repository.PCInstallStatus, maxRow)
	for i := range grid {
		grid[i] = make([]repository.PCInstallStatus, maxCol)
	}
	for _, p := range pcList {
		if p.Row >= 1 && p.Row <= maxRow && p.Column >= 1 && p.Column <= maxCol {
			grid[p.Row-1][p.Column-1] = p
		}
	}
	return grid
}

func renderToFile(tmpl *template.Template, name string, data interface{}, path string) error {
	os.MkdirAll(filepath.Dir(path), 0755)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.ExecuteTemplate(f, name, data)
}

var indexRedirectHTML = `<!DOCTYPE html><html><head><meta http-equiv="refresh" content="0;url=dashboard.html"></head><body></body></html>`

func RunPublicBuild(db *database.DB, cfg config.PublicBuildConfig, labName, labTitle string) error {
	pcRepo := repository.NewPCRepository(db)
	deviceRepo := repository.NewDeviceRepository(db)
	softwareRepo := repository.NewSoftwareRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)

	pcs, err := pcRepo.List(repository.PCFilters{})
	if err != nil {
		return fmt.Errorf("query PCs: %w", err)
	}
	devices, err := deviceRepo.List(repository.DeviceFilters{})
	if err != nil {
		return fmt.Errorf("query devices: %w", err)
	}
	softwareStats, err := softwareRepo.List("", "")
	if err != nil {
		return fmt.Errorf("query software: %w", err)
	}
	schedules, err := scheduleRepo.List("", "")
	if err != nil {
		return fmt.Errorf("query schedules: %w", err)
	}

	if pcs == nil { pcs = []models.PC{} }
	if devices == nil { devices = []models.Device{} }
	if softwareStats == nil { softwareStats = []repository.SoftwareStat{} }
	if schedules == nil { schedules = []models.CourseSchedule{} }

	funcMap := loadPublicFuncMap()
	tmpl, err := loadPublicTemplates(cfg.TemplateDir, funcMap)
	if err != nil {
		return fmt.Errorf("load templates: %w", err)
	}

	outDir := filepath.Join(cfg.OutDir, labName)
	os.RemoveAll(outDir)
	os.MkdirAll(outDir, 0755)

	basePath := "/" + labName
	commonData := map[string]interface{}{
		"basePath": basePath, "labName": labName, "labTitle": labTitle,
	}

	var errs []error
	re := func(tmplName, filePath string, data interface{}) {
		if err := renderToFile(tmpl, tmplName, data, filePath); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", filePath, err))
			log.Printf("PublicBuild: render error %s: %v", filePath, err)
		}
	}
	wj := func(filePath string, v interface{}) {
		if err := writeJSON(filePath, v); err != nil {
			errs = append(errs, fmt.Errorf("json %s: %w", filePath, err))
		}
	}

	// Dashboard
	layout := config.GetGridLayout(labName)
	grid, extraPCs, pcLecturer, pcLaboran, pcCCTV, specialPCs, statusCounts, spareCount := buildDashboardGrid(pcs, layout.ColsPerRow)
	re("dashboard.html", filepath.Join(outDir, "dashboard.html"), mergeData(commonData, map[string]interface{}{
		"title": "Dashboard", "currentPage": "dashboard",
		"pcGrid": grid, "pcs": pcs, "extraPCs": extraPCs,
		"statusCounts": statusCounts, "spareCount": spareCount,
		"pcLecturer": pcLecturer, "pcLaboran": pcLaboran, "pcCCTV": pcCCTV,
		"specialPCs": specialPCs,
	}))

	// Index (redirect to dashboard)
	if err := os.WriteFile(filepath.Join(outDir, "index.html"), []byte(indexRedirectHTML), 0644); err != nil {
		errs = append(errs, fmt.Errorf("index: %w", err))
	}

	// PC list
	pcJSON, err := json.Marshal(pcs)
	if err != nil {
		errs = append(errs, fmt.Errorf("marshal pcs: %w", err))
	}
	re("pc/list.html", filepath.Join(outDir, "pc", "list.html"), mergeData(commonData, map[string]interface{}{
		"title": "Daftar PC", "currentPage": "pc",
		"dataJSON": template.JS(pcJSON),
	}))

	// PC detail — one file per PC
	for _, pc := range pcs {
		label := pc.Label
		if label == "" {
			label = fmt.Sprintf("pc-%d", pc.ID)
		}
		re("pc/detail.html", filepath.Join(outDir, "pc", "detail", label+".html"), mergeData(commonData, map[string]interface{}{
			"title": "Detail " + label, "currentPage": "pc",
			"pc": pc,
		}))
	}

	// Device list
	devJSON, err := json.Marshal(devices)
	if err != nil {
		errs = append(errs, fmt.Errorf("marshal devices: %w", err))
	}
	re("device/list.html", filepath.Join(outDir, "devices", "list.html"), mergeData(commonData, map[string]interface{}{
		"title": "Daftar Perangkat", "currentPage": "devices",
		"dataJSON": template.JS(devJSON),
	}))

	// Device detail — one file per device
	for _, d := range devices {
		re("device/detail.html", filepath.Join(outDir, "devices", "detail", strings.ToLower(d.AssetCode)+".html"), mergeData(commonData, map[string]interface{}{
			"title":          "Detail - " + d.AssetCode,
			"currentPage":    "devices",
			"device":         d,
			"deviceTypeName": d.DeviceTypeName,
		}))
	}

	// Software list
	swJSON, err := json.Marshal(softwareStats)
	if err != nil {
		errs = append(errs, fmt.Errorf("marshal software: %w", err))
	}
	re("software/list.html", filepath.Join(outDir, "software", "list.html"), mergeData(commonData, map[string]interface{}{
		"title": "Software Catalog", "currentPage": "software",
		"dataJSON": template.JS(swJSON),
	}))

	// Software detail — one file per software
	for _, sw := range softwareStats {
		pcList, err := softwareRepo.GetPCInstallStatus(sw.ID)
		if err != nil {
			errs = append(errs, fmt.Errorf("install status sw=%d: %w", sw.ID, err))
			continue
		}
		installedCount := 0
		for _, p := range pcList {
			if p.Installed {
				installedCount++
			}
		}
		swGrid := buildSoftwareGrid(pcList)
		re("software/detail.html", filepath.Join(outDir, "software", "detail", sw.Slug+".html"), mergeData(commonData, map[string]interface{}{
			"title":          "Detail Software - " + sw.Name,
			"currentPage":    "software",
			"software":       sw.SoftwareCatalog,
			"pcGrid":         swGrid,
			"installedCount": installedCount,
			"totalPCs":       len(pcList),
		}))
	}

	// Schedule list
	schJSON, err := json.Marshal(schedules)
	if err != nil {
		errs = append(errs, fmt.Errorf("marshal schedules: %w", err))
	}
	re("schedule/list.html", filepath.Join(outDir, "schedules", "list.html"), mergeData(commonData, map[string]interface{}{
		"title": "Jadwal Mata Kuliah", "currentPage": "schedules",
		"dataJSON": template.JS(schJSON),
	}))

	// Generate JSON data files
	os.MkdirAll(filepath.Join(outDir, "data"), 0755)
	wj(filepath.Join(outDir, "data", "pc.json"), pcs)
	wj(filepath.Join(outDir, "data", "devices.json"), devices)
	wj(filepath.Join(outDir, "data", "software.json"), softwareStats)
	wj(filepath.Join(outDir, "data", "schedules.json"), schedules)

	// Copy static assets (shared at root level)
	if err := copyDir(cfg.StaticDir, filepath.Join(cfg.OutDir, "static")); err != nil {
		errs = append(errs, fmt.Errorf("copy static: %w", err))
	}

	// Copy device type photos (per-lab)
	deviceTypesDir := filepath.Join("uploads", "device_types")
	if fi, err := os.Stat(deviceTypesDir); err == nil && fi.IsDir() {
		if err := copyDir(deviceTypesDir, filepath.Join(outDir, "uploads", "device_types")); err != nil {
			errs = append(errs, fmt.Errorf("copy device_type photos: %w", err))
		}
	}

	// Copy PC photos (per-lab)
	pcPhotosDir := filepath.Join("uploads", "pc")
	if fi, err := os.Stat(pcPhotosDir); err == nil && fi.IsDir() {
		if err := copyDir(pcPhotosDir, filepath.Join(outDir, "uploads", "pc")); err != nil {
			errs = append(errs, fmt.Errorf("copy pc photos: %w", err))
		}
	}

	// Git push to public repo if configured
	if cfg.RepoDir != "" {
		if err := gitPushIfChanged(cfg.RepoDir, cfg.OutDir, cfg.Branch); err != nil {
			log.Printf("PublicBuild: git push warning: %v", err)
		}
	}

	if len(errs) > 0 {
		log.Printf("PublicBuild: complete with %d error(s)", len(errs))
		return errors.Join(errs...)
	}
	log.Printf("PublicBuild: complete — %s", outDir)
	return nil
}

func gitPushIfChanged(repoDir, outDir, branch string) error {
	distPath := filepath.Join(repoDir, outDir)

	if err := os.MkdirAll(distPath, 0755); err != nil {
		return fmt.Errorf("mkdir dist: %w", err)
	}

	if err := removeAllContents(distPath); err != nil {
		return fmt.Errorf("clean dist: %w", err)
	}

	if err := copyDir(outDir, distPath); err != nil {
		return fmt.Errorf("copy dist: %w", err)
	}

	// Ensure we are on the target branch (must exist locally)
	coCmd := exec.Command("git", "-C", repoDir, "checkout", branch)
	coCmd.Stderr = os.Stderr
	if err := coCmd.Run(); err != nil {
		return fmt.Errorf("git checkout %s: %w — ensure branch '%s' exists locally", branch, err, branch)
	}

	// Clear git index for this path to handle case-only renames
	// on case-insensitive filesystems (Windows)
	rmCmd := exec.Command("git", "-C", repoDir, "rm", "--cached", "-r", "--ignore-unmatch", outDir)
	rmCmd.Stderr = nil
	rmCmd.Run()

	cmds := [][]string{
		{"git", "-C", repoDir, "add", "-A"},
		{"git", "-C", repoDir, "diff", "--cached", "--quiet"},
	}

	for _, args := range cmds[:1] {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git add: %w", err)
		}
	}

	// Check if anything changed
	diffCmd := exec.Command(cmds[1][0], cmds[1][1:]...)
	if err := diffCmd.Run(); err == nil {
		log.Println("PublicBuild: git — no changes, skipping commit")
		return nil
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	commitCmd := exec.Command("git", "-C", repoDir, "commit", "-m", fmt.Sprintf("auto-build %s", now))
	commitCmd.Stderr = os.Stderr
	commitCmd.Stdout = os.Stdout
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	pushCmd := exec.Command("git", "-C", repoDir, "push", "origin", branch)
	pushCmd.Stderr = os.Stderr
	pushCmd.Stdout = os.Stdout
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	log.Println("PublicBuild: git — committed and pushed")
	return nil
}

func removeAllContents(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		if name == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

func mergeData(base, extra map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(base)+len(extra))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range extra {
		result[k] = v
	}
	return result
}

func GenerateLabSelector(labs []config.LabConfig, cfg config.PublicBuildConfig) error {
	os.MkdirAll(cfg.OutDir, 0755)

	tmpl, err := loadPublicTemplates(cfg.TemplateDir, template.FuncMap{})
	if err != nil {
		tmpl = template.New("lab_selector")
		tmpl.Parse(labSelectorTemplate)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "lab_selector.html", map[string]interface{}{
		"title": "Pilih Laboratorium",
		"labs":  labs,
	}); err != nil {
		// fallback: execute as standalone
		buf.Reset()
		if err2 := tmpl.Execute(&buf, map[string]interface{}{
			"title": "Pilih Laboratorium",
			"labs":  labs,
		}); err2 != nil {
			return fmt.Errorf("render lab selector: %w", err2)
		}
	}

	path := filepath.Join(cfg.OutDir, "index.html")
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write lab selector: %w", err)
	}
	log.Printf("PublicBuild: lab selector generated at %s", path)
	return nil
}

var labSelectorTemplate = `<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.title}}</title>
<link href="/static/vendor/bootstrap/css/bootstrap.min.css" rel="stylesheet">
<link rel="stylesheet" href="/static/vendor/bootstrap-icons/bootstrap-icons.min.css">
<link rel="stylesheet" href="/static/css/style.css">
<style>
body { display:flex; align-items:center; justify-content:center; min-height:100vh; background:#f5f7fa; }
.card { transition: transform .2s; cursor:pointer; }
.card:hover { transform:translateY(-4px); box-shadow:0 8px 25px rgba(0,0,0,.15); }
</style>
</head>
<body>
<div class="container text-center">
<h1 class="mb-2"><i class="bi bi-pc-display-horizontal"></i> Inventaris Lab Kom</h1>
<p class="text-muted mb-5">Pilih laboratorium untuk melihat inventaris</p>
<div class="row justify-content-center g-4">
{{range .labs}}
<div class="col-md-4">
<a href="/{{.Name}}/dashboard.html" class="text-decoration-none">
<div class="card shadow-sm h-100">
<div class="card-body text-center py-5">
<i class="bi bi-building fs-1 text-primary"></i>
<h4 class="mt-3">{{.Title}}</h4>
<p class="text-muted small mb-0">{{.Name}}</p>
</div>
</div>
</a>
</div>
{{end}}
</div>
</div>
<script src="/static/vendor/bootstrap/js/bootstrap.bundle.min.js"></script>
</body>
</html>`

func PublicBuildOutputDir(cfg config.PublicBuildConfig, labName string) string {
	if cfg.RepoDir != "" {
		return filepath.Join(cfg.RepoDir, cfg.OutDir, labName)
	}
	return filepath.Join(cfg.OutDir, labName)
}
