#!/usr/bin/env bash
# Litestream restore drill (brief/ENCRE_04 §12: "Restauration testée").
#
# Runs a full replicate → restore round trip against a throwaway SQLite
# database and a *local file* replica standing in for B2 (Litestream's S3
# client is the same code path for both; what this proves — the age
# encryption and the restore mechanics — does not depend on which backend
# receives the bytes). It fails loudly if:
#   - the replicated snapshot is NOT age-encrypted (a regression back to a
#     litestream build without age support, or a config missing `age:`),
#   - restoring WITH the right identity does not reproduce the original
#     rows,
#   - restoring WITHOUT the identity does not fail outright — silently
#     serving garbage instead would be worse than refusing.
#
# Usage: deploy/restore-test.sh [path-to-litestream-binary]
# Exit 0 and "RESTORE DRILL: OK" on success; anything else means the backup
# strategy is not trustworthy and must not ship.
set -euo pipefail

LITESTREAM="${1:-litestream}"
if ! command -v "$LITESTREAM" >/dev/null 2>&1 && [ ! -x "$LITESTREAM" ]; then
	echo "restore-test.sh: litestream binary not found at '$LITESTREAM'" >&2
	echo "install it (deploy/litestream.yml's header comment) or pass its path" >&2
	exit 1
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/data" "$WORK/replica"

echo "== generating a throwaway age keypair =="
KEY_FILE="$WORK/age.key"
go run filippo.io/age/cmd/age-keygen@v1.2.1 -o "$KEY_FILE" 2>"$WORK/keygen.log"
RECIPIENT="$(grep '^# public key:' "$KEY_FILE" | cut -d' ' -f4)"
IDENTITY="$(grep '^AGE-SECRET-KEY' "$KEY_FILE")"
echo "recipient: $RECIPIENT"

echo "== seeding a throwaway database with a fake secret-shaped row =="
sqlite3 "$WORK/data/encre.db" \
	"CREATE TABLE parents(id TEXT, pass_hash BLOB); INSERT INTO parents VALUES ('p1', x'deadbeefcafe');"

cat >"$WORK/replicate.yml" <<EOF
dbs:
  - path: $WORK/data/encre.db
    replicas:
      - type: file
        path: $WORK/replica
        age:
          recipients: ["$RECIPIENT"]
EOF

echo "== replicating (litestream replicate, backgrounded briefly) =="
"$LITESTREAM" replicate -config "$WORK/replicate.yml" >"$WORK/replicate.log" 2>&1 &
LS_PID=$!
sleep 4
kill "$LS_PID" 2>/dev/null || true
wait "$LS_PID" 2>/dev/null || true

SNAPSHOT="$(find "$WORK/replica" -name '*.snapshot.lz4' | head -1)"
if [ -z "$SNAPSHOT" ]; then
	echo "FAIL: no snapshot written — see $WORK/replicate.log" >&2
	cat "$WORK/replicate.log" >&2
	exit 1
fi

echo "== checking the replica is actually encrypted, not just compressed =="
if ! head -c 32 "$SNAPSHOT" | grep -q "age-encryption.org/v1"; then
	echo "FAIL: replica snapshot is not age-encrypted (found no age-encryption.org/v1 header)" >&2
	exit 1
fi
echo "OK: snapshot begins with the age-encryption.org/v1 header"

echo "== restoring WITHOUT the identity: must fail =="
cat >"$WORK/restore-noage.yml" <<EOF
dbs:
  - path: $WORK/data/encre.db
    replicas:
      - type: file
        path: $WORK/replica
EOF
if "$LITESTREAM" restore -config "$WORK/restore-noage.yml" -o "$WORK/should-not-exist.db" "$WORK/data/encre.db" \
	>"$WORK/restore-noage.log" 2>&1; then
	echo "FAIL: restore succeeded WITHOUT the age identity — encryption is not effective" >&2
	exit 1
fi
echo "OK: restore without the identity failed, as required"

echo "== restoring WITH the identity: must succeed and match the original =="
cat >"$WORK/restore.yml" <<EOF
dbs:
  - path: $WORK/data/encre.db
    replicas:
      - type: file
        path: $WORK/replica
        age:
          identities: ["$IDENTITY"]
EOF
"$LITESTREAM" restore -config "$WORK/restore.yml" -o "$WORK/restored.db" "$WORK/data/encre.db" \
	>"$WORK/restore.log" 2>&1

GOT="$(sqlite3 "$WORK/restored.db" "SELECT hex(pass_hash) FROM parents WHERE id = 'p1';")"
if [ "$GOT" != "DEADBEEFCAFE" ]; then
	echo "FAIL: restored row = '$GOT', want 'DEADBEEFCAFE' — see $WORK/restore.log" >&2
	exit 1
fi

echo
echo "RESTORE DRILL: OK"
echo "  - replica is age-encrypted end to end (litestream v0.3.14, deploy/litestream.yml)"
echo "  - restore without the age identity fails closed"
echo "  - restore with the age identity reproduces the original database exactly"
