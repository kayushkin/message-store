package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

var base = time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)

func row(id int, minutes int, roomID string) loggedMessage {
	return loggedMessage{Ref: "message_" + strconv.Itoa(100000+id), EventID: "$e" + strconv.Itoa(id), Source: "matrix", RoomID: roomID,
		RoomName: "Carol", Platform: "whatsapp", SenderUserID: "@whatsapp_1:example.org", SenderDisplayName: "Carol",
		MessageType: "m.text", Body: "words " + strconv.Itoa(id), ContentJSON: "{}", SentAt: base.Add(time.Duration(minutes) * time.Minute)}
}

func testRooms() map[string]room {
	small := room{RoomID: "!small:example.org", Platform: "whatsapp", MemberCount: 2}
	small.Members = append(small.Members, struct {
		UserID      string `json:"user_id"`
		DisplayName string `json:"display_name"`
	}{UserID: "@whatsapp_1:example.org", DisplayName: "Carol"})
	return map[string]room{
		"!small:example.org": small,
		"!big:example.org":   {RoomID: "!big:example.org", Platform: "whatsapp", MemberCount: 12},
		"!dm:example.org":    {RoomID: "!dm:example.org", Platform: "discord", MemberCount: 2},
		"!chan:example.org":  {RoomID: "!chan:example.org", Platform: "discord", MemberCount: 40},
	}
}

func TestSkipReasonKeepsSmallChatsAndNamedRoomsAndServers(t *testing.T) {
	chosen := selection{rooms: map[string]bool{"!named:example.org": true}, discordServers: map[string]bool{"900": true}}
	rooms := testRooms()
	cases := []struct {
		row  loggedMessage
		keep bool
	}{
		{loggedMessage{RoomID: "!small:example.org", Platform: "whatsapp"}, true},
		{loggedMessage{RoomID: "!big:example.org", Platform: "whatsapp"}, false},
		{loggedMessage{RoomID: "!dm:example.org", Platform: "discord"}, true},
		{loggedMessage{RoomID: "!chan:example.org", Platform: "discord", NetworkID: "800"}, false},
		{loggedMessage{RoomID: "!chan:example.org", Platform: "discord", NetworkID: "900"}, true},
		{loggedMessage{RoomID: "!gone:example.org", Platform: "whatsapp"}, false},
		{loggedMessage{RoomID: "!named:example.org", Platform: "discord", NetworkID: "800"}, true},
	}
	for _, c := range cases {
		if got := chosen.skipReason(c.row, rooms) == ""; got != c.keep {
			t.Errorf("%s/%s kept = %v, want %v", c.row.RoomID, c.row.NetworkID, got, c.keep)
		}
	}
}

func TestMessageConvertsArchiveAndLiveRows(t *testing.T) {
	rooms := testRooms()
	archive := loggedMessage{Ref: "message_000001", Source: "discord-archive", DiscordMessageID: "5001", RoomID: "!dm:example.org", Platform: "discord",
		ChannelID: "7001", SenderUserID: "@discord_42:example.org", SentAt: base,
		ContentJSON: `{"type":19,"content":"","edited_timestamp":"2024-03-01T12:01:00Z","attachments":[{"filename":"a.png","content_type":"image/png"}],"message_reference":{"message_id":"5000"}}`}
	live := row(2, 5, "!small:example.org")
	live.ReplyToEventID = "$e3"
	redacted := base
	live.RedactedAt = &redacted
	edit := row(4, 6, "!small:example.org")
	edit.ReplacesEventID, edit.Body = live.EventID, "better words"
	target := row(3, 1, "!small:example.org")
	media := row(5, 7, "!small:example.org")
	media.MessageType, media.Body, media.MediaMimetype, media.ContentJSON = "m.image", "a caption", "image/jpeg", `{"filename":"b.jpg"}`
	c := converter{rooms: rooms, people: map[string]string{"@discord_42:example.org": "person_000001"}, selfPerson: "person_000001",
		photos: map[string][]photoRef{"message_000001": {{ID: "photo_000009", MediaType: "image/png"}}, media.Ref: {{ID: "photo_000010"}, {ID: "photo_000011", MediaType: "image/jpeg"}}},
		events: map[string]loggedMessage{target.EventID: target, live.EventID: live}, edits: map[string][]loggedMessage{live.EventID: {edit}}}

	got, err := c.message(archive)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "discord" || got.SourceRef != "5001" || got.ConversationSource != "discord" || got.ConversationSourceRef != "7001" ||
		got.SenderHandle != "42" || got.Kind != "media" || !got.Edited || got.ReplyToSourceRef != "5000" || got.Attachments[0].PhotoID != "photo_000009" {
		t.Errorf("archive row = %+v", got)
	}
	if got, err = c.message(live); err != nil {
		t.Fatal(err)
	}
	if got.Source != "multichat" || got.SourceRef != live.Ref || got.ConversationSource != "matrix" || got.ConversationSourceRef != "!small:example.org" ||
		got.Body != "better words" || !got.Edited || !got.DeletedOnPlatform || got.ReplyToSourceRef != target.Ref || got.SourceHash == "" {
		t.Errorf("live row = %+v", got)
	}
	if got, err = c.message(media); err != nil {
		t.Fatal(err)
	}
	if got.Kind != "text" || got.Body != "a caption" || len(got.Attachments) != 2 || got.Attachments[0].Name != "b.jpg" ||
		got.Attachments[0].PhotoID != "photo_000010" || got.Attachments[1].PhotoID != "photo_000011" {
		t.Errorf("media row = %+v", got)
	}
	conversations, err := c.conversations([]loggedMessage{live, archive})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(conversations, func(i, j int) bool { return conversations[i].Source < conversations[j].Source })
	if len(conversations) != 2 || conversations[0].Platform != "discord" || conversations[0].Participants[0].Handle != "42" || !conversations[0].Participants[0].IsSelf ||
		conversations[1].Platform != "whatsapp" || conversations[1].IsGroup || len(conversations[1].Participants) != 1 {
		t.Errorf("conversations = %+v", conversations)
	}
	if _, err := c.message(loggedMessage{Ref: "message_000009", Platform: "pager", RoomID: "!x:example.org", ContentJSON: "{}"}); err == nil {
		t.Error("an unknown platform was accepted")
	}
}

