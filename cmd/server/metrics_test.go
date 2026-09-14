package main

import (
	"testing"

	"github.com/oioio-space/encre/server/store"
)

// seedMetricsFixture inserts the rows [queryAdminMetrics] reads from,
// respecting every foreign key the schema enforces (parents → children →
// word_lists → items → runs → attempts).
func seedMetricsFixture(t *testing.T, db *store.Store) {
	t.Helper()
	ctx := t.Context()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.DB().ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("seed fixture: %v", err)
		}
	}

	exec(`INSERT INTO parents (id, email, pass_hash, created_at) VALUES ('p1', 'a@example.com', x'00', 0)`)
	exec(`INSERT INTO children (id, parent_id, pseudo, pattern_hash, avatar, daily_limit_json, rank,
			best_rank, prestige, boss_wins_at_rank, weeks_at_rank, boss_fail_streak, kindness,
			base_json, level_json, xp_json, levelup_w_json, unlocked_json, exploits_json,
			boss_wins_total, settings_json, created_at)
		VALUES ('c1', 'p1', 'Mila', x'00', 0, '{}', 1, 1, 0, 0, 0, 0, 1,
			'{}', '{}', '{}', '{}', '{}', '{}', 0, '{}', 0)`)
	exec(`INSERT INTO children (id, parent_id, pseudo, pattern_hash, avatar, daily_limit_json, rank,
			best_rank, prestige, boss_wins_at_rank, weeks_at_rank, boss_fail_streak, kindness,
			base_json, level_json, xp_json, levelup_w_json, unlocked_json, exploits_json,
			boss_wins_total, settings_json, created_at)
		VALUES ('c2', 'p1', 'Théo', x'00', 0, '{}', 2, 2, 0, 0, 0, 0, 1,
			'{}', '{}', '{}', '{}', '{}', '{}', 0, '{}', 0)`)
	exec(`INSERT INTO word_lists (id, child_id, label, share_code, validated, created_at)
		VALUES ('l1', 'c1', 'liste', 'code', 1, 0)`)
	exec(`INSERT INTO items (id, list_id, kind, text, targets_json, colors_json, rules_json,
			family, audio_path, source, confidence, enabled)
		VALUES ('i1', 'l1', 0, 'cœur', '[]', '[]', '[]', '', '', 0, 1, 1)`)
	exec(`INSERT INTO runs (id, child_id, started_at, finished_at, rank, deck_json, targets_json,
			talismans_json, rooms_json, failed_at, applied)
		VALUES ('r1', 'c1', 0, 10, 1, '{}', '[]', '[]', '[]', 1, 1)`)
	exec(`INSERT INTO runs (id, child_id, started_at, finished_at, rank, deck_json, targets_json,
			talismans_json, rooms_json, failed_at, applied)
		VALUES ('r2', 'c1', 0, 10, 1, '{}', '[]', '[]', '[]', NULL, 1)`)
	exec(`INSERT INTO attempts (run_id, idx, item_id, manche, blind, copy, correct, typed, millis,
			chips, mult)
		VALUES ('r1', 0, 'i1', 0, 0, 0, 1, 'cœur', 1000, 10, 1)`)
	exec(`INSERT INTO attempts (run_id, idx, item_id, manche, blind, copy, correct, typed, millis,
			chips, mult)
		VALUES ('r1', 1, 'i1', 1, 1, 0, 0, 'coeur', 3000, 0, 0)`)
}

func TestQueryAdminMetrics(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	seedMetricsFixture(t, db)

	got, err := queryAdminMetrics(t.Context(), db.DB())
	if err != nil {
		t.Fatalf("queryAdminMetrics() error = %v", err)
	}

	wantRanks := []rankCount{{Rank: 1, Count: 1}, {Rank: 2, Count: 1}}
	if len(got.RankDistribution) != len(wantRanks) {
		t.Fatalf("RankDistribution = %+v, want %+v", got.RankDistribution, wantRanks)
	}
	for i, want := range wantRanks {
		if got.RankDistribution[i] != want {
			t.Errorf("RankDistribution[%d] = %+v, want %+v", i, got.RankDistribution[i], want)
		}
	}

	wantFailures := []mancheFailure{{Rank: 1, Manche: 1, Count: 1}}
	if len(got.FailuresByRankManche) != len(wantFailures) {
		t.Fatalf("FailuresByRankManche = %+v, want %+v", got.FailuresByRankManche, wantFailures)
	}
	if got.FailuresByRankManche[0] != wantFailures[0] {
		t.Errorf("FailuresByRankManche[0] = %+v, want %+v", got.FailuresByRankManche[0], wantFailures[0])
	}

	// Average is over correct attempts only: the single correct attempt at
	// 1000ms, not the incorrect one at 3000ms.
	if got.AvgMillisPerWord != 1000 {
		t.Errorf("AvgMillisPerWord = %v, want %v", got.AvgMillisPerWord, 1000.0)
	}

	// One of the two attempts was blind.
	if got.BlindShare != 0.5 {
		t.Errorf("BlindShare = %v, want %v", got.BlindShare, 0.5)
	}

	if len(got.Limitations) != len(unavailableMetrics) {
		t.Errorf("Limitations = %v, want %v", got.Limitations, unavailableMetrics)
	}
}

func TestQueryAdminMetricsEmptyDatabase(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	got, err := queryAdminMetrics(t.Context(), db.DB())
	if err != nil {
		t.Fatalf("queryAdminMetrics() error = %v", err)
	}
	if got.RankDistribution != nil {
		t.Errorf("RankDistribution = %v, want nil on an empty database", got.RankDistribution)
	}
	if got.BlindShare != 0 {
		t.Errorf("BlindShare = %v, want 0 on an empty database (no division by zero)", got.BlindShare)
	}
	if got.AvgMillisPerWord != 0 {
		t.Errorf("AvgMillisPerWord = %v, want 0 on an empty database", got.AvgMillisPerWord)
	}
}
