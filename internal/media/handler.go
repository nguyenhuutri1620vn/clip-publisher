package media

import (
	"context"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Resolver maps media URLs to files on disk.
type Resolver interface {
	RenderPath(renderID int64) (string, error)
	VideoDir(videoID int64) (string, error)
	ThumbPath(ctx context.Context, videoID int64) (string, error)
}

// Handler serves /media/render/<id>, /media/file/<videoID>/<rel>, /media/thumb/<videoID>.
func Handler(r Resolver) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.EscapedPath()
		if !strings.HasPrefix(path, "/media/") {
			http.NotFound(w, req)
			return
		}
		parts := strings.SplitN(strings.TrimPrefix(path, "/media/"), "/", 3)
		if len(parts) < 2 {
			http.NotFound(w, req)
			return
		}
		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			http.NotFound(w, req)
			return
		}
		var file string
		switch parts[0] {
		case "render":
			file, err = r.RenderPath(id)
		case "thumb":
			file, err = r.ThumbPath(req.Context(), id)
		case "file":
			if len(parts) != 3 {
				http.NotFound(w, req)
				return
			}
			file, err = resolveInside(r, id, parts[2])
		default:
			http.NotFound(w, req)
			return
		}
		if err != nil || file == "" {
			http.NotFound(w, req)
			return
		}
		serveFile(w, req, file)
	})
}

// resolveInside unescapes rel and returns it only if it stays inside the video's folder.
func resolveInside(r Resolver, videoID int64, escaped string) (string, error) {
	dir, err := r.VideoDir(videoID)
	if err != nil {
		return "", err
	}
	rel, err := url.PathUnescape(escaped)
	if err != nil || rel == "" || filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return "", os.ErrNotExist
	}
	full := filepath.Join(dir, filepath.FromSlash(rel))
	inside, err := filepath.Rel(dir, full)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", os.ErrNotExist
	}
	return full, nil
}

func serveFile(w http.ResponseWriter, req *http.Request, path string) {
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, req)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, req)
		return
	}
	if ct := mime.TypeByExtension(filepath.Ext(path)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, req, st.Name(), st.ModTime(), f)
}
