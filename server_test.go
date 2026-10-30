package messagestore

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type testServer struct {
	t      *testing.T
	server *httptest.Server
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	mux := http.NewServeMux()
	RegisterHandlers(mux, store)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return &testServer{t: t, server: server}
}

func (ts *testServer) call(method, path string, body any, wantStatus int, answer any) {
	ts.t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, _ := json.Marshal(body)
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	request, _ := http.NewRequest(method, ts.server.URL+path, reader)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		ts.t.Fatal(err)
	}
	defer response.Body.Close()
	var raw json.RawMessage
	_ = json.NewDecoder(response.Body).Decode(&raw)
	if response.StatusCode != wantStatus {
		ts.t.Fatalf("%s %s = %d %s, want %d", method, path, response.StatusCode, raw, wantStatus)
	}
	if answer != nil {
		if err := json.Unmarshal(raw, answer); err != nil {
			ts.t.Fatal(err)
		}
	}
}

func at(text string) int64 {
	moment, err := time.Parse(time.RFC3339, text)
	if err != nil {
		panic(err)
	}
	return moment.UnixMilli()
}

// sampleBatch is a direct chat between the user and Carol in Lisbon, and a
// group with Carol and Dave.
func sampleBatch() ImportBatch {
	return ImportBatch{
		Conversations: []ImportConversation{
			{Platform: "skype", Source: "skype", SourceRef: "chat-carol", Participants: []Participant{
				{Handle: "me_self", DisplayName: "Me Self", IsSelf: true},
				{Handle: "carol_s", DisplayName: "Carol", PersonID: "person_000012"},
			}},
			{Platform: "discord", Source: "discord", SourceRef: "1000", Title: "Board games", IsGroup: true, Participants: []Participant{
				{Handle: "1", DisplayName: "Me Self", IsSelf: true},
				{Handle: "2", DisplayName: "Carol", PersonID: "person_000012"},
				{Handle: "3", DisplayName: "Dave"},
			}},
		},
		Messages: []ImportMessage{
			{ConversationSource: "skype", ConversationSourceRef: "chat-carol", SentAt: at("2016-03-01T23:30:00Z"), SenderHandle: "carol_s", Kind: "text", Body: "late night hello", Source: "skype", SourceRef: "r1", SourceHash: "a"},
			{ConversationSource: "skype", ConversationSourceRef: "chat-carol", SentAt: at("2016-03-02T00:30:00Z"), SenderHandle: "me_self", Kind: "text", Body: "hi Carol", Source: "skype", SourceRef: "r2", SourceHash: "a"},
			{ConversationSource: "discord", ConversationSourceRef: "1000", SentAt: at("2016-03-01T12:00:00Z"), SenderHandle: "3", Kind: "text", Body: "game night?", Source: "discord", SourceRef: "m1", SourceHash: "a"},
			{ConversationSource: "discord", ConversationSourceRef: "1000", SentAt: at("2016-03-01T12:01:00Z"), SenderHandle: "1", Kind: "media", Attachments: []Attachment{{PhotoID: "photo_000001"}}, Source: "discord", SourceRef: "m2", SourceHash: "a"},
		},
		ConversationDays: []ImportConversationDay{
			{ConversationSource: "skype", ConversationSourceRef: "chat-carol", Day: "2016-03-01", TimeZone: "Europe/Lisbon", JournalSource: "skype", JournalSourceRef: "chat-carol/2016-03-01"},
		},
	}
}

func importSample(ts *testServer) Import {
	var run Import
	ts.call("POST", "/imports", ImportStart{Importer: "test"}, http.StatusCreated, &run)
	var counts ImportCounts
	ts.call("POST", "/imports/"+run.ID+"/batches", sampleBatch(), http.StatusOK, &counts)
	if counts.ConversationsCreated != 2 || counts.MessagesCreated != 4 || counts.DaysCreated != 1 {
		ts.t.Fatalf("first batch counts = %+v", counts)
	}
	return run
}

func TestMessagesComeBackInTimeOrderAcrossConversations(t *testing.T) {
	ts := newTestServer(t)
	importSample(ts)
	var page MessagePage
	ts.call("GET", "/messages", nil, http.StatusOK, &page)
	var bodies []string
	for _, message := range page.Messages {
		bodies = append(bodies, message.Body)
	}
	want := []string{"game night?", "", "late night hello", "hi Carol"}
	if len(bodies) != len(want) {
		t.Fatalf("bodies = %q", bodies)
	}
	for i := range want {
		if bodies[i] != want[i] {
			t.Fatalf("bodies = %q, want %q", bodies, want)
		}
	}
	if page.Messages[2].SenderPersonID != "person_000012" || page.Messages[2].SenderName != "Carol" || page.Messages[3].FromSelf != true {
		t.Fatalf("sender fields not read through participants: %+v", page.Messages[2:])
	}
	if page.Messages[1].Attachments[0].PhotoID != "photo_000001" {
		t.Fatalf("attachments = %+v", page.Messages[1].Attachments)
	}
}

