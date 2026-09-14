-- 0002_totp_replay adds the bookkeeping server/auth needs to refuse a TOTP
-- code that was already accepted once, inside the +/-1 period skew ENCRE_04
-- §7 requires. It stores the last accepted 30-second step (unix time / 30)
-- rather than the code itself: steps only increase, so "reject a step at or
-- before the last accepted one" is a full replay check without ever keeping
-- a code, valid or not, around.
ALTER TABLE parents ADD COLUMN totp_last_step INTEGER NOT NULL DEFAULT 0;
