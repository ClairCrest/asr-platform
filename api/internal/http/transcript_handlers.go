package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/ClairCrest/asr-platform/api/internal/http/dto"
	"github.com/ClairCrest/asr-platform/api/internal/http/middleware"
	"github.com/ClairCrest/asr-platform/api/internal/job"
)

// englishConfidenceThreshold mirrors worker.transcribe.ENGLISH_CONFIDENCE_THRESHOLD.
// Below this, the audio very likely is not English and the dashboard
// should show a banner saying so rather than silently trusting the text.
const englishConfidenceThreshold = 0.5

// GetTranscript serves the finished transcript in one of four formats
// selected by ?format=, defaulting to json.
func (h *JobHandler) GetTranscript(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	jobID, err := parseUUIDParam(r, "id")
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid job id")
		return
	}

	transcript, segments, err := h.svc.GetTranscript(r.Context(), userID, jobID)
	if err != nil {
		writeJobError(w, r, err)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	switch format {
	case "json":
		WriteJSON(w, http.StatusOK, toTranscriptResponse(transcript, segments))
	case "txt":
		writeText(w, "text/plain; charset=utf-8", transcript.Text)
	case "srt":
		writeText(w, "application/x-subrip", toSRT(segments))
	case "vtt":
		writeText(w, "text/vtt; charset=utf-8", toVTT(segments))
	default:
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "format must be one of json, txt, srt, vtt")
	}
}

func writeText(w http.ResponseWriter, contentType, body string) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func toTranscriptResponse(t job.Transcript, segments []job.Segment) dto.TranscriptResponse {
	resp := dto.TranscriptResponse{
		Text:                t.Text,
		LanguageDetected:    t.LanguageDetected,
		LanguageProbability: t.LanguageProbability,
		LanguageWarning:     t.LanguageProbability < englishConfidenceThreshold,
		Model:               t.Model,
		ProcessingSeconds:   t.ProcessingSeconds,
		RealTimeFactor:      t.RealTimeFactor,
		CreatedAt:           t.CreatedAt,
	}
	for _, s := range segments {
		resp.Segments = append(resp.Segments, dto.SegmentResponse{
			Idx:        s.Idx,
			StartMs:    s.StartMs,
			EndMs:      s.EndMs,
			Text:       s.Text,
			AvgLogprob: s.AvgLogprob,
		})
	}
	return resp
}

func toSRT(segments []job.Segment) string {
	var b strings.Builder
	for i, s := range segments {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, srtTimestamp(s.StartMs), srtTimestamp(s.EndMs), s.Text)
	}
	return b.String()
}

func toVTT(segments []job.Segment) string {
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for _, s := range segments {
		fmt.Fprintf(&b, "%s --> %s\n%s\n\n", vttTimestamp(s.StartMs), vttTimestamp(s.EndMs), s.Text)
	}
	return b.String()
}

// srtTimestamp formats milliseconds as SRT's HH:MM:SS,mmm.
func srtTimestamp(ms int32) string {
	return formatTimestamp(ms, ",")
}

// vttTimestamp formats milliseconds as WebVTT's HH:MM:SS.mmm.
func vttTimestamp(ms int32) string {
	return formatTimestamp(ms, ".")
}

func formatTimestamp(ms int32, fracSep string) string {
	total := int64(ms)
	hours := total / 3_600_000
	total %= 3_600_000
	minutes := total / 60_000
	total %= 60_000
	seconds := total / 1000
	millis := total % 1000
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", hours, minutes, seconds, fracSep, millis)
}
