# message-store routes

All JSON. Times are unix milliseconds, UTC. Errors are `{"error": "…"}`: 400 for
a bad request (an unknown field included), 404 for a missing id.

| Route | What it does |
|---|---|
| `GET /health` | `{"status":"ok"}` when the database answers |
| `GET /settings` | the declared settings and their values |
| `GET /vocabulary` | platforms, message kinds, activity groupings and intervals |
| `GET /summary` | conversations and messages per platform, with first and last times |
| `GET /messages` | messages in time order. `conversation` (repeatable), `person`, `platform`, `after`, `before`, `q` (full text), `order=oldest\|newest`, `limit` (≤5000, default 500), `cursor` (the last page's `next`) |
| `GET /messages/{id}` | one message |
| `GET /conversations` | most recently active first. `person`, `platform`, `q` (title or participant name), `limit` |
| `GET /conversations/{id}` | one conversation, participants and message summary |
| `PATCH /conversations/{id}/participants/{handle}` | `{"person_id":"person_000001"}` names the handle's person; `""` clears it |
| `GET /conversation-days` | `conversation`, `journal_entry`, `person`, `from`, `to` (YYYY-MM-DD), `limit` |
| `GET /conversation-days/{id}` | the day, its conversation and its messages |
| `POST /conversation-days/journal-entries` | `[{"journal_source","journal_source_ref","journal_entry_id"}]`: records which journal entry held each day's transcript; answers `linked` and `unmatched` |
| `GET /activity` | message counts per time bucket. `group_by=person\|conversation\|platform`, `interval=day\|week\|month\|year`, `time_zone` (IANA, required), `person`, `after`, `before` |
| `GET /imports`, `GET /imports/{id}` | import runs and what each wrote |
| `POST /imports` | `{"importer","description"}` starts a run |
| `POST /imports/{id}/batches` | `{"conversations","messages","conversation_days"}`, written in one transaction: one bad record writes nothing |
| `POST /imports/{id}/finish` | ends a run |

## Import rules

- A conversation is found by `(source, source_ref)`. An empty title in a batch
  keeps the stored one. Participants are added or updated by handle; a
  participant's `person_id` is written only when the batch gives one, so a
  person set by hand survives an import that knows nobody.
- A message is found by `(source, source_ref)`. The same `source_hash` skips it;
  another hash rewrites it. A sender handle the conversation never listed gets a
  participant row.
- A conversation day is found by `(conversation, day)`; its span is computed from
  `day` and `time_zone`.

## Activity by person

A message counts for a person when that person sent it (`received`), or when
the user sent it in a direct conversation with that person (`sent`). The user's
messages in a group are credited to nobody.
