CREATE TABLE segments (
    id            uuid PRIMARY KEY,
    transcript_id uuid NOT NULL REFERENCES transcripts(id) ON DELETE CASCADE,
    idx           int NOT NULL,
    start_ms      int NOT NULL,
    end_ms        int NOT NULL,
    text          text NOT NULL,
    avg_logprob   double precision,
    UNIQUE (transcript_id, idx)
);

CREATE INDEX segments_transcript_id_idx ON segments(transcript_id);
