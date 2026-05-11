package profiles

import (
	"fmt"
	"strings"
)

// PreviewOpts controls which fields are rendered by FormatPortablePreview.
type PreviewOpts struct {
	// TargetModel is the local model path the profile will attach to.
	// When empty, the "Target model:" line is omitted (--dry-run path).
	TargetModel string
}

// maxPreviewItems is the maximum number of args/env entries shown before
// truncating with "(+ N more)".
const maxPreviewItems = 6

// maxEnvValueLen is the maximum length of an env var value in the preview.
const maxEnvValueLen = 40

// FormatPortablePreview formats a PortableFile for display before import.
// The output includes one block per profile. When opts.TargetModel is set,
// a "Target model:" line is appended to each profile block.
func FormatPortablePreview(f *PortableFile, opts PreviewOpts) string {
	var b strings.Builder

	for i, p := range f.Profiles {
		if i > 0 {
			b.WriteString("\n---\n\n")
		}

		fmt.Fprintf(&b, "Profile: %s\n", p.Name)
		fmt.Fprintf(&b, "Backend: %s\n", p.Backend)
		if p.ModelHint != "" {
			fmt.Fprintf(&b, "Model hint: %s\n", p.ModelHint)
		}

		// Args
		if len(p.Args) > 0 {
			shown := p.Args
			more := ""
			if len(shown) > maxPreviewItems {
				more = fmt.Sprintf(" (+%d more)", len(shown)-maxPreviewItems)
				shown = shown[:maxPreviewItems]
			}
			fmt.Fprintf(&b, "Args (%d): %s%s\n", len(p.Args), strings.Join(shown, ", "), more)
		}

		// Env
		if len(p.Env) > 0 {
			shown := p.Env
			more := ""
			if len(shown) > maxPreviewItems {
				more = fmt.Sprintf(" (+%d more)", len(shown)-maxPreviewItems)
				shown = shown[:maxPreviewItems]
			}
			var parts []string
			for _, e := range shown {
				v := e.Value
				if len(v) > maxEnvValueLen {
					v = v[:maxEnvValueLen] + "…"
				}
				parts = append(parts, e.Key+"="+v)
			}
			fmt.Fprintf(&b, "Env (%d): %s%s\n", len(p.Env), strings.Join(parts, ", "), more)
		}

		if opts.TargetModel != "" {
			fmt.Fprintf(&b, "Target model: %s\n", opts.TargetModel)
		}
	}

	return b.String()
}
