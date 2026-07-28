package store

import (
	"database/sql"
	"time"

	"cortex-cc/internal/models"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite single-writer
	s := &Store{db: db}
	return s, s.migrate()
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
	CREATE TABLE IF NOT EXISTS calls (
		id            TEXT PRIMARY KEY,
		caller_number TEXT NOT NULL,
		caller_name   TEXT,
		agent_id      TEXT,
		queue_name    TEXT NOT NULL,
		status        TEXT NOT NULL,
		wait_seconds  INTEGER DEFAULT 0,
		talk_seconds  INTEGER DEFAULT 0,
		sentiment     REAL DEFAULT 0,
		sla_breached  INTEGER DEFAULT 0,
		flagged       INTEGER DEFAULT 0,
		flag_reason   TEXT,
		started_at    DATETIME NOT NULL,
		ended_at      DATETIME
	);

	CREATE TABLE IF NOT EXISTS agents (
		id              TEXT PRIMARY KEY,
		name            TEXT NOT NULL,
		status          TEXT NOT NULL,
		current_call_id TEXT,
		calls_handled   INTEGER DEFAULT 0,
		avg_handle_time REAL DEFAULT 0,
		skills          TEXT DEFAULT '[]'
	);

	CREATE TABLE IF NOT EXISTS transcripts (
		id        TEXT PRIMARY KEY,
		call_id   TEXT NOT NULL,
		speaker   TEXT NOT NULL,
		text      TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		FOREIGN KEY (call_id) REFERENCES calls(id)
	);

	CREATE TABLE IF NOT EXISTS call_summaries (
		call_id         TEXT PRIMARY KEY,
		issue           TEXT,
		resolution      TEXT,
		follow_up       TEXT,
		sentiment_label TEXT,
		created_at      DATETIME NOT NULL,
		FOREIGN KEY (call_id) REFERENCES calls(id)
	);

	CREATE TABLE IF NOT EXISTS call_scores (
		call_id          TEXT PRIMARY KEY,
		empathy          INTEGER NOT NULL,
		resolution       INTEGER NOT NULL,
		professionalism  INTEGER NOT NULL,
		overall          INTEGER NOT NULL,
		notes            TEXT,
		scored_at        DATETIME NOT NULL,
		FOREIGN KEY (call_id) REFERENCES calls(id)
	);
	`)
	return err
}

// ── Calls ──────────────────────────────────────────────────────────────────

func (s *Store) UpsertCall(c *models.Call) error {
	_, err := s.db.Exec(`
		INSERT INTO calls (id, caller_number, caller_name, agent_id, queue_name, status,
		                   wait_seconds, talk_seconds, sentiment, sla_breached, flagged,
		                   flag_reason, started_at, ended_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			agent_id=excluded.agent_id, status=excluded.status,
			wait_seconds=excluded.wait_seconds, talk_seconds=excluded.talk_seconds,
			sentiment=excluded.sentiment, sla_breached=excluded.sla_breached,
			flagged=excluded.flagged, flag_reason=excluded.flag_reason,
			ended_at=excluded.ended_at`,
		c.ID, c.CallerNumber, c.CallerName, c.AgentID, c.QueueName, string(c.Status),
		c.WaitSeconds, c.TalkSeconds, c.Sentiment, c.SLABreached, c.Flagged,
		c.FlagReason, c.StartedAt, c.EndedAt,
	)
	return err
}

func (s *Store) GetActiveCalls() ([]*models.Call, error) {
	rows, err := s.db.Query(`
		SELECT id, caller_number, caller_name, agent_id, queue_name, status,
		       wait_seconds, talk_seconds, sentiment, sla_breached, flagged, flag_reason, started_at
		FROM calls WHERE status NOT IN ('completed','abandoned')
		ORDER BY started_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCalls(rows)
}

func (s *Store) GetCallByID(id string) (*models.Call, error) {
	row := s.db.QueryRow(`
		SELECT id, caller_number, caller_name, agent_id, queue_name, status,
		       wait_seconds, talk_seconds, sentiment, sla_breached, flagged, flag_reason, started_at
		FROM calls WHERE id=?`, id)
	return scanCall(row)
}

func (s *Store) GetRecentCalls(limit int) ([]*models.Call, error) {
	rows, err := s.db.Query(`
		SELECT id, caller_number, caller_name, agent_id, queue_name, status,
		       wait_seconds, talk_seconds, sentiment, sla_breached, flagged, flag_reason, started_at
		FROM calls ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCalls(rows)
}

// ── Agents ─────────────────────────────────────────────────────────────────

func (s *Store) UpsertAgent(a *models.Agent) error {
	skills := "[]"
	if len(a.Skills) > 0 {
		skills = `["` + join(a.Skills, `","`) + `"]`
	}
	_, err := s.db.Exec(`
		INSERT INTO agents (id, name, status, current_call_id, calls_handled, avg_handle_time, skills)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			status=excluded.status, current_call_id=excluded.current_call_id,
			calls_handled=excluded.calls_handled, avg_handle_time=excluded.avg_handle_time`,
		a.ID, a.Name, string(a.Status), a.CurrentCallID, a.CallsHandled, a.AvgHandleTime, skills,
	)
	return err
}

func (s *Store) GetAllAgents() ([]*models.Agent, error) {
	rows, err := s.db.Query(`
		SELECT id, name, status, current_call_id, calls_handled, avg_handle_time
		FROM agents ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var agents []*models.Agent
	for rows.Next() {
		a := &models.Agent{}
		var currentCallID sql.NullString
		if err := rows.Scan(&a.ID, &a.Name, &a.Status, &currentCallID, &a.CallsHandled, &a.AvgHandleTime); err != nil {
			return nil, err
		}
		a.CurrentCallID = currentCallID.String
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// ── Transcripts ────────────────────────────────────────────────────────────

func (s *Store) InsertTranscript(t *models.Transcript) error {
	_, err := s.db.Exec(`
		INSERT INTO transcripts (id, call_id, speaker, text, timestamp)
		VALUES (?,?,?,?,?)`,
		t.ID, t.CallID, t.Speaker, t.Text, t.Timestamp,
	)
	return err
}

func (s *Store) GetTranscriptByCallID(callID string) ([]*models.Transcript, error) {
	rows, err := s.db.Query(`
		SELECT id, call_id, speaker, text, timestamp
		FROM transcripts WHERE call_id=? ORDER BY timestamp ASC`, callID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ts []*models.Transcript
	for rows.Next() {
		t := &models.Transcript{}
		if err := rows.Scan(&t.ID, &t.CallID, &t.Speaker, &t.Text, &t.Timestamp); err != nil {
			return nil, err
		}
		ts = append(ts, t)
	}
	return ts, rows.Err()
}

// ── Summaries ──────────────────────────────────────────────────────────────

func (s *Store) InsertSummary(cs *models.CallSummary) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO call_summaries (call_id, issue, resolution, follow_up, sentiment_label, created_at)
		VALUES (?,?,?,?,?,?)`,
		cs.CallID, cs.Issue, cs.Resolution, cs.FollowUp, cs.Sentiment, cs.CreatedAt,
	)
	return err
}

func (s *Store) GetSummaryByCallID(callID string) (*models.CallSummary, error) {
	row := s.db.QueryRow(`
		SELECT call_id, issue, resolution, follow_up, sentiment_label, created_at
		FROM call_summaries WHERE call_id=?`, callID)
	cs := &models.CallSummary{}
	err := row.Scan(&cs.CallID, &cs.Issue, &cs.Resolution, &cs.FollowUp, &cs.Sentiment, &cs.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return cs, err
}

// ── Call Scores ────────────────────────────────────────────────────────────

func (s *Store) InsertCallScore(cs *models.CallScore) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO call_scores (call_id, empathy, resolution, professionalism, overall, notes, scored_at)
		VALUES (?,?,?,?,?,?,?)`,
		cs.CallID, cs.Empathy, cs.Resolution, cs.Professionalism, cs.Overall, cs.Notes, cs.ScoredAt,
	)
	return err
}

func (s *Store) GetCallScore(callID string) (*models.CallScore, error) {
	row := s.db.QueryRow(`
		SELECT call_id, empathy, resolution, professionalism, overall, notes, scored_at
		FROM call_scores WHERE call_id=?`, callID)
	cs := &models.CallScore{}
	err := row.Scan(&cs.CallID, &cs.Empathy, &cs.Resolution, &cs.Professionalism, &cs.Overall, &cs.Notes, &cs.ScoredAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return cs, err
}

func (s *Store) GetRecentCallScores(limit int) ([]*models.CallScore, error) {
	rows, err := s.db.Query(`
		SELECT call_id, empathy, resolution, professionalism, overall, notes, scored_at
		FROM call_scores ORDER BY scored_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var scores []*models.CallScore
	for rows.Next() {
		cs := &models.CallScore{}
		if err := rows.Scan(&cs.CallID, &cs.Empathy, &cs.Resolution, &cs.Professionalism, &cs.Overall, &cs.Notes, &cs.ScoredAt); err != nil {
			return nil, err
		}
		scores = append(scores, cs)
	}
	return scores, rows.Err()
}

// ── Queue Stats ────────────────────────────────────────────────────────────

func (s *Store) GetQueueStats() ([]*models.QueueStats, error) {
	rows, err := s.db.Query(`
		SELECT
			queue_name,
			COUNT(CASE WHEN status='active' THEN 1 END) as active,
			COUNT(CASE WHEN status='queued' THEN 1 END) as waiting,
			AVG(CASE WHEN status='queued' THEN wait_seconds END) as avg_wait,
			SUM(CASE WHEN sla_breached=1 AND status NOT IN ('completed','abandoned') THEN 1 ELSE 0 END) as breaches
		FROM calls
		WHERE status NOT IN ('completed','abandoned')
		GROUP BY queue_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stats []*models.QueueStats
	for rows.Next() {
		qs := &models.QueueStats{}
		var avgWait sql.NullFloat64
		if err := rows.Scan(&qs.QueueName, &qs.ActiveCalls, &qs.WaitingCalls, &avgWait, &qs.SLABreachCount); err != nil {
			return nil, err
		}
		qs.AvgWaitSeconds = avgWait.Float64
		stats = append(stats, qs)
	}
	return stats, rows.Err()
}

// ── helpers ────────────────────────────────────────────────────────────────

func scanCalls(rows *sql.Rows) ([]*models.Call, error) {
	var calls []*models.Call
	for rows.Next() {
		c, err := scanCallRow(rows)
		if err != nil {
			return nil, err
		}
		calls = append(calls, c)
	}
	return calls, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCall(row *sql.Row) (*models.Call, error) {
	return scanCallRow(row)
}

func scanCallRow(s scanner) (*models.Call, error) {
	c := &models.Call{}
	var agentID, callerName, flagReason sql.NullString
	var startedAt time.Time
	err := s.Scan(
		&c.ID, &c.CallerNumber, &callerName, &agentID, &c.QueueName, &c.Status,
		&c.WaitSeconds, &c.TalkSeconds, &c.Sentiment, &c.SLABreached, &c.Flagged,
		&flagReason, &startedAt,
	)
	if err != nil {
		return nil, err
	}
	c.AgentID = agentID.String
	c.CallerName = callerName.String
	c.FlagReason = flagReason.String
	c.StartedAt = startedAt
	return c, nil
}

func join(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
