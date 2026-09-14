-- 0001_init creates the ENCRE schema (ENCRE_04 §6).
CREATE TABLE parents (
    id         TEXT PRIMARY KEY,
    email      TEXT UNIQUE NOT NULL,
    pass_hash  BLOB NOT NULL,
    totp_secret BLOB,
    created_at INTEGER NOT NULL
);

CREATE TABLE children (
    id                TEXT PRIMARY KEY,
    parent_id         TEXT NOT NULL REFERENCES parents(id) ON DELETE CASCADE,
    pseudo            TEXT NOT NULL,
    pattern_hash      BLOB NOT NULL,
    avatar            INTEGER NOT NULL,
    daily_limit_json  TEXT NOT NULL,
    rank              INTEGER NOT NULL,
    best_rank         INTEGER NOT NULL,
    prestige          INTEGER NOT NULL,
    boss_wins_at_rank INTEGER NOT NULL,
    weeks_at_rank     INTEGER NOT NULL,
    boss_fail_streak  INTEGER NOT NULL,
    kindness          REAL NOT NULL,
    base_json         TEXT NOT NULL,
    level_json        TEXT NOT NULL,
    xp_json           TEXT NOT NULL,
    levelup_w_json    TEXT NOT NULL,
    unlocked_json     TEXT NOT NULL,
    exploits_json     TEXT NOT NULL,
    boss_wins_total   INTEGER NOT NULL,
    settings_json     TEXT NOT NULL,
    created_at        INTEGER NOT NULL
);

CREATE INDEX idx_children_parent_id ON children(parent_id);

CREATE TABLE word_lists (
    id          TEXT PRIMARY KEY,
    child_id    TEXT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
    label       TEXT NOT NULL,
    share_code  TEXT NOT NULL,
    due_date    INTEGER,
    validated   INTEGER NOT NULL,
    created_at  INTEGER NOT NULL
);

CREATE INDEX idx_word_lists_child_id ON word_lists(child_id);

CREATE TABLE items (
    id             TEXT PRIMARY KEY,
    list_id        TEXT NOT NULL REFERENCES word_lists(id) ON DELETE CASCADE,
    kind           INTEGER NOT NULL,
    text           TEXT NOT NULL,
    targets_json   TEXT NOT NULL,
    colors_json    TEXT NOT NULL,
    rules_json     TEXT NOT NULL,
    family         TEXT NOT NULL,
    audio_path     TEXT NOT NULL,
    source         INTEGER NOT NULL,
    confidence     REAL NOT NULL,
    enabled        INTEGER NOT NULL
);

CREATE INDEX idx_items_list_id ON items(list_id);

CREATE TABLE sentences (
    id          TEXT PRIMARY KEY,
    item_id     TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    text        TEXT NOT NULL,
    target_form TEXT NOT NULL,
    audio_path  TEXT NOT NULL,
    approved    INTEGER NOT NULL
);

CREATE INDEX idx_sentences_item_id ON sentences(item_id);

CREATE TABLE word_states (
    child_id   TEXT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
    item_id    TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    state_json TEXT NOT NULL,
    PRIMARY KEY (child_id, item_id)
);

CREATE INDEX idx_word_states_child_id ON word_states(child_id);

CREATE TABLE runs (
    id             TEXT PRIMARY KEY,
    child_id       TEXT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
    started_at     INTEGER NOT NULL,
    finished_at    INTEGER,
    rank           INTEGER NOT NULL,
    deck_json      TEXT NOT NULL,
    targets_json   TEXT NOT NULL,
    talismans_json TEXT NOT NULL,
    rooms_json     TEXT NOT NULL,
    failed_at      INTEGER,
    applied        INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_runs_child_id_started_at ON runs(child_id, started_at);

CREATE TABLE attempts (
    run_id  TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    idx     INTEGER NOT NULL,
    item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    manche  INTEGER NOT NULL,
    blind   INTEGER NOT NULL,
    copy    INTEGER NOT NULL,
    correct INTEGER NOT NULL,
    typed   TEXT NOT NULL,
    millis  INTEGER NOT NULL,
    chips   REAL NOT NULL,
    mult    REAL NOT NULL,
    PRIMARY KEY (run_id, idx)
);

CREATE TABLE dictee_results (
    id         TEXT PRIMARY KEY,
    list_id    TEXT NOT NULL REFERENCES word_lists(id) ON DELETE CASCADE,
    item_id    TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    correct    INTEGER NOT NULL,
    entered_at INTEGER NOT NULL
);

CREATE TABLE sessions (
    token          TEXT PRIMARY KEY,
    kind           INTEGER NOT NULL,
    subject_id     TEXT NOT NULL,
    expires_at     INTEGER NOT NULL,
    totp_ok_until  INTEGER
);

CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE play_time (
    child_id      TEXT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
    day           INTEGER NOT NULL,
    seconds       INTEGER NOT NULL,
    bonus_seconds INTEGER NOT NULL,
    PRIMARY KEY (child_id, day)
);
