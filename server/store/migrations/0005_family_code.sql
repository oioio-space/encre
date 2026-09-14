-- 0005_family_code closes encre-qpx.5's structural fix: a child used to be
-- addressable globally by pseudo alone, which has no uniqueness constraint
-- and is not scoped to a parent. Two children in different families sharing
-- a pseudo and, by coincidence, a pattern could land in each other's
-- account, silently; server/auth.LoginChild's fail-closed check on an
-- ambiguous match is the stopgap this migration replaces with a real fix.
--
-- family_code is a per-parent, unguessable token — the same shape as
-- word_lists.share_code — that scopes a child login to one family: the
-- login page is reached by a URL or cookie carrying it, the child picks
-- their own avatar from that family's [server/store.Store.ChildrenOfParent]
-- list, and only then types a pattern, checked against exactly one child
-- row rather than every row sharing a pseudo. It is not a secret in the
-- password sense (nothing sensitive follows from knowing it beyond a list
-- of first names and avatar indices, which the panel already shows any
-- authenticated parent) but it is still unguessable, so it is generated the
-- same way [server/parent.newID] generates every other opaque identifier —
-- not sequential, not derived from anything public.
ALTER TABLE parents ADD COLUMN family_code TEXT;
CREATE UNIQUE INDEX idx_parents_family_code ON parents(family_code);
