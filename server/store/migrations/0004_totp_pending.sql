-- 0004_totp_pending closes encre-qpx.7: EnrollTOTP used to hand the parent's
-- browser a plaintext secret to be echoed back on the verification POST,
-- with no server-side place to hold it in between. The natural
-- implementation of that gap is a hidden form field or a cookie — and a TOTP
-- secret in either one leaks into browser history, the back/forward cache,
-- access logs (if the POST is ever replayed as a GET) and Referer headers on
-- any third-party resource the page loads. The second factor would be
-- compromised before it was even confirmed.
--
-- totp_pending holds the secret server-side instead, keyed by the enrolling
-- parent, for the few minutes between BeginTOTPEnrollment rendering the QR
-- code and CompleteTOTPEnrollment consuming (and deleting) the row. The
-- secret never touches the client except inside the QR code's PNG.
CREATE TABLE totp_pending (
    parent_id  TEXT PRIMARY KEY REFERENCES parents(id) ON DELETE CASCADE,
    secret     BLOB NOT NULL,
    expires_at INTEGER NOT NULL
);
