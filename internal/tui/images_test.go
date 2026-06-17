package tui

import (
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
)

func newTestKittyRenderer() *KittyImageRenderer {
	return &KittyImageRenderer{
		imageIDs:         make(map[string]uint32),
		uploaded:         make(map[uint32]bool),
		activePlacements: make(map[uint32]activePlacement),
	}
}

func TestKittyRendererUploadsAndPlacesImage(t *testing.T) {
	renderer := newTestKittyRenderer()
	placement := imagePlacement{path: "/tmp/a.jpg", cols: 12, rows: 8, left: 4, top: 6, z: 1}
	imageID := mediaImageIDForPath(placement.path)

	renderer.SetFrame([]imagePlacement{placement})
	seq, _ := renderer.frame()

	wantUpload := "\x1b_Gq=2,i=" + strconv.FormatUint(uint64(imageID), 10) + ",t=f,f=100;" + base64.StdEncoding.EncodeToString([]byte(placement.path))
	if !strings.Contains(seq, wantUpload) {
		t.Fatalf("frame missing upload sequence:\n%s", seq)
	}
	if strings.Contains(seq, "a=t") {
		t.Fatalf("frame should not use invalid kitty action a=t:\n%s", seq)
	}
	if !strings.Contains(seq, "\x1b[7;5H\x1b_Gq=2,a=p,i="+strconv.FormatUint(uint64(imageID), 10)+",p=1,c=12,r=8,C=1,z=1\x1b\\") {
		t.Fatalf("frame missing placement sequence:\n%s", seq)
	}
	if strings.Contains(seq, "a=d") {
		t.Fatalf("frame should not clear all placements:\n%s", seq)
	}
}

func TestKittyRendererKeepsStablePlacementWithoutReupload(t *testing.T) {
	renderer := newTestKittyRenderer()
	placement := imagePlacement{path: "/tmp/a.jpg", cols: 12, rows: 8, left: 4, top: 6, z: 1}

	renderer.SetFrame([]imagePlacement{placement})
	renderer.SetFrame([]imagePlacement{placement})
	seq, _ := renderer.frame()

	if seq != "" {
		t.Fatalf("unchanged frame should emit no commands, got:\n%s", seq)
	}
}

func TestKittyRendererReplacesPlacementWhenImageChanges(t *testing.T) {
	renderer := newTestKittyRenderer()
	firstPath := "/tmp/a.jpg"
	secondPath := "/tmp/b.jpg"
	firstID := mediaImageIDForPath(firstPath)
	secondID := mediaImageIDForPath(secondPath)
	renderer.SetFrame([]imagePlacement{{path: firstPath, cols: 12, rows: 8, left: 4, top: 6}})

	renderer.SetFrame([]imagePlacement{{path: secondPath, cols: 12, rows: 8, left: 4, top: 6}})
	seq, _ := renderer.frame()

	if !strings.Contains(seq, "\x1b_Gq=2,a=d,d=i,i="+strconv.FormatUint(uint64(firstID), 10)+",p=1\x1b\\") {
		t.Fatalf("frame missing delete-old-placement sequence:\n%s", seq)
	}
	if !strings.Contains(seq, "\x1b_Gq=2,i="+strconv.FormatUint(uint64(secondID), 10)+",t=f,f=100;") {
		t.Fatalf("frame missing second upload sequence:\n%s", seq)
	}
	if !strings.Contains(seq, "a=p,i="+strconv.FormatUint(uint64(secondID), 10)+",p=1") {
		t.Fatalf("frame missing replacement placement sequence:\n%s", seq)
	}
}

func TestKittyRendererDeletesPlacementWhenFrameCleared(t *testing.T) {
	renderer := newTestKittyRenderer()
	path := "/tmp/a.jpg"
	imageID := mediaImageIDForPath(path)
	renderer.SetFrame([]imagePlacement{{path: path, cols: 12, rows: 8, left: 4, top: 6}})

	renderer.SetFrame(nil)
	seq, _ := renderer.frame()

	if seq != "\x1b_Gq=2,a=d,d=i,i="+strconv.FormatUint(uint64(imageID), 10)+",p=1\x1b\\" {
		t.Fatalf("clear frame sequence = %q", seq)
	}
}

func TestKittenICatArgsUseUnicodePlaceholderBackend(t *testing.T) {
	placement := imagePlacement{path: "/tmp/a.jpg", cols: 12, rows: 8, left: 4, top: 6, z: 99, placeholder: true, winCols: 80, winRows: 24}
	args := kittenICatArgs(placement)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--unicode-placeholder") {
		t.Fatalf("args missing unicode placeholder: %v", args)
	}
	if !strings.Contains(joined, "--use-window-size 80,24,800,480") {
		t.Fatalf("args missing window size: %v", args)
	}
	if !strings.Contains(joined, "--place 12x8@4x6") {
		t.Fatalf("args missing place: %v", args)
	}
	if strings.Contains(joined, "--image-id") {
		t.Fatalf("args should not pin image-id for placeholder backend: %v", args)
	}
}
