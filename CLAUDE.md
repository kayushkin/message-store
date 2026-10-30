# About message-store

## What it owns

`127.0.0.1:8337`, loopback only. Every chat message the user sent or received, one row each, in time order, whatever app carried it: phone texts and calls, Skype, Messenger, Discord, and what multichat logs live. Three kinds of record, each registered in kanban-store's entity-type registry so a timeline or note cites one as a chip: **conversations** (`conversation_000001`, one chat on one platform, with participants by platform handle), **messages** (`chat_message_000001`) and **conversation days** (`conversation_day_000001`, one conversation on one local day). `CONTRACT.md` is the route table. **Messages are private**: dash is the front door and adds the auth this service has none of; never copy a message into noteboard or an agent's output unless the user asked.

# How it works

## Sources, handles and people

A conversation and a message are each found by `(source, source_ref)`, where `source` is the **id namespace**, not the importer: `discord` (channel id, message snowflake), `skype`, `facebook-messenger`, `sms`, `matrix`/`multichat`. Keep that rule: it is what keeps the same Discord message, arriving from a data package and from multichat's log, as one row. A participant row maps a handle to people-store's `person_id`; a message stores only its `sender_handle`, and its sender person is read through the participant row, so naming a handle once (`PATCH /conversations/{id}/participants/{handle}`) names every message it sent. An import writes `person_id` only when it has one, so a person set by hand survives.

## Superseded rows: one message, two ids

When an importer learns a message's real id after filing it under another namespace, it names the old key in the message's `supersedes`. The store has no delete: it copies the old row whole into `superseded_messages` (`GET /superseded-messages`), removes it from `messages`, and answers its old `chat_message_…` id with a 301 to the row that holds it now. So a chip that cited the old id still lands. Sending a superseded key again is a 400, which catches an importer that went back to the old key. First use: multichat's live Discord rows, filed by multichat ref until multichat gave them their Discord message id on 2026-10-30; `message-import-multichat` now files them by snowflake, joins the rows of one Discord message (its text and each attachment) into one message, and supersedes each row's ref.

## Calls with no texts

`message-import-android` gives a phone number the user called or was called by, but kept no texts with, a direct `sms` conversation of its calls alone (source_ref the E.164 number, as for texts; no conversation days, since the journal had none). A call from a private or unknown number is left out and counted.

## Conversation days and journal transcripts

Until 2026-10-30 chat history lived in journal-store as transcript entries, one per conversation per local day. Each importer sends a `conversation_day` for every day the old journal importer wrote, with the journal entry's exact `source` and `source_ref` and the IANA time zone the day was filed in; `POST /conversation-days/journal-entries` then records the `entry_…` id, and `GET /conversation-days?journal_entry=` answers it, so a timeline item that cites a transcript entry still lands on the messages.

# Access and operations

## Who may call it

No authentication, so it binds 127.0.0.1 only (`MESSAGE_STORE_ADDR` in the unit; `deploy.sh` fails on any other bind). Unit `message-store.service` (user unit), database `~/.config/message-store/message-store.db` (directory 0700, file 0600). healthcheck watches the unit.

# Working in this repo

## Build, test and deploy

Module `github.com/kayushkin/message-store`, root package `messagestore`, server `cmd/message-store`, importers `cmd/message-import-*` posting through `internal/importclient`. ⚠️ **`go test` and `go build` need `-tags sqlite_fts5`**; `deploy.sh` sets it, and installs every command under `cmd/` to `~/bin`. Every environment variable is declared once in `settings.go` with llm-bridge `servicesettings`: the service's in `SettingDefinitions`, the importers' in `ImporterSettingDefinitions`; `TestEveryEnvironmentVariableTheServiceReadsIsDeclared` fails on an undeclared `os.Getenv`. `./generate-ts.sh` renders `message.go` and `vocabulary.go` to `ts/model.ts`, published source-only as `@kayushkin/message-store-types`.

## Public repository: nothing personal

This repo may be made public. Nothing about the user's messages goes in it: no message text, names of people or chats, handles, phone numbers, account ids, archive paths or home directories, in code, tests, comments or commit messages. Tests use made-up data (`Carol`, `Me Self`, `Europe/Lisbon`). A fact an agent needs about this host's archives goes in a noteboard note tagged `message-store`.