func TestPagingByCursorVisitsEveryMessageOnce(t *testing.T) {
	ts := newTestServer(t)
	importSample(ts)
	seen := map[string]bool{}
	cursor := ""
	for pages := 0; pages < 10; pages++ {
		var page MessagePage
		ts.call("GET", "/messages?order=newest&limit=1&cursor="+cursor, nil, http.StatusOK, &page)
		for _, message := range page.Messages {
			if seen[message.ID] {
				t.Fatalf("%s twice", message.ID)
			}
			seen[message.ID] = true
		}
		if page.Next == "" {
			break
		}
		cursor = page.Next
	}
	if len(seen) != 4 {
		t.Fatalf("saw %d messages, want 4", len(seen))
	}
}

func TestAReimportWithTheSameHashChangesNothingAndANewHashUpdates(t *testing.T) {
	ts := newTestServer(t)
	importSample(ts)
	var run Import
	ts.call("POST", "/imports", ImportStart{Importer: "test"}, http.StatusCreated, &run)
	batch := sampleBatch()
	batch.Messages[0].Body, batch.Messages[0].SourceHash = "late night hello!", "b"
	var counts ImportCounts
	ts.call("POST", "/imports/"+run.ID+"/batches", batch, http.StatusOK, &counts)
	if counts.ConversationsCreated != 0 || counts.MessagesCreated != 0 || counts.MessagesUpdated != 1 || counts.MessagesUnchanged != 3 || counts.DaysCreated != 0 {
		t.Fatalf("second batch counts = %+v", counts)
	}
	var page MessagePage
	ts.call("GET", "/messages?q=hello", nil, http.StatusOK, &page)
	if len(page.Messages) != 1 || page.Messages[0].Body != "late night hello!" {
		t.Fatalf("search after update = %+v", page.Messages)
	}
}

func TestOneBadMessageWritesNothingOfItsBatch(t *testing.T) {
	ts := newTestServer(t)
	var run Import
	ts.call("POST", "/imports", ImportStart{Importer: "test"}, http.StatusCreated, &run)
	batch := sampleBatch()
	batch.Messages[3].Kind = "telegram"
	ts.call("POST", "/imports/"+run.ID+"/batches", batch, http.StatusBadRequest, nil)
	var conversations []Conversation
	ts.call("GET", "/conversations", nil, http.StatusOK, &conversations)
	if len(conversations) != 0 {
		t.Fatalf("a refused batch left %d conversations", len(conversations))
	}
}

func TestAConversationDayIsMidnightToMidnightWhereTheUserLived(t *testing.T) {
	ts := newTestServer(t)
	importSample(ts)
	ts.call("POST", "/conversation-days/journal-entries", []JournalEntryLink{
		{JournalSource: "skype", JournalSourceRef: "chat-carol/2016-03-01", JournalEntryID: "entry_000007"},
		{JournalSource: "skype", JournalSourceRef: "chat-carol/1999-01-01", JournalEntryID: "entry_000008"},
	}, http.StatusOK, nil)
	var days []ConversationDay
	ts.call("GET", "/conversation-days?journal_entry=entry_000007", nil, http.StatusOK, &days)
	if len(days) != 1 {
		t.Fatalf("days for entry_000007 = %+v", days)
	}
	var day ConversationDayWithMessages
	ts.call("GET", "/conversation-days/"+days[0].ID, nil, http.StatusOK, &day)
	// Lisbon is UTC+0 in March, so 2016-03-02T00:30Z is the next day.
	if len(day.Messages) != 1 || day.Messages[0].Body != "late night hello" || day.MessageCount != 1 {
		t.Fatalf("day messages = %+v", day.Messages)
	}
	if day.Conversation.MessageCount != 2 || len(day.Conversation.Participants) != 2 {
		t.Fatalf("day conversation = %+v", day.Conversation)
	}
}

func TestNamingAHandleNamesEveryMessageItSent(t *testing.T) {
	ts := newTestServer(t)
	importSample(ts)
	var conversations []Conversation
	ts.call("GET", "/conversations?platform=discord", nil, http.StatusOK, &conversations)
	ts.call("PATCH", "/conversations/"+conversations[0].ID+"/participants/3", map[string]string{"person_id": "person_000044"}, http.StatusOK, nil)
	var page MessagePage
	ts.call("GET", "/messages?person=person_000044", nil, http.StatusOK, &page)
	if len(page.Messages) != 2 || page.Messages[0].SenderPersonID != "person_000044" {
		t.Fatalf("messages of person_000044 = %+v", page.Messages)
	}
}

