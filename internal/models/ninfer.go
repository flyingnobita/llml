package models

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ninferExt is the file extension of a native NInfer artifact.
const ninferExt = ".ninfer"

// ninferMagic opens every NInfer container. The trailing two bytes are the
// container version; only the "NINFER\x00" prefix is checked so a newer
// version is still recognized as an NInfer file.
//
// Source: tools/artifact/framing.py in https://github.com/Neroued/ninfer
var ninferMagic = []byte("NINFER\x00")

// ninferHeaderSize is the fixed entry header: 8-byte magic, little-endian
// uint64 manifest length, then a 16-byte identifier.
const ninferHeaderSize = 8 + 8 + 16

// ninferMaxManifestBytes bounds how much of a file is read as its JSON
// manifest. Real manifests list every tensor and run to a few hundred KiB; the
// cap keeps a corrupt length field from allocating gigabytes.
const ninferMaxManifestBytes = 16 << 20

// ninferManifest holds the manifest fields shown in the Parameters column.
type ninferManifest struct {
	Components map[string]struct {
		Config struct {
			ModelType     string   `json:"model_type"`
			Architectures []string `json:"architectures"`
		} `json:"config"`
	} `json:"components"`
}

type ninferSource struct{}

func (ninferSource) match(full, _ string, _ os.DirEntry) string {
	if strings.EqualFold(filepath.Ext(full), ninferExt) {
		return filepath.Clean(full)
	}
	return ""
}

func (ninferSource) build(path string) (ModelFile, bool) {
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return ModelFile{}, false
	}
	return ModelFile{
		Backend:    BackendNInfer,
		ID:         path,
		Path:       path,
		Location:   path,
		Name:       filepath.Base(path),
		Size:       fi.Size(),
		ModTime:    fi.ModTime(),
		Parameters: ninferParamsSummary(path),
	}, true
}

// ninferParamsSummary returns "ninfer · model_type · Architecture" for the text
// component of an NInfer artifact, or "ninfer · —" when the manifest cannot be
// read. Components other than text (vision, mtp, dflash2) are sidecars loaded
// at startup and do not describe the model.
//
//nolint:gosec // G304: path from model discovery — trusted source.
func ninferParamsSummary(path string) string {
	const unknown = "ninfer · —"
	f, err := os.Open(path)
	if err != nil {
		return unknown
	}
	defer func() { _ = f.Close() }()
	m, ok := readNInferManifest(f)
	if !ok {
		return unknown
	}
	text, ok := m.Components["text"]
	if !ok {
		return unknown
	}
	parts := []string{"ninfer"}
	if t := strings.TrimSpace(text.Config.ModelType); t != "" {
		parts = append(parts, t)
	}
	if len(text.Config.Architectures) > 0 {
		if a := strings.TrimSpace(text.Config.Architectures[0]); a != "" {
			parts = append(parts, a)
		}
	}
	if len(parts) == 1 {
		return unknown
	}
	return strings.Join(parts, " · ")
}

// readNInferManifest reads the container header and decodes the JSON manifest
// that follows it. It reports false for anything that is not an NInfer file.
func readNInferManifest(r io.Reader) (ninferManifest, bool) {
	var hdr [ninferHeaderSize]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return ninferManifest{}, false
	}
	if !bytes.HasPrefix(hdr[:], ninferMagic) {
		return ninferManifest{}, false
	}
	n := binary.LittleEndian.Uint64(hdr[8:16])
	if n == 0 || n > ninferMaxManifestBytes {
		return ninferManifest{}, false
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return ninferManifest{}, false
	}
	// The manifest is padded to the payload alignment with NUL bytes.
	buf = bytes.TrimRight(buf, "\x00 ")
	var m ninferManifest
	if err := json.Unmarshal(buf, &m); err != nil {
		return ninferManifest{}, false
	}
	return m, true
}
