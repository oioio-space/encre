-- 0006_sessions_subject_id indexes sessions(subject_id) for
-- [server/store.Store.DeleteSessionsForSubject] (encre-qpx.6): a parent
-- changing a leaked password, re-enrolling TOTP, or deleting their account
-- must revoke every session for that subject in one statement, not scan the
-- whole table to find them. sessions.subject_id names a row in either
-- parents or children depending on kind, so it cannot carry a real foreign
-- key to either — revocation on account deletion is therefore a query this
-- index makes cheap, not a CASCADE the schema can express.
CREATE INDEX idx_sessions_subject_id ON sessions(subject_id);
