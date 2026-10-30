-- message-store: every chat message the user sent or received, one row each,
-- in time order, whatever app carried it. Applied on every start; every
-- statement is idempotent.

-- One chat: a direct conversation or a small group, on one platform.
CREATE TABLE IF NOT EXISTS conversations (
    seq          INTEGER PRIMARY KEY,          -- generates id
    id           TEXT NOT NULL UNIQUE,         -- conversation_000001
    platform     TEXT NOT NULL,                -- vocabulary.go Platforms
    source       TEXT NOT NULL,                -- the id namespace source_ref lives in (discord, skype…)
    source_ref   TEXT NOT NULL,                -- the chat's id there (a Discord channel id, a Skype ChatID…)
    title        TEXT NOT NULL DEFAULT '',     -- as the platform names it; empty for most direct chats
    is_group     INTEGER NOT NULL DEFAULT 0,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    UNIQUE (source, source_ref)
);

-- Who is in a conversation, by the handle the platform uses. The person is
-- people-store's id, set here once; a message's sender person is read through
-- this row, never stored on the message, so naming a handle names every
-- message it sent.
CREATE TABLE IF NOT EXISTS participants (
    conversation_id TEXT NOT NULL REFERENCES conversations(id),
    handle          TEXT NOT NULL,             -- platform user id, phone number, Skype name…
    display_name    TEXT NOT NULL DEFAULT '',  -- for display only; never joined on
    person_id       TEXT NOT NULL DEFAULT '',  -- person_000001, or '' when nobody is known
    is_self         INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (conversation_id, handle)
);
CREATE INDEX IF NOT EXISTS idx_participants_person ON participants(person_id) WHERE person_id != '';

CREATE TABLE IF NOT EXISTS messages (
    seq             INTEGER PRIMARY KEY,       -- generates id; FTS rowid
    id              TEXT NOT NULL UNIQUE,      -- chat_message_000001
    conversation_id TEXT NOT NULL REFERENCES conversations(id),
    sent_at         INTEGER NOT NULL,          -- unix milliseconds, UTC
    sender_handle   TEXT NOT NULL DEFAULT '',  -- participants.handle; '' for a system line
    kind            TEXT NOT NULL,             -- vocabulary.go MessageKinds
    body            TEXT NOT NULL DEFAULT '',
    attachments     TEXT NOT NULL DEFAULT '[]',-- JSON []Attachment
    reply_to_source_ref TEXT NOT NULL DEFAULT '',
    call_seconds    INTEGER NOT NULL DEFAULT 0,-- kind call: how long it lasted
    edited          INTEGER NOT NULL DEFAULT 0,
    deleted_on_platform INTEGER NOT NULL DEFAULT 0, -- someone deleted it there; the text is kept
    source          TEXT NOT NULL,             -- id namespace, as for conversations
    source_ref      TEXT NOT NULL,             -- the message's id there
    source_hash     TEXT NOT NULL DEFAULT '',  -- the importer's hash; same hash = skip
    imported_by     TEXT NOT NULL DEFAULT '',  -- message_import_000001 that last wrote it
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL,
    UNIQUE (source, source_ref)
);
CREATE INDEX IF NOT EXISTS idx_messages_time ON messages(sent_at, seq);
CREATE INDEX IF NOT EXISTS idx_messages_conversation_time ON messages(conversation_id, sent_at, seq);
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(conversation_id, sender_handle);

CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(body, content='messages', content_rowid='seq');
CREATE TRIGGER IF NOT EXISTS messages_fts_insert AFTER INSERT ON messages BEGIN
    INSERT INTO messages_fts(rowid, body) VALUES (new.seq, new.body);
END;
CREATE TRIGGER IF NOT EXISTS messages_fts_delete AFTER DELETE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, body) VALUES ('delete', old.seq, old.body);
END;
CREATE TRIGGER IF NOT EXISTS messages_fts_update AFTER UPDATE OF body ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, body) VALUES ('delete', old.seq, old.body);
    INSERT INTO messages_fts(rowid, body) VALUES (new.seq, new.body);
END;

-- One conversation on one local day: what a journal-store transcript entry
-- used to hold. The importer that knows the user's time zone on that day
-- sends the span; journal_entry_id is the transcript entry that now points
-- here, so a timeline that cites the entry still lands on these messages.
CREATE TABLE IF NOT EXISTS conversation_days (
    seq             INTEGER PRIMARY KEY,
    id              TEXT NOT NULL UNIQUE,      -- conversation_day_000001
    conversation_id TEXT NOT NULL REFERENCES conversations(id),
    day             TEXT NOT NULL,             -- YYYY-MM-DD, local to where the user lived
    time_zone       TEXT NOT NULL,             -- IANA zone the day is local to
    starts_at       INTEGER NOT NULL,          -- unix ms, inclusive
    ends_at         INTEGER NOT NULL,          -- unix ms, exclusive
    journal_source      TEXT NOT NULL DEFAULT '', -- the transcript's source in journal-store
    journal_source_ref  TEXT NOT NULL DEFAULT '', -- and its source_ref
    journal_entry_id    TEXT NOT NULL DEFAULT '', -- entry_000001 once linked
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL,
    UNIQUE (conversation_id, day)
);
CREATE INDEX IF NOT EXISTS idx_conversation_days_journal ON conversation_days(journal_source, journal_source_ref) WHERE journal_source != '';
CREATE INDEX IF NOT EXISTS idx_conversation_days_entry ON conversation_days(journal_entry_id) WHERE journal_entry_id != '';

CREATE TABLE IF NOT EXISTS imports (
    seq          INTEGER PRIMARY KEY,
    id           TEXT NOT NULL UNIQUE,         -- message_import_000001
    importer     TEXT NOT NULL,                -- the command that ran (message-import-skype…)
    description  TEXT NOT NULL DEFAULT '',     -- what it read, without private paths
    started_at   INTEGER NOT NULL,
    finished_at  INTEGER NOT NULL DEFAULT 0,
    counts       TEXT NOT NULL DEFAULT '{}'    -- JSON ImportCounts, summed over batches
);
