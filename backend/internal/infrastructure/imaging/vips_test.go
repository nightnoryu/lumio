package imaging

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lumio/internal/domain"
)

type memoryObjects struct {
	input  []byte
	output map[string][]byte
}

func (m *memoryObjects) Download(_ context.Context, _ string, w io.Writer, _ int64) error {
	_, err := w.Write(m.input)
	return err
}
func (m *memoryObjects) Put(_ context.Context, key, _ string, r io.Reader) error {
	data, err := io.ReadAll(r)
	m.output[key] = data
	return err
}
func (*memoryObjects) UploadURL(context.Context, string, string, int64) (string, error) {
	panic("unused")
}
func (*memoryObjects) Promote(context.Context, string, string) error       { panic("unused") }
func (*memoryObjects) Check(context.Context, string, string, int64) error  { panic("unused") }
func (*memoryObjects) DeletePrefix(context.Context, string) error          { panic("unused") }
func (*memoryObjects) PreviewURL(context.Context, string) (string, error)  { panic("unused") }
func (*memoryObjects) PruneVariants(context.Context, string, string) error { panic("unused") }
func TestVipsImages(t *testing.T) {
	if _, err := exec.LookPath("vips"); err != nil {
		t.Skip("run in worker image with libvips")
	}
	for _, tc := range []struct {
		name, kind, mark string
		w, h             int
		rotate           bool
	}{{"portrait", pngType, "", 300, 600, false}, {"landscape", jpegType, "Photographer", 1200, 800, false}, {"exif", jpegType, "", 800, 400, true}, {"webp", webpType, "", 640, 480, false}} {
		t.Run(tc.name, func(t *testing.T) {
			input := testImage(t, tc.w, tc.h, tc.kind, tc.rotate)
			store := &memoryObjects{input: input, output: map[string][]byte{}}
			p := domain.Photo{ID: "photo", Lease: "attempt", Original: "media/photo/original", Size: int64(len(input)), ContentType: tc.kind, Watermark: tc.mark}
			w, h, raw, err := (Vips{}).Process(t.Context(), p, store)
			if err != nil {
				t.Fatal(err)
			}
			wantW, wantH := tc.w, tc.h
			if tc.rotate {
				wantW, wantH = tc.h, tc.w
			}
			if w != wantW || h != wantH {
				t.Fatalf("dimensions %dx%d expected %dx%d", w, h, wantW, wantH)
			}
			if !bytes.Equal(store.input, input) || store.output[p.Original] != nil {
				t.Fatal("original was modified")
			}
			checkVariants(t, raw, store.output, w, h)
		})
	}
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func TestVipsRejectsInvalid(t *testing.T) {
	store := &memoryObjects{input: []byte("<svg><script>alert(1)</script></svg>"), output: map[string][]byte{}}
	width, height, variants, err := (Vips{}).Process(t.Context(), domain.Photo{Size: int64(len(store.input)), ContentType: pngType}, store)
	if err == nil || len(store.output) != 0 || width != 0 || height != 0 || variants != "" {
		t.Fatal("invalid upload accepted")
	}
}

func testImage(t *testing.T, w, h int, kind string, rotate bool) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x % 256), G: uint8(y % 256), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	var err error
	if kind == pngType || kind == webpType {
		err = png.Encode(&buf, img)
	} else {
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95})
	}
	if err != nil {
		t.Fatal(err)
	}
	input := buf.Bytes()
	if kind == webpType {
		dir := t.TempDir()
		source := filepath.Join(dir, "source.png")
		output := filepath.Join(dir, "source.webp")
		if err = os.WriteFile(source, input, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err = command(t.Context(), "vips", "webpsave", source, output); err != nil {
			t.Fatal(err)
		}
		input, err = os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
	}

	if rotate {
		// EXIF IFD0 orientation=6, clockwise 90 degrees.
		exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
		data := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0, 34}, exif...)
		input = append(data, input[2:]...)
	}
	return input
}
func checkVariants(t *testing.T, raw string, output map[string][]byte, w, h int) {
	t.Helper()
	var keys []string
	if err := json.Unmarshal([]byte(raw), &keys); err != nil {
		t.Fatal(err)
	}
	if len(keys) < 2 {
		t.Fatal("missing variants")
	}
	for _, key := range keys {
		data := output[key]
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Width > w || cfg.Height > h {
			t.Fatal("upscaled")
		}
		if abs(cfg.Width*h-cfg.Height*w) > w {
			t.Fatal("aspect ratio changed")
		}
		if strings.HasSuffix(key, ".jpg") && format != "jpeg" {
			t.Fatal("missing JPEG fallback")
		}
		if bytes.Contains(data, []byte("Exif")) {
			t.Fatal("EXIF leaked")
		}
	}
}
