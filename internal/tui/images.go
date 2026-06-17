package tui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"treehole/internal/client"
)

const (
	listImageCellSize   = 5
	detailImageCellSize = 10
	imageCellGap        = 1
)

type imagePlacement struct {
	path        string
	cols        int
	rows        int
	left        int
	top         int
	z           int
	placeholder bool
	winCols     int
	winRows     int
}

type KittyImageRenderer struct {
	mu                sync.Mutex
	imageIDs          map[string]uint32
	uploaded          map[uint32]bool
	activePlacements  map[uint32]activePlacement
	nextImageID       uint32
	frameSeq          string
	frameGeneration   uint64
	paintedGeneration uint64
}

type activePlacement struct {
	imageID   uint32
	placement imagePlacement
}

func NewKittyImageRenderer() *KittyImageRenderer {
	if !supportsKittyGraphics() {
		return nil
	}
	return &KittyImageRenderer{
		imageIDs:         make(map[string]uint32),
		uploaded:         make(map[uint32]bool),
		activePlacements: make(map[uint32]activePlacement),
	}
}

func supportsKittyGraphics() bool {
	return os.Getenv("KITTY_WINDOW_ID") != "" || strings.Contains(os.Getenv("TERM"), "kitty")
}

func (r *KittyImageRenderer) Enabled() bool {
	return r != nil
}

func (r *KittyImageRenderer) ClearSequence() string {
	if r == nil {
		return ""
	}
	return ""
}

func (r *KittyImageRenderer) SetFrame(placements []imagePlacement) {
	if r == nil {
		return
	}
	seq := r.RenderPlacements(placements)
	r.mu.Lock()
	r.frameSeq = seq
	r.frameGeneration++
	r.mu.Unlock()
}

