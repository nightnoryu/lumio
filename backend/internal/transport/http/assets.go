package httptransport

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
)

const indexHTML = "index.html"
const healthPath = "/healthz"
const livePath = "/livez"
const viewerPath = "/portfolio.js"
const recoveryPath = "/recovery.js"

var hashedAsset = regexp.MustCompile(`-[A-Za-z0-9_-]{8,}\.(js|css)$`)

func staticAssets(assets fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = indexHTML
		}
		if !fs.ValidPath(name) || (name != indexHTML && name != "recovery.js" && !strings.HasPrefix(name, "assets/")) {
			http.NotFound(w, r)
			return
		}
		data, err := fs.ReadFile(assets, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		if strings.HasPrefix(name, "assets/") && hashedAsset.MatchString(path.Base(name)) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		tag := fmt.Sprintf(`"%x"`, sha256.Sum256(data))
		w.Header().Set("ETag", tag)
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	})
}
