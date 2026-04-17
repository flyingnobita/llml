package tui

import (
	"strings"
	"testing"
)

func TestNormalizeSplitServerLogLine_structuredUpdatesWidth(t *testing.T) {
	align := 0
	structured := "(EngineCore pid=1234567) INFO 04-14 12:34:56 [loader.py:45] loading"
	got := normalizeSplitServerLogLine(structured, &align)
	if got != structured {
		t.Fatalf("structured line should be unchanged: %q", got)
	}
	if align <= 0 {
		t.Fatalf("expected positive align width, got %d", align)
	}
}

func TestNormalizeSplitServerLogLine_tqdmPadsToStructuredWidth(t *testing.T) {
	align := 0
	_ = normalizeSplitServerLogLine("(EngineCore pid=1) INFO 04-14 12:34:56 [x.py:1] msg", &align)
	tqdm := "Loading shards:   0% Completed | 0/1 [00:00<?, ?it/s]"
	got := normalizeSplitServerLogLine(tqdm, &align)
	if !strings.HasPrefix(got, strings.Repeat(" ", align)) {
		t.Fatalf("expected tqdm line padded to width %d, got %q", align, got)
	}
	if strings.TrimLeft(got, " ") != tqdm {
		t.Fatalf("padding should only be leading spaces")
	}
}

func TestNormalizeSplitServerLogLine_invocationEchoUnchanged(t *testing.T) {
	align := 0
	line := "+ '/bin/vllm' serve '/path' --served-model-name 'id' --port 8000"
	got := normalizeSplitServerLogLine(line, &align)
	if got != line {
		t.Fatalf("got %q want %q", got, line)
	}
	if align != 0 {
		t.Fatalf("align should stay 0, got %d", align)
	}
}

func TestNormalizeSplitServerLogLine_plainLlamaLineNotPaddedEarly(t *testing.T) {
	align := 0
	line := "print_info: file format = GGUF"
	got := normalizeSplitServerLogLine(line, &align)
	if got != line {
		t.Fatalf("unexpected padding for plain line: %q", got)
	}
}

func TestLooksLikeUnprefixedProgressLine(t *testing.T) {
	if !looksLikeUnprefixedProgressLine("x: 100%| 1/1 [00:01<00:00, 1.25it/s]") {
		t.Fatal("expected tqdm line to match")
	}
	if looksLikeUnprefixedProgressLine("random stderr") {
		t.Fatal("should not match arbitrary text")
	}
}

func TestNormalizeSplitServerLogLine_carriageReturns(t *testing.T) {
	align := 0
	cases := []struct {
		name string
		in   string
		out  string
	}{
		{"leading", "\rtqdm: 0%", "tqdm: 0%"},
		{"trailing", "tqdm: 0%\r", "tqdm: 0%"},
		{"multiple", "\r0%\r10%\r20%", "20%"},
		{"with padding setup", "plain", "plain"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizeSplitServerLogLine(c.in, &align)
			// For unprefixed strings without padding setup, it leaves them alone unless it matches progress line,
			// or if align=0 and it is just plain string, it returns plain.
			// The CRs should be stripped in all cases.
			if !strings.Contains(got, c.out) || strings.Contains(got, "\r") {
				t.Fatalf("expected output to contain %q without \\r, got %q", c.out, got)
			}
		})
	}
}