func (r *KittyImageRenderer) frame() (string, uint64) {
	if r == nil {
		return "", 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.frameSeq, r.frameGeneration
}

func (r *KittyImageRenderer) markPainted(generation uint64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if generation > r.paintedGeneration {
		r.paintedGeneration = generation
	}
}

func (r *KittyImageRenderer) OutputWriter(dst io.Writer) io.Writer {
	if r == nil {
		return dst
	}
	return &kittyFrameWriter{
		dst:      dst,
		renderer: r,
	}
}

func (r *KittyImageRenderer) PlacementSequence(p imagePlacement) string {
	if r == nil || p.path == "" || p.cols <= 0 || p.rows <= 0 {
		return ""
	}
	return ""
}

func (r *KittyImageRenderer) RenderPlacements(placements []imagePlacement) string {
	if r == nil {
		return ""
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var b strings.Builder

	nextActive := make(map[uint32]activePlacement, len(placements))
	for idx, placement := range placements {
		if placement.path == "" || placement.cols <= 0 || placement.rows <= 0 {
			continue
		}

		placementID := uint32(idx + 1)
		imageID := r.imageIDForPath(placement.path)
		prev, hadPrev := r.activePlacements[placementID]

		if !r.uploaded[imageID] {
			if placement.placeholder {
				r.uploaded[imageID] = true
			} else {
				b.WriteString(uploadImageSequence(imageID, placement.path))
				r.uploaded[imageID] = true
			}
		}
		if hadPrev && prev.imageID != imageID {
			b.WriteString(deletePlacementSequence(prev.imageID, prev.placement, placementID))
		}
		if !hadPrev || prev.imageID != imageID || prev.placement != placement {
			if placement.placeholder {
				b.WriteString(r.placeholderBackendSequence(placement))
			} else {
				b.WriteString(placeImageSequence(imageID, placementID, placement))
			}
		}

		nextActive[placementID] = activePlacement{
			imageID:   imageID,
			placement: placement,
		}
	}

	for placementID, prev := range r.activePlacements {
		if _, stillVisible := nextActive[placementID]; stillVisible {
			continue
		}
		b.WriteString(deletePlacementSequence(prev.imageID, prev.placement, placementID))
	}

	r.activePlacements = nextActive
	return b.String()
}

func (r *KittyImageRenderer) imageIDForPath(path string) uint32 {
	if id, ok := r.imageIDs[path]; ok {
		return id
	}
	id := mediaImageIDForPath(path)
	if _, exists := r.uploaded[id]; exists {
		r.nextImageID++
		id = id + r.nextImageID
		if id == 0 {
			id = 1
		}
	}
	r.imageIDs[path] = id
	return id
}

func uploadImageSequence(imageID uint32, path string) string {
	payload := base64.StdEncoding.EncodeToString([]byte(path))
	return fmt.Sprintf("\x1b_Gq=2,i=%d,t=f,f=100;%s\x1b\\", imageID, payload)
}

func placeImageSequence(imageID, placementID uint32, placement imagePlacement) string {
	return fmt.Sprintf(
		"\x1b7\x1b[%d;%dH\x1b_Gq=2,a=p,i=%d,p=%d,c=%d,r=%d,C=1,z=%d\x1b\\\x1b8",
		placement.top+1,
		placement.left+1,
		imageID,
		placementID,
		placement.cols,
		placement.rows,
		placement.z,
	)
}

func deletePlacementSequence(imageID uint32, placement imagePlacement, placementID uint32) string {
	if placement.placeholder {
		return ""
	}
	return fmt.Sprintf("\x1b_Gq=2,a=d,d=i,i=%d,p=%d\x1b\\", imageID, placementID)
}

func (r *KittyImageRenderer) placeholderBackendSequence(placement imagePlacement) string {
	args := kittenICatArgs(placement)
	out, err := exec.Command("kitten", args...).Output()
	if err != nil {
		log.Printf("[TUI][kitty] kitten icat backend failed: err=%v args=%v", err, args)
		return ""
	}
	return string(out)
}

func kittenICatArgs(placement imagePlacement) []string {
	winCols := maxInt(1, placement.winCols)
	winRows := maxInt(1, placement.winRows)
	winPixelsW := winCols * 10
	winPixelsH := winRows * 20
	return []string{
		"icat",
		"--stdin=no",
		"--unicode-placeholder",
		"--use-window-size", fmt.Sprintf("%d,%d,%d,%d", winCols, winRows, winPixelsW, winPixelsH),
		"--place", fmt.Sprintf("%dx%d@%dx%d", placement.cols, placement.rows, placement.left, placement.top),
		"--transfer-mode=file",
		placement.path,
	}
}

type kittyFrameWriter struct {
	dst      io.Writer
	renderer *KittyImageRenderer
}

func (w *kittyFrameWriter) Write(p []byte) (int, error) {
	n, err := w.dst.Write(p)
	if n > 0 {
		seq, generation := w.renderer.frame()
		if generation > 0 {
			if _, seqErr := io.WriteString(w.dst, seq); seqErr != nil && err == nil {
				err = seqErr
			}
			w.renderer.markPainted(generation)
		}
	}
	return n, err
}

func (w *kittyFrameWriter) Read(p []byte) (int, error) {
	if r, ok := w.dst.(interface{ Read([]byte) (int, error) }); ok {
		return r.Read(p)
	}
	return 0, io.EOF
}

func (w *kittyFrameWriter) Close() error {
	if c, ok := w.dst.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

func (w *kittyFrameWriter) Fd() uintptr {
	if f, ok := w.dst.(interface{ Fd() uintptr }); ok {
		return f.Fd()
	}
	return 0
}

type postImageLayout struct {
	lines      []string
	placements []imagePlacement
}

func visiblePlacements(placements []imagePlacement, yOffset, height, topAdjust int) []imagePlacement {
	if len(placements) == 0 || height <= 0 {
		return nil
	}

	visible := make([]imagePlacement, 0, len(placements))
	for _, placement := range placements {
		top := placement.top - yOffset + topAdjust
		bottom := top + placement.rows
		if top < 0 || bottom > height {
			continue
		}
		placement.top = top
		visible = append(visible, placement)
	}
	return visible
}

type resolvedMedia struct {
	id   string
	path string
}

var mediaPathCache sync.Map

func tuiProjectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "../..")
}

func resolveMediaPaths(mediaIDs string, preferOriginal bool) []resolvedMedia {
	ids := parseMediaIDs(mediaIDs)
	if len(ids) == 0 {
		return nil
	}

	resolved := make([]resolvedMedia, 0, len(ids))
	for _, id := range ids {
		if path := resolveMediaPath(id, preferOriginal); path != "" {
			resolved = append(resolved, resolvedMedia{id: id, path: path})
		}
	}
	return resolved
}

func resolveMediaPathsWithClient(c *client.Client, mediaIDs string, preferOriginal bool) []resolvedMedia {
	ids := parseMediaIDs(mediaIDs)
	if len(ids) == 0 {
		return nil
	}

	resolved := make([]resolvedMedia, 0, len(ids))
	for _, id := range ids {
		path := resolveMediaPath(id, preferOriginal)
		if path == "" && c != nil {
			path = fetchAndCacheMedia(c, id, preferOriginal)
		}
		if path != "" {
			resolved = append(resolved, resolvedMedia{id: id, path: path})
		}
	}
	return resolved
}

func resolvePostMediaPathsWithClient(c *client.Client, postID int32, postType, mediaIDs string, preferOriginal bool) []resolvedMedia {
	if resolved := resolveMediaPathsWithClient(c, mediaIDs, preferOriginal); len(resolved) > 0 {
		return resolved
	}
	if postType != "image" || postID <= 0 {
		return nil
	}

	id := strconv.Itoa(int(postID))
	path := resolveMediaPath(id, preferOriginal)
	if path == "" && c != nil {
		path = fetchAndCacheMediaByPID(c, id, preferOriginal)
	}
	if path == "" {
		return nil
	}
	return []resolvedMedia{{id: id, path: path}}
}

func parseMediaIDs(mediaIDs string) []string {
	parts := strings.Split(mediaIDs, ",")
	ids := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		ids = append(ids, part)
	}
	return ids
}

