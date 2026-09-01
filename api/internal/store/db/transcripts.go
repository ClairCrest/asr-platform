package db

import (
	"context"

	"github.com/google/uuid"
)

const getTranscriptByJobID = `
SELECT id, job_id, text, language_detected, language_probability,
       model, processing_seconds, real_time_factor, created_at
FROM transcripts
WHERE job_id = $1
`

func (q *Queries) GetTranscriptByJobID(ctx context.Context, jobID uuid.UUID) (Transcript, error) {
	row := q.db.QueryRow(ctx, getTranscriptByJobID, jobID)
	var t Transcript
	err := row.Scan(
		&t.ID, &t.JobID, &t.Text, &t.LanguageDetected, &t.LanguageProbability,
		&t.Model, &t.ProcessingSeconds, &t.RealTimeFactor, &t.CreatedAt,
	)
	return t, err
}

const listSegmentsByTranscript = `
SELECT id, transcript_id, idx, start_ms, end_ms, text, avg_logprob
FROM segments
WHERE transcript_id = $1
ORDER BY idx ASC
`

func (q *Queries) ListSegmentsByTranscript(ctx context.Context, transcriptID uuid.UUID) ([]Segment, error) {
	rows, err := q.db.Query(ctx, listSegmentsByTranscript, transcriptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	segments := []Segment{}
	for rows.Next() {
		var s Segment
		if err := rows.Scan(&s.ID, &s.TranscriptID, &s.Idx, &s.StartMs, &s.EndMs, &s.Text, &s.AvgLogprob); err != nil {
			return nil, err
		}
		segments = append(segments, s)
	}
	return segments, rows.Err()
}
