package imaging

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"image"
	_ "image/jpeg" // Register JPEG header validation.
	_ "image/png"  // Register PNG header validation.
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	_ "golang.org/x/image/webp" // Register WebP header validation.

	"lumio/internal/app"
	"lumio/internal/domain"
)

const (
	jpegType = "image/jpeg"
	pngType  = "image/png"
	webpType = "image/webp"
)

type Vips struct{}

func command(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "VIPS_CONCURRENCY=2", "VIPS_DISC_THRESHOLD=64m")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %.1000s", name, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}
func dimension(ctx context.Context, path, field string) (int, error) {
	out, err := command(ctx, "vipsheader", "-f", field, path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}
func (Vips) Process(ctx context.Context, p domain.Photo, objects app.ObjectStore) (width, height int, manifest string, err error) {
	dir, err := os.MkdirTemp("", "lumio-photo-")
	if err != nil {
		return 0, 0, "", err
	}
	defer os.RemoveAll(dir)
	source := filepath.Join(dir, "original")
	f, err := os.Create(source)
	if err != nil {
		return 0, 0, "", err
	}
	err = objects.Download(ctx, p.Original, f, p.Size)
	closeErr := f.Close()
	if err != nil {
		return 0, 0, "", err
	}
	if closeErr != nil {
		return 0, 0, "", closeErr
	}
	upright, err := prepare(ctx, source, dir, p.ContentType)
	if err != nil {
		return 0, 0, "", err
	}
	width, err = dimension(ctx, upright, "width")
	if err != nil {
		return 0, 0, "", err
	}
	height, err = dimension(ctx, upright, "height")
	if err != nil {
		return 0, 0, "", err
	}
	keys := make([]string, 0, 8)
	last := 0
	for _, target := range []int{480, 960, 1600, 2400} {
		target = min(target, width)
		if target == last {
			continue
		}
		last = target
		variantKeys, variantErr := variants(ctx, objects, p, upright, dir, target)
		if variantErr != nil {
			return 0, 0, "", variantErr
		}
		keys = append(keys, variantKeys...)
	}
	data, err := json.Marshal(keys)
	return width, height, string(data), err
}

func validateFile(source, kind string) error {
	f, err := os.Open(source)
	if err != nil {
		return err
	}
	defer f.Close()
	head := make([]byte, 512)
	n, err := f.Read(head)
	if err != nil {
		return err
	}
	if http.DetectContentType(head[:n]) != kind {
		return domain.ErrInvalid
	}
	if _, err = f.Seek(0, 0); err != nil {
		return err
	}
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return err
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 30000 || cfg.Height > 30000 || int64(cfg.Width)*int64(cfg.Height) > 100000000 {
		return domain.ErrInvalid
	}
	return nil
}
func prepare(ctx context.Context, source, dir, kind string) (string, error) {
	if err := validateFile(source, kind); err != nil {
		return "", err
	}
	// Explicit loaders prevent libvips from interpreting an unsupported format.
	loader := map[string]string{jpegType: "jpegload", pngType: "pngload", webpType: "webpload"}[kind]
	if loader == "" {
		return "", domain.ErrInvalid
	}
	decoded := filepath.Join(dir, "decoded.v")
	if _, err := command(ctx, "vips", loader, source, decoded, "--fail-on", "warning"); err != nil {
		return "", err
	}
	upright := filepath.Join(dir, "upright.v")
	_, err := command(ctx, "vips", "autorot", decoded, upright)
	return upright, err
}
func variants(ctx context.Context, objects app.ObjectStore, p domain.Photo, upright, dir string, target int) ([]string, error) {
	thumb := filepath.Join(dir, "thumb.v")
	if _, err := command(ctx, "vips", "thumbnail", upright, thumb, strconv.Itoa(target), "--size", "down", "--height", "30000", "--output-profile", "srgb"); err != nil {
		return nil, err
	}
	input := thumb
	if p.Watermark != "" {
		marked, err := watermark(ctx, thumb, dir, target, p.Watermark)
		if err != nil {
			return nil, err
		}
		input = marked
	}
	keys := make([]string, 0, 2)
	for _, format := range []string{"webp", "jpg"} {
		output := filepath.Join(dir, "out."+format)
		op, kind := "webpsave", webpType
		if format == "jpg" {
			op, kind = "jpegsave", jpegType
		}
		if _, err := command(ctx, "vips", op, input, output, "--Q", "85", "--strip"); err != nil {
			return nil, err
		}
		key := fmt.Sprintf("media/%s/v1/%s/%d.%s", p.ID, p.Lease, target, format)
		if err := uploadFile(ctx, objects, key, kind, output); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}
func watermark(ctx context.Context, thumb, dir string, width int, text string) (string, error) {
	height, err := dimension(ctx, thumb, "height")
	if err != nil {
		return "", err
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d"><text x="50%%" y="90%%" text-anchor="middle" font-family="sans-serif" font-size="%d" fill="white" stroke="black" stroke-width="1" paint-order="stroke" opacity="0.65">%s</text></svg>`, width, height, max(8, width/40), html.EscapeString(text))
	overlay := filepath.Join(dir, "mark.svg")
	if err = os.WriteFile(overlay, []byte(svg), 0o600); err != nil {
		return "", err
	}
	marked := filepath.Join(dir, "marked.v")
	_, err = command(ctx, "vips", "composite2", thumb, overlay, marked, "over")
	return marked, err
}
func uploadFile(ctx context.Context, objects app.ObjectStore, key, kind, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return objects.Put(ctx, key, kind, f)
}