func resolveMediaPath(id string, preferOriginal bool) string {
	key := id + "|" + strconv.FormatBool(preferOriginal)
	if cached, ok := mediaPathCache.Load(key); ok {
		return cached.(string)
	}

	dirs := []string{"data/thumbnails", "data/images"}
	if preferOriginal {
		dirs = []string{"data/images", "data/thumbnails"}
	}
	exts := []string{".webp", ".png", ".jpg", ".jpeg", ".gif"}

	for _, dir := range dirs {
		for _, ext := range exts {
			path := filepath.Join(tuiProjectRoot(), dir, id+ext)
			if _, err := os.Stat(path); err == nil {
				abs, absErr := filepath.Abs(path)
				if absErr == nil {
					mediaPathCache.Store(key, abs)
					return abs
				}
				mediaPathCache.Store(key, path)
				return path
			}
		}
	}

	mediaPathCache.Store(key, "")
	return ""
}

func clearMediaPathCache(id string) {
	mediaPathCache.Delete(id + "|true")
	mediaPathCache.Delete(id + "|false")
}

func mediaImageIDForPath(path string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(path))
	id := h.Sum32()
	if id == 0 {
		return 1
	}
	return id
}

func fetchAndCacheMedia(c *client.Client, id string, preferOriginal bool) string {
	if c == nil || strings.TrimSpace(id) == "" {
		return ""
	}

	type fetchTarget struct {
		url string
		dir string
	}

	targets := []fetchTarget{
		{
			url: "https://treehole.pku.edu.cn/chapi/api/v3/media/getThumbnail?id=" + id,
			dir: filepath.Join(tuiProjectRoot(), "data/thumbnails"),
		},
		{
			url: "https://treehole.pku.edu.cn/chapi/api/v3/media/getImageBinary?id=" + id,
			dir: filepath.Join(tuiProjectRoot(), "data/images"),
		},
	}
	if preferOriginal {
		targets[0], targets[1] = targets[1], targets[0]
	}

	for _, target := range targets {
		if path := fetchMediaToDir(c, id, target.url, target.dir); path != "" {
			clearMediaPathCache(id)
			return resolveMediaPath(id, preferOriginal)
		}
	}
	return ""
}