func TestActivityCreditsDirectMessagesBothWaysAndGroupMessagesToTheirSender(t *testing.T) {
	ts := newTestServer(t)
	importSample(ts)
	var buckets []ActivityBucket
	ts.call("GET", "/activity?group_by=person&interval=month&time_zone=Europe/Lisbon", nil, http.StatusOK, &buckets)
	// Carol: one received in the direct chat, and one sent to her in it.
	// Dave has no person, and the user's group message is credited to nobody.
	if len(buckets) != 1 || buckets[0].Key != "person_000012" || buckets[0].Sent != 1 || buckets[0].Received != 1 {
		t.Fatalf("buckets = %+v", buckets)
	}
	ts.call("GET", "/activity?group_by=person&interval=fortnight&time_zone=Europe/Lisbon", nil, http.StatusBadRequest, nil)
}

func TestUnknownFieldsAreRefused(t *testing.T) {
	ts := newTestServer(t)
	ts.call("POST", "/imports", map[string]string{"importer": "test", "file": "x"}, http.StatusBadRequest, nil)
}

// A live copy filed under another namespace is retired by the message that
// names it in supersedes: its row is kept whole in the superseded record,
// its old id redirects, and sending its old key again is refused.
func TestASupersededMessageIsKeptAsARecordAndItsIDRedirects(t *testing.T) {
	ts := newTestServer(t)
	importSample(ts)
	var run Import
	ts.call("POST", "/imports", ImportStart{Importer: "test"}, http.StatusCreated, &run)
	liveCopy := ImportMessage{ConversationSource: "discord", ConversationSourceRef: "1000", SentAt: at("2016-03-01T12:00:00Z"), SenderHandle: "3",
		Kind: "text", Body: "game night?", Source: "multichat", SourceRef: "message_000042", SourceHash: "a"}
	var counts ImportCounts
	ts.call("POST", "/imports/"+run.ID+"/batches", ImportBatch{Messages: []ImportMessage{liveCopy}}, http.StatusOK, &counts)
	var page MessagePage
	ts.call("GET", "/messages?q=game", nil, http.StatusOK, &page)
	if len(page.Messages) != 2 {
		t.Fatalf("before: %d copies, want 2", len(page.Messages))
	}
	oldID := ""
	for _, message := range page.Messages {
		if message.Source == "multichat" {
			oldID = message.ID
		}
	}

	merged := sampleBatch().Messages[2]
	merged.SourceHash = "b"
	merged.Supersedes = []MessageKey{{Source: "multichat", SourceRef: "message_000042"}, {Source: "multichat", SourceRef: "message_never_stored"}}
	ts.call("POST", "/imports/"+run.ID+"/batches", ImportBatch{Messages: []ImportMessage{merged}}, http.StatusOK, &counts)
	if counts.MessagesSuperseded != 1 || counts.MessagesUpdated != 1 {
		t.Fatalf("counts = %+v", counts)
	}
	ts.call("GET", "/messages?q=game", nil, http.StatusOK, &page)
	if len(page.Messages) != 1 || page.Messages[0].Source != "discord" {
		t.Fatalf("after: %+v", page.Messages)
	}
	newID := page.Messages[0].ID

	var retired []SupersededMessage
	ts.call("GET", "/superseded-messages?superseded_by="+newID, nil, http.StatusOK, &retired)
	var row map[string]any
	if len(retired) != 1 || retired[0].ID != oldID || json.Unmarshal(retired[0].Row, &row) != nil || row["body"] != "game night?" || row["source_ref"] != "message_000042" {
		t.Fatalf("superseded record = %+v", retired)
	}

	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := noFollow.Get(ts.server.URL + "/messages/" + oldID)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusMovedPermanently || response.Header.Get("Location") != newID {
		t.Fatalf("GET old id = %d Location %q", response.StatusCode, response.Header.Get("Location"))
	}
	var followed Message
	ts.call("GET", "/messages/"+oldID, nil, http.StatusOK, &followed)
	if followed.ID != newID {
		t.Fatalf("followed to %s, want %s", followed.ID, newID)
	}
	ts.call("POST", "/imports/"+run.ID+"/batches", ImportBatch{Messages: []ImportMessage{liveCopy}}, http.StatusBadRequest, nil)
}
