package httpapi

import (
	"strings"
	"testing"

	"github.com/ClairCrest/asr-platform/api/internal/job"
)

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		ms      int32
		fracSep string
		want    string
	}{
		{0, ",", "00:00:00,000"},
		{1500, ",", "00:00:01,500"},
		{61_000, ",", "00:01:01,000"},
		{3_661_234, ".", "01:01:01.234"},
	}
	for _, tt := range tests {
		if got := formatTimestamp(tt.ms, tt.fracSep); got != tt.want {
			t.Errorf("formatTimestamp(%d, %q) = %q, want %q", tt.ms, tt.fracSep, got, tt.want)
		}
	}
}

func TestToSRT(t *testing.T) {
	segments := []job.Segment{
		{Idx: 0, StartMs: 0, EndMs: 1000, Text: "hello"},
		{Idx: 1, StartMs: 1000, EndMs: 2500, Text: "world"},
	}

	got := toSRT(segments)

	want := "1\n00:00:00,000 --> 00:00:01,000\nhello\n\n2\n00:00:01,000 --> 00:00:02,500\nworld\n\n"
	if got != want {
		t.Errorf("toSRT() = %q, want %q", got, want)
	}
}

func TestToVTT(t *testing.T) {
	segments := []job.Segment{
		{Idx: 0, StartMs: 0, EndMs: 1000, Text: "hello"},
	}

	got := toVTT(segments)

	if !strings.HasPrefix(got, "WEBVTT\n\n") {
		t.Errorf("toVTT() missing WEBVTT header: %q", got)
	}
	if !strings.Contains(got, "00:00:00.000 --> 00:00:01.000\nhello") {
		t.Errorf("toVTT() = %q, missing expected cue", got)
	}
}

func TestToTranscriptResponseLanguageWarning(t *testing.T) {
	confident := toTranscriptResponse(job.Transcript{LanguageProbability: 0.9}, nil)
	if confident.LanguageWarning {
		t.Error("expected no language warning at 0.9 probability")
	}

	unconfident := toTranscriptResponse(job.Transcript{LanguageProbability: 0.3}, nil)
	if !unconfident.LanguageWarning {
		t.Error("expected a language warning at 0.3 probability")
	}
}
