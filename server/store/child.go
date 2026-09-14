package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/oioio-space/encre/engine"
)

// Child is one player's row (ENCRE_04 §6): the account fields the server owns
// directly, plus the engine.Child standing serialized into the *_json
// columns. Build one directly for the account fields, then call [Child.SetEngine]
// to attach the engine state before [Store.CreateChild] or [Store.SaveChild].
type Child struct {
	// ID is the child's opaque, caller-assigned identifier.
	ID string
	// ParentID is the owning parent; deleting that parent cascades to this
	// row and everything under it (ENCRE_04 §7).
	ParentID string
	Pseudo   string
	// PatternHash is the peppered argon2id hash of the child's tap pattern,
	// used in place of a password — see
	// [github.com/oioio-space/encre/server/auth.Pepper.PepperHash].
	PatternHash []byte
	Avatar      int
	// DailyLimit is the child's daily-play-time configuration. There is no
	// engine type for it yet, so it round-trips as opaque JSON.
	DailyLimit json.RawMessage
	// Exploits is the child's achievement/"exploits" progress. There is no
	// engine type for it yet, so it round-trips as opaque JSON.
	Exploits json.RawMessage
	// Settings is per-child, parent-configured settings (rules on/off,
	// bonus time, and the like) with no engine type yet, round-tripped as
	// opaque JSON.
	Settings  json.RawMessage
	CreatedAt time.Time

	engine engine.Child
}

// LogValue implements [log/slog.LogValuer], redacting PatternHash so a
// handler that logs a Child never writes a child's peppered pattern hash
// into a log file — one more secret ENCRE_04 §12's Litestream replication
// and pepper key never protects once it leaves the database (encre-qpx.8).
// Pseudo is not redacted: it is a first name or a made-up nickname, the
// same information the parent panel already shows over an authenticated
// session, not the kind of data this rule protects.
func (c *Child) LogValue() slog.Value {
	if c == nil {
		return slog.StringValue("<nil>")
	}
	return slog.GroupValue(
		slog.String("id", c.ID),
		slog.String("parentID", c.ParentID),
		slog.String("pseudo", c.Pseudo),
		slog.Bool("hasPatternHash", len(c.PatternHash) > 0),
		slog.Int("avatar", c.Avatar),
		slog.Time("createdAt", c.CreatedAt),
	)
}

// String implements [fmt.Stringer] with the same redaction as
// [Child.LogValue].
func (c *Child) String() string {
	if c == nil {
		return "<nil>"
	}
	return fmt.Sprintf("Child{ID: %q, ParentID: %q, Pseudo: %q, hasPatternHash: %t, Avatar: %d, CreatedAt: %s}",
		c.ID, c.ParentID, c.Pseudo, len(c.PatternHash) > 0, c.Avatar, c.CreatedAt)
}

// SetEngine attaches the engine standing (rank, mastery, unlocked Talismans…)
// that CreateChild and SaveChild persist into the *_json columns.
func (c *Child) SetEngine(e engine.Child) { c.engine = e }

// Engine returns the engine standing this row carries. It is only meaningful
// after [Store.ChildByID], [Store.ChildrenOfParent] or an explicit
// [Child.SetEngine] call.
func (c *Child) Engine() engine.Child { return c.engine }

// childBase bundles the rolling win rate with the rank cooldown week: both
// are small pieces of rank-cycle bookkeeping with no column of their own, so
// they share base_json rather than adding two single-purpose columns to a
// schema ENCRE_04 §6 fixes exactly.
type childBase struct {
	Wins, Total int
	LastRankW   int32
}

// childProfile bundles the per-Couleur affinity, the general skill and the
// learning rate: level_json's payload, alongside the per-Couleur level it
// already carries. All four feed the same PHat estimate (engine/target.go)
// and have no column of their own.
type childProfile struct {
	Level     map[engine.Color]int
	Aff       map[engine.Color]float64
	LearnRate float64
	Skill     float64
}

