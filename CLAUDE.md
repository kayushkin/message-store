# About message-store

## What it owns

`127.0.0.1:8337`, loopback only. Every chat message the user sent or received, one row each, in time order, whatever app carried it: phone texts and calls, Skype, Messenger, Discord, and what multichat logs live. Three kinds of record, each registered in kanban-store's entity-type registry so a timeline or note cites one as a chip: **conversations** (`conversation_000001`, one chat on one platform, with participants by platform handle), **messages** (`chat_message_000001`) and **conversation days** (`conversation_day_000001`, one conversation on one local day). `CONTRACT.md` is the route table. **Messages are private**: dash is the front door and adds the auth this service has none of; never copy a message into noteboard or an agent's output unless the user asked.

# How it works

## Sources, handles and people

A conversation and a message are each found by `(source, source_ref)`, where `source` is the **id namespace**, not the importer: `discord` (channel id, message snowflake), `skype`, `facebook-messenger`, `sms`, `matrix`/`multichat`. Keep that rule: it is what keeps the same Discord message, arriving from a data package and from multichat's log, as one row. A participant row maps a handle to people-store's `person_id`; a message stores only its `sender_handle`, and its sender person is read through the participant row, so naming a handle once (`PATCH /conversations/{id}/participants/{handle}`) names every message it sent. An import writes `person_id` only when it has one, so a person set by hand survives.

## Superseded rows: one message, two ids

When an importer learns a message's real id after filing it under another namespace, it names the old key in the message's `supersedes`. The store has no delete: it copies the old row whole into `superseded_messages` (`GET /superseded-messages`), removes it from `messages`, and answers its old `chat_message_…` id with a 301 to the row that holds it now. So a chip that cited the old id still lands. A message sent again under a superseded key is skipped and counted (`messages_superseded_skipped`), so a windowed importer that still reads the old row does not fail. A batch's `supersessions` retire a row in favour of a message another importer stored, without touching that message. First use: multichat's live Discord rows, filed by multichat ref until multichat gave them their Discord message id on 2026-10-30; `message-import-multichat` now files them by snowflake, joins the rows of one Discord message (its text and each attachment) into one message, and supersedes each row's ref.

## Texts carried twice by multichat

Texts reach multichat by two paths, each with its own Matrix room per conversation: the mautrix Google Messages bridge (`gmessages`, exact times) and gmessages-web-bridge (`gmessagesweb`, times cut to the minute). Both rooms name Google's conversation id in their `m.bridge` channel (the bridge as `<login prefix>.<id>`, the web bridge as base64url protobuf with the id in field 2). `message-import-multichat` reads every row of both platforms each run, pairs a web row with the bridge row in the twin room that has the same text in the same minute, files it as the bridge row and supersedes the web row's ref (`cmd/message-import-multichat/google_messages_twins.go`). Each text is still kept once. Where the phone import (source `sms`) holds the same text, the phone copy wins: the importer reads the stored phone texts, matches a bridge text to the millisecond and a web text to the minute, holds each room to the one phone conversation most of its matches fall in, and retires the multichat row (and the rows it superseded) in favour of the phone row (`cmd/message-import-multichat/phone_copies.go`).

## One chat, one conversation: merges

A text chat can reach the store as three conversations: the phone thread (source `sms`, filed by the other people's sorted E.164 numbers), the mautrix room and the web bridge's room (source `matrix`). Since 2026-10-30 they are merged into one. **The store** keeps a merge as a row of `conversation_merges` (`GET /conversation-merges`): the merged conversation's messages, days and participants move to the survivor, its row and participants are kept whole, its id answers with a 301, and its `(source, source_ref)` resolves to the survivor on every later import, which never changes the survivor's title or kind (`store_merge.go`). **The importer decides**, by ids only (`cmd/message-import-multichat/conversation_merges.go`), on every run over every Google Messages room multichat lists: rooms with the same `google_messages_conversation_id` merge into the mautrix room (`google_conversation_id`); a direct room merges into the phone thread of its partner's number, and a group whose mautrix room's other members have exactly a phone group's numbers merges into it (`phone_numbers`); direct rooms in two Google conversations with the same people-store person merge when no phone thread takes them (`direct_person`). Numbers come from mautrix's puppet records through multichat's `GET /google-messages/phone-numbers`, else from the people-store person's phone identities and their mautrix puppets' numbers; the user's own numbers never count. A web-only group is never matched, since its room lists only who wrote. The phone thread survives, else the mautrix room. The first run (2026-10-30) found 21 direct chats and 3 groups with a phone thread; most other direct chats are with numbers the phone archive (ending 2025-11) never held. The importer also takes a Google room's title from multichat's `name` and calls it a group only when multichat names no direct partner.

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

## Two sources, one message: merged, never replaced

One message can arrive from several importers under one key (a Discord message from a data package, a fetch and multichat's live log, all filed by its Discord id). message-import-multichat links pictures through photo-store's `GET /sightings`, so a picture sent in several messages is linked in each. Since 2026-11-01 an import that changes a stored message merges into it (`mergeMessage` in `store_import.go`): text it sends empty keeps the stored text, every stored attachment and photo link stays, and a delete or an edit once seen stays seen; text it sends replaces the stored text, which is how an edit arrives. The row as it was goes to `message_revisions` first, and `GET /messages/{id}/revisions` answers every earlier version. The store is the record: no source that knows less may take away what another said.
