package versioner

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var hashPattern = regexp.MustCompile(`\.([a-f0-9]{8})\.(css|js)$`)

type Versioner struct {
	hashes    map[string]string
	staticDir string
}

func New(staticDir string) (*Versioner, error) {
	v := &Versioner{
		hashes:    make(map[string]string),
		staticDir: staticDir,
	}
	err := filepath.WalkDir(staticDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".css" && ext != ".js" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h := sha256.Sum256(data)
		hash := fmt.Sprintf("%08x", h[:4])
		relPath, err := filepath.Rel(staticDir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		base := strings.TrimSuffix(relPath, ext)
		v.hashes[relPath] = base + "." + hash + ext
		return nil
	})
	return v, err
}

func (v *Versioner) URL(path string) string {
	if versioned, ok := v.hashes[path]; ok {
		return "/static/" + versioned
	}
	return "/static/" + path
}

func (v *Versioner) Handler() gin.HandlerFunc {
	absStatic, _ := filepath.Abs(v.staticDir)
	return func(c *gin.Context) {
		reqPath := c.Param("filepath")
		origPath := hashPattern.ReplaceAllString(reqPath, ".$2")
		fullPath := filepath.Join(v.staticDir, origPath)
		absFull, _ := filepath.Abs(fullPath)
		if !strings.HasPrefix(absFull, absStatic) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.File(fullPath)
	}
}