// CreateChild inserts c, serializing its attached engine state. It returns an
// error if c.ParentID names no parent.
func (s *Store) CreateChild(ctx context.Context, c *Child) error {
	cols, err := marshalChildEngine(c)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO children (
			id, parent_id, pseudo, pattern_hash, avatar, daily_limit_json,
			rank, best_rank, prestige, boss_wins_at_rank, weeks_at_rank, boss_fail_streak,
			kindness, base_json, level_json, xp_json, levelup_w_json, unlocked_json,
			exploits_json, boss_wins_total, settings_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.ParentID, c.Pseudo, orEmptyBytes(c.PatternHash), c.Avatar, orEmptyJSON(c.DailyLimit),
		cols.rank, cols.bestRank, cols.prestige, cols.bossWinsAtRank, cols.weeksAtRank, cols.bossFailStreak,
		cols.kindness, cols.base, cols.level, cols.xp, cols.levelupW, cols.unlocked,
		orEmptyJSON(c.Exploits), cols.bossWinsTotal, orEmptyJSON(c.Settings), c.CreatedAt.Unix())
	if err != nil {
		return fmt.Errorf("creating child: %w", err)
	}
	return nil
}

// SaveChild overwrites every column of an existing child row, including its
// engine state. It returns [ErrNotFound] if no child with c.ID exists.
func (s *Store) SaveChild(ctx context.Context, c *Child) error {
	cols, err := marshalChildEngine(c)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE children SET
			parent_id = ?, pseudo = ?, pattern_hash = ?, avatar = ?, daily_limit_json = ?,
			rank = ?, best_rank = ?, prestige = ?, boss_wins_at_rank = ?, weeks_at_rank = ?,
			boss_fail_streak = ?, kindness = ?, base_json = ?, level_json = ?, xp_json = ?,
			levelup_w_json = ?, unlocked_json = ?, exploits_json = ?, boss_wins_total = ?,
			settings_json = ?
		WHERE id = ?`,
		c.ParentID, c.Pseudo, orEmptyBytes(c.PatternHash), c.Avatar, orEmptyJSON(c.DailyLimit),
		cols.rank, cols.bestRank, cols.prestige, cols.bossWinsAtRank, cols.weeksAtRank,
		cols.bossFailStreak, cols.kindness, cols.base, cols.level, cols.xp,
		cols.levelupW, cols.unlocked, orEmptyJSON(c.Exploits), cols.bossWinsTotal,
		orEmptyJSON(c.Settings), c.ID)
	if err != nil {
		return fmt.Errorf("saving child: %w", err)
	}
	return requireRowAffected(result, "saving child")
}

// ChildByID returns the child with the given ID, or [ErrNotFound] if none
// exists.
func (s *Store) ChildByID(ctx context.Context, id string) (*Child, error) {
	return s.scanChild(s.db.QueryRowContext(ctx, childSelect+`WHERE id = ?`, id))
}

// ChildrenOfParent returns every child belonging to the given parent, in no
// particular order.
func (s *Store) ChildrenOfParent(ctx context.Context, parentID string) ([]*Child, error) {
	rows, err := s.db.QueryContext(ctx, childSelect+`WHERE parent_id = ?`, parentID)
	if err != nil {
		return nil, fmt.Errorf("querying children: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var children []*Child
	for rows.Next() {
		c, err := scanChildRow(rows)
		if err != nil {
			return nil, err
		}
		children = append(children, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating children: %w", err)
	}
	return children, nil
}

const childSelect = `
	SELECT id, parent_id, pseudo, pattern_hash, avatar, daily_limit_json,
		rank, best_rank, prestige, boss_wins_at_rank, weeks_at_rank, boss_fail_streak,
		kindness, base_json, level_json, xp_json, levelup_w_json, unlocked_json,
		exploits_json, boss_wins_total, settings_json, created_at
	FROM children `

// childScanner is what [sql.Row] and [sql.Rows] share of the Scan method,
// letting scanChildRow serve both a single-row and a multi-row query.
type childScanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanChild(row *sql.Row) (*Child, error) {
	c, err := scanChildRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func scanChildRow(row childScanner) (*Child, error) {
	var (
		c                                                                    Child
		createdAt                                                            int64
		dailyLimit, exploits, settings                                       string
		base, level, xp, levelupW, unlocked                                  string
		bossWinsAtRank, weeksAtRank, bossFailStreak, bossWinsTotal, prestige int
	)
	err := row.Scan(
		&c.ID, &c.ParentID, &c.Pseudo, &c.PatternHash, &c.Avatar, &dailyLimit,
		&c.engine.Rank, &c.engine.BestRank, &prestige, &bossWinsAtRank, &weeksAtRank, &bossFailStreak,
		&c.engine.Kindness, &base, &level, &xp, &levelupW, &unlocked,
		&exploits, &bossWinsTotal, &settings, &createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning child: %w", err)
	}
	c.DailyLimit = json.RawMessage(dailyLimit)
	c.Exploits = json.RawMessage(exploits)
	c.Settings = json.RawMessage(settings)
	c.engine.Prestige = prestige
	c.engine.BossWinsAtRank = bossWinsAtRank
	c.engine.WeeksAtRank = weeksAtRank
	c.engine.BossFailStreak = bossFailStreak
	c.engine.BossWinsTotal = bossWinsTotal
	c.CreatedAt = time.Unix(createdAt, 0).UTC()

	if err := unmarshalChildEngine(&c, base, level, xp, levelupW, unlocked); err != nil {
		return nil, err
	}
	return &c, nil
}

// childColumns are the marshaled forms of the account-level scalar and JSON
// columns that carry the engine state, ready to bind as query arguments.
type childColumns struct {
	rank, bestRank, prestige                    int
	bossWinsAtRank, weeksAtRank, bossFailStreak int
	bossWinsTotal                               int
	kindness                                    float64
	base, level, xp, levelupW, unlocked         string
}

func marshalChildEngine(c *Child) (childColumns, error) {
	e := c.engine

	baseJSON, err := json.Marshal(childBase{Wins: e.Base.Wins, Total: e.Base.Total, LastRankW: e.LastRankW})
	if err != nil {
		return childColumns{}, fmt.Errorf("marshaling base_json: %w", err)
	}
	levelJSON, err := json.Marshal(childProfile{Level: e.Level, Aff: e.Aff, LearnRate: e.LearnRate, Skill: e.Skill})
	if err != nil {
		return childColumns{}, fmt.Errorf("marshaling level_json: %w", err)
	}
	xpJSON, err := json.Marshal(e.XP)
	if err != nil {
		return childColumns{}, fmt.Errorf("marshaling xp_json: %w", err)
	}
	levelupWJSON, err := json.Marshal(e.NextLevelW)
	if err != nil {
		return childColumns{}, fmt.Errorf("marshaling levelup_w_json: %w", err)
	}
	unlockedJSON, err := json.Marshal(e.Unlocked)
	if err != nil {
		return childColumns{}, fmt.Errorf("marshaling unlocked_json: %w", err)
	}

	return childColumns{
		rank: e.Rank, bestRank: e.BestRank, prestige: e.Prestige,
		bossWinsAtRank: e.BossWinsAtRank, weeksAtRank: e.WeeksAtRank, bossFailStreak: e.BossFailStreak,
		bossWinsTotal: e.BossWinsTotal, kindness: e.Kindness,
		base: string(baseJSON), level: string(levelJSON), xp: string(xpJSON),
		levelupW: string(levelupWJSON), unlocked: string(unlockedJSON),
	}, nil
}

func unmarshalChildEngine(c *Child, base, level, xp, levelupW, unlocked string) error {
	var b childBase
	if err := json.Unmarshal([]byte(base), &b); err != nil {
		return fmt.Errorf("unmarshaling base_json: %w", err)
	}
	c.engine.Base = engine.RollingRate{Wins: b.Wins, Total: b.Total}
	c.engine.LastRankW = b.LastRankW

	var p childProfile
	if err := json.Unmarshal([]byte(level), &p); err != nil {
		return fmt.Errorf("unmarshaling level_json: %w", err)
	}
	c.engine.Level, c.engine.Aff, c.engine.LearnRate, c.engine.Skill = p.Level, p.Aff, p.LearnRate, p.Skill

	if err := json.Unmarshal([]byte(xp), &c.engine.XP); err != nil {
		return fmt.Errorf("unmarshaling xp_json: %w", err)
	}
	if err := json.Unmarshal([]byte(levelupW), &c.engine.NextLevelW); err != nil {
		return fmt.Errorf("unmarshaling levelup_w_json: %w", err)
	}
	if err := json.Unmarshal([]byte(unlocked), &c.engine.Unlocked); err != nil {
		return fmt.Errorf("unmarshaling unlocked_json: %w", err)
	}
	return nil
}

// orEmptyJSON returns "{}" for a nil or empty RawMessage so the column is
// always valid JSON, and b unchanged otherwise.
func orEmptyJSON(b json.RawMessage) string {
	if len(b) == 0 {
		return "{}"
	}
	return string(b)
}

// orEmptyBytes returns an empty, non-nil slice for nil input so a BLOB NOT
// NULL column binds a zero-length value instead of SQL NULL.
func orEmptyBytes(b []byte) []byte {
	if b == nil {
		return []byte{}
	}
	return b
}

// requireRowAffected returns [ErrNotFound] if result reports no row changed,
// wrapping op into any error checking RowsAffected itself produces.
func requireRowAffected(result sql.Result, op string) error {
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: checking rows affected: %w", op, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
