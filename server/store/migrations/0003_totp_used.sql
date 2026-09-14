-- 0003_totp_used replaces server/auth's monotonic totp_last_step counter
-- (0002) with a set of consumed steps: a race between two concurrent
-- verifications of the same code is only closed by a single atomic
-- statement, and a SELECT-then-UPDATE pair across two connections (as the
-- counter required) is not one. INSERT ... ON CONFLICT DO NOTHING against
-- this table's primary key is: exactly one of two concurrent inserts for the
-- same (parent_id, step) reports a row affected, no matter how the two
-- connections interleave.
--
-- This also fixes a clock-rollback failure mode the counter had: after the
-- system clock ever moved backward, every otherwise-correct code was
-- rejected for as long as the current step stayed below the counter's high
-- watermark, with no error to diagnose it by. A set of specific steps has no
-- such watermark — a step is either in the set or it isn't.
--
-- seen_at exists only so a periodic sweep (server/auth.PurgeExpiredTOTPUses)
-- can delete rows once they are older than any skew this package allows;
-- 0002's totp_last_step column is left in place, unused, per review: this
-- migration does not touch any that came before it.
CREATE TABLE totp_used (
    parent_id TEXT NOT NULL REFERENCES parents(id) ON DELETE CASCADE,
    step      INTEGER NOT NULL,
    seen_at   INTEGER NOT NULL,
    PRIMARY KEY (parent_id, step)
);