// fakeLog serves GET /message-log over rows, newest first, as multichat
// does: strictly before `before`, deleted only when asked.
func fakeLog(t *testing.T, rows []loggedMessage, limit int) multichatClient {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		sorted := append([]loggedMessage(nil), rows...)
		sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].SentAt.After(sorted[j].SentAt) })
		var before time.Time
		if value := r.URL.Query().Get("before"); value != "" {
			var err error
			if before, err = time.Parse(time.RFC3339Nano, value); err != nil {
				t.Errorf("before = %q", value)
			}
		}
		page := logPage{Messages: []loggedMessage{}}
		for _, candidate := range sorted {
			if (!before.IsZero() && !candidate.SentAt.Before(before)) || (r.URL.Query().Get("deleted") == "true" && candidate.RedactedAt == nil) {
				continue
			}
			if len(page.Messages) == limit {
				break
			}
			page.Messages = append(page.Messages, candidate)
		}
		json.NewEncoder(w).Encode(page)
	}))
	t.Cleanup(server.Close)
	return multichatClient{base: server.URL, token: "secret"}
}

func refs(rows []loggedMessage) string {
	var out []string
	for _, item := range rows {
		out = append(out, strings.TrimPrefix(item.Ref, "message_1000"))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return strings.Join(out, ",")
}

func TestGatherReadsTheWindowAndWhatItPointsAt(t *testing.T) {
	room := "!small:example.org"
	old := row(1, 0, room)       // replied to from inside the window
	edited := row(2, 1, room)    // edited from inside the window
	deleted := row(3, 2, room)   // deleted: read whatever the window
	untouched := row(4, 3, room) // left alone
	reply := row(5, 100, room)
	reply.ReplyToEventID = old.EventID
	edit := row(6, 101, room)
	edit.ReplacesEventID = edited.EventID
	when := base
	deleted.RedactedAt = &when
	// Two rows at one moment straddle a page boundary.
	tieA, tieB := row(7, 102, room), row(8, 102, room)
	big := row(9, 103, "!big:example.org")
	rows := []loggedMessage{old, edited, deleted, untouched, reply, edit, tieA, tieB, big}
	pageSize = 2
	defer func() { pageSize = 1000 }()
	multichat := fakeLog(t, rows, pageSize)

	whole, err := gather(multichat, selection{}, testRooms(), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if got := refs(whole.imported); got != "08,07,05,04,03,02,01" || whole.skipped["a group of more than 8"] != 1 {
		t.Errorf("whole log imported %s, skipped %v", got, whole.skipped)
	}
	if len(whole.edits[edited.EventID]) != 1 {
		t.Errorf("edits = %v", whole.edits)
	}
	windowed, err := gather(multichat, selection{}, testRooms(), base.Add(50*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if got := refs(windowed.imported); got != "08,07,05,03,02" {
		t.Errorf("window imported %s", got)
	}
	if _, found := windowed.events[old.EventID]; !found || windowed.unresolved != 0 {
		t.Errorf("the replied-to message was not read: unresolved %d", windowed.unresolved)
	}
}

// The live rows of one Discord message (its text and a picture) become one
// message filed under the Discord id, superseding both rows' refs; a live row
// with no Discord id stays filed by its ref.
func TestLiveDiscordPartsJoinIntoOneMessage(t *testing.T) {
	text := row(1, 0, "!dm:example.org")
	text.Platform, text.ChannelID, text.SenderUserID, text.DiscordMessageID = "discord", "700", "@discord_9:example.org", "5100"
	picture := text
	picture.Ref, picture.EventID, picture.MessageType, picture.Body, picture.MediaMimetype = "message_100002", "$e2", "m.image", "cat.png", "image/png"
	picture.ContentJSON = `{"filename":"cat.png"}`
	alone := row(3, 1, "!dm:example.org")
	alone.Platform, alone.ChannelID, alone.SenderUserID = "discord", "700", "@discord_9:example.org"
	c := converter{rooms: testRooms(), people: map[string]string{}, events: map[string]loggedMessage{}, edits: map[string][]loggedMessage{}}
	messages, err := c.messages([]loggedMessage{alone, picture, text})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("%d messages, want 2: %+v", len(messages), messages)
	}
	joined := messages[1]
	if joined.Source != "discord" || joined.SourceRef != "5100" || joined.Body != "words 1" || joined.Kind != "text" ||
		len(joined.Attachments) != 1 || joined.Attachments[0].Name != "cat.png" || len(joined.Supersedes) != 2 ||
		joined.Supersedes[0].SourceRef != "message_100001" || joined.Supersedes[1].SourceRef != "message_100002" {
		t.Fatalf("joined = %+v", joined)
	}
	if messages[0].Source != "multichat" || messages[0].SourceRef != alone.Ref || len(messages[0].Supersedes) != 0 {
		t.Fatalf("a row with no Discord id = %+v", messages[0])
	}
}