func fetchAndCacheMediaByPID(c *client.Client, pid string, preferOriginal bool) string {
	if c == nil || strings.TrimSpace(pid) == "" {
		return ""
	}

	targets := []struct {
		url string
		dir string
	}{
		{
			url: "https://treehole.pku.edu.cn/chapi/api/v3/media/getImageBinary?pid=" + pid,
			dir: filepath.Join(tuiProjectRoot(), "data/images"),
		},
	}
	if !preferOriginal {
		targets = append(targets, struct {
			url string
			dir string
		}{
			url: "https://treehole.pku.edu.cn/chapi/api/v3/media/getImageBinary?pid=" + pid,
			dir: filepath.Join(tuiProjectRoot(), "data/thumbnails"),
		})
	}

	for _, target := range targets {
		if path := fetchMediaToDir(c, pid, target.url, target.dir); path != "" {
			clearMediaPathCache(pid)
			return resolveMediaPath(pid, preferOriginal)
		}
	}
	return ""
}

func fetchMediaToDir(c *client.Client, id, mediaURL, dir string) string {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ""
	}

	req, err := http.NewRequest(http.MethodGet, mediaURL, nil)
	if err != nil {
		return ""
	}
	if auth := c.GetAuthorization(); auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	if token := c.GetXSRFToken(); token != "" {
		req.Header.Set("x-xsrf-token", token)
	}

	resp, err := c.GetHttpClient().Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil || len(body) == 0 {
		return ""
	}

	ext := mediaExtensionFromContentType(resp.Header.Get("Content-Type"))
	path := filepath.Join(dir, id+ext)
	if err := os.WriteFile(path, body, 0644); err != nil {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		return abs
	}
	return path
}

func mediaExtensionFromContentType(contentType string) string {
	switch strings.TrimSpace(strings.ToLower(contentType)) {
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	default:
		return ".jpg"
	}
}

func buildImageRows(count, width, cellSize int) int {
	if count <= 0 || width <= 0 || cellSize <= 0 {
		return 0
	}
	perRow := imagesPerRow(width, cellSize)
	rows := (count + perRow - 1) / perRow
	return rows * cellSize
}

func imagesPerRow(width, cellSize int) int {
	if width <= 0 || cellSize <= 0 {
		return 1
	}
	perRow := (width + imageCellGap) / (cellSize + imageCellGap)
	if perRow < 1 {
		return 1
	}
	return perRow
}

func buildImageLayout(images []resolvedMedia, width, cellSize, leftOffset, topOffset int) postImageLayout {
	if len(images) == 0 || width <= 0 || cellSize <= 0 {
		return postImageLayout{}
	}

	perRow := imagesPerRow(width, cellSize)
	totalRows := buildImageRows(len(images), width, cellSize)
	lines := make([][]byte, totalRows)
	for i := range lines {
		lines[i] = bytes.Repeat([]byte(" "), width)
	}

	placements := make([]imagePlacement, 0, len(images))
	for idx, image := range images {
		col := idx % perRow
		row := idx / perRow
		left := col * (cellSize + imageCellGap)
		top := row * cellSize
		placements = append(placements, imagePlacement{
			path: image.path,
			cols: cellSize,
			rows: cellSize,
			left: leftOffset + left,
			top:  topOffset + top,
		})
	}

	resultLines := make([]string, 0, len(lines))
	for _, line := range lines {
		resultLines = append(resultLines, string(line))
	}
	return postImageLayout{
		lines:      resultLines,
		placements: placements,
	}
}
