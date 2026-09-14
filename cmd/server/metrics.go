// Metrics for the parent-only dashboard of brief/ENCRE_04 §12: "tableau
// minimal en SQL (/admin/metrics, parent seulement) : échec par manche et
// rang, répartition des rangs, dorées/semaine, temps/mot, part de l'aveugle,
// revanches."
//
// Two of those six are not queryable against the schema of
// server/store/migrations/0001_init.sql as it stands today, and this file
// says so rather than fake a number:
//
//   - "dorées/semaine" (gold words per week) needs the week a word turned
//     gold. [engine.WordState] tracks Gold as a bare bool with no timestamp
//     of the transition, so a snapshot query can only ever report "gold
//     right now", never a weekly trend.
//   - "revanches" (how often a lost manche was replayed) is decided per run
//     by the client and checked by [engine.Apply], but the outcome — the
//     [engine.RevancheTaken] event — is returned to the client and never
//     written to runs; there is no column to count it from.
//
// Both are one migration and one write away (a `gold_since_week` column on
// word_states, a `revanche_json` column on runs) — that is server/store's
// call, not cmd/server's, so it is left as a documented gap instead of a
// silent one.
package main

import (
	"context"
	"database/sql"
	"fmt"
)

// rankCount is how many children, or how many failed runs, fall under one
// rank.
type rankCount struct {
	Rank  int `json:"rank"`
	Count int `json:"count"`
}

// mancheFailure is how many runs failed at a given rank and manche.
// [store.Run.FailedAt] already names the manche a run failed in (despite the
// SQL column's name, `failed_at` holds a manche index, not a timestamp), so
// this is an exact count, not an estimate.
type mancheFailure struct {
	Rank   int `json:"rank"`
	Manche int `json:"manche"`
	Count  int `json:"count"`
}

// adminMetrics is the full response of GET /admin/metrics.
type adminMetrics struct {
	// RankDistribution counts children currently at each rank.
	RankDistribution []rankCount `json:"rankDistribution"`
	// FailuresByRankManche counts failed runs by rank and the manche they
	// failed in.
	FailuresByRankManche []mancheFailure `json:"failuresByRankManche"`
	// AvgMillisPerWord is the mean time, in milliseconds, a correct attempt
	// took across every run ever recorded.
	AvgMillisPerWord float64 `json:"avgMillisPerWord"`
	// BlindShare is the fraction of attempts (0–1) played blind.
	BlindShare float64 `json:"blindShare"`
	// Limitations names the ENCRE_04 §12 metrics this response cannot
	// compute yet, and why — see this file's package doc comment.
	Limitations []string `json:"limitations"`
}

// unavailableMetrics lists the ENCRE_04 §12 metrics [queryAdminMetrics]
// cannot compute from the current schema; see this file's doc comment.
var unavailableMetrics = []string{
	"dorées/semaine : word_states ne garde pas la semaine où un mot est devenu Gold, seulement l'état courant",
	"revanches : l'événement RevancheTaken est renvoyé au client mais jamais écrit en base",
}

// queryAdminMetrics runs the ENCRE_04 §12 dashboard queries against db and
// assembles [adminMetrics]. It is read-only and safe to call on every
// request: the tables involved are small enough (one row per child, one per
// run, one per attempt) that a family server never notices the cost.
func queryAdminMetrics(ctx context.Context, db *sql.DB) (adminMetrics, error) {
	m := adminMetrics{Limitations: unavailableMetrics}

	var err error
	if m.RankDistribution, err = rankDistribution(ctx, db); err != nil {
		return adminMetrics{}, fmt.Errorf("rank distribution: %w", err)
	}
	if m.FailuresByRankManche, err = failuresByRankManche(ctx, db); err != nil {
		return adminMetrics{}, fmt.Errorf("failures by rank and manche: %w", err)
	}
	if m.AvgMillisPerWord, err = avgMillisPerWord(ctx, db); err != nil {
		return adminMetrics{}, fmt.Errorf("average time per word: %w", err)
	}
	if m.BlindShare, err = blindShare(ctx, db); err != nil {
		return adminMetrics{}, fmt.Errorf("blind share: %w", err)
	}
	return m, nil
}

func rankDistribution(ctx context.Context, db *sql.DB) ([]rankCount, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT rank, COUNT(*)
		FROM children
		GROUP BY rank
		ORDER BY rank`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // read-only cursor, nothing to report

	var out []rankCount
	for rows.Next() {
		var rc rankCount
		if err := rows.Scan(&rc.Rank, &rc.Count); err != nil {
			return nil, err
		}
		out = append(out, rc)
	}
	return out, rows.Err()
}

func failuresByRankManche(ctx context.Context, db *sql.DB) ([]mancheFailure, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT rank, failed_at, COUNT(*)
		FROM runs
		WHERE failed_at IS NOT NULL
		GROUP BY rank, failed_at
		ORDER BY rank, failed_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // read-only cursor, nothing to report

	var out []mancheFailure
	for rows.Next() {
		var mf mancheFailure
		if err := rows.Scan(&mf.Rank, &mf.Manche, &mf.Count); err != nil {
			return nil, err
		}
		out = append(out, mf)
	}
	return out, rows.Err()
}

func avgMillisPerWord(ctx context.Context, db *sql.DB) (float64, error) {
	var avg sql.NullFloat64
	err := db.QueryRowContext(ctx, `
		SELECT AVG(millis) FROM attempts WHERE correct = 1`).Scan(&avg)
	if err != nil {
		return 0, err
	}
	return avg.Float64, nil
}

func blindShare(ctx context.Context, db *sql.DB) (float64, error) {
	var total, blind int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(blind), 0) FROM attempts`).Scan(&total, &blind)
	if err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, nil
	}
	return float64(blind) / float64(total), nil
}
