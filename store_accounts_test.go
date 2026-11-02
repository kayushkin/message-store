package messagestore

import "testing"

func TestAnAccountSeesTheConversationsItsHandleTakesPartIn(t *testing.T) {
	ts := newTestServer(t)
	var bot Account
	ts.call("POST", "/accounts", AccountPut{Platform: "discord", Handle: "900", DisplayName: "Helper", Kind: "bot"}, 200, &bot)
	var again Account
	ts.call("POST", "/accounts", AccountPut{Platform: "discord", Handle: "900", Kind: "bot"}, 200, &again)
	if again.ID != bot.ID || again.DisplayName != "Helper" {
		t.Fatalf("putting the account again gave %+v, want %s keeping its name", again, bot.ID)
	}
	ts.call("POST", "/accounts", AccountPut{Platform: "discord", Handle: "901", Kind: "robot"}, 400, nil)

	var run Import
	ts.call("POST", "/imports", ImportStart{Importer: "test"}, 201, &run)
	batch := ImportBatch{
		Conversations: []ImportConversation{
			{Platform: "discord", Source: "discord", SourceRef: "c1", Title: "general", IsGroup: true,
				Participants: []Participant{{Handle: "900", DisplayName: "Helper"}}},
			{Platform: "discord", Source: "discord", SourceRef: "c2", Title: "elsewhere", IsGroup: true},
			{Platform: "skype", Source: "skype", SourceRef: "900", Title: "same handle, other platform"},
		},
		Messages: []ImportMessage{{ConversationSource: "discord", ConversationSourceRef: "c1", SentAt: at("2026-01-02T10:00:00Z"),
			SenderHandle: "900", Kind: "text", Source: "discord", SourceRef: "m1",
			Embeds:  []Embed{{Title: "Picnic", Fields: []EmbedField{{Name: "Going", Value: "3 of 10"}}}},
			Buttons: []Button{{Label: "Join"}}}},
	}
	ts.call("POST", "/imports/"+run.ID+"/batches", batch, 200, nil)

	var conversations []Conversation
	ts.call("GET", "/conversations?account="+bot.ID, nil, 200, &conversations)
	if len(conversations) != 1 || conversations[0].SourceRef != "c1" {
		t.Fatalf("account conversations = %+v, want only c1", conversations)
	}
	ts.call("GET", "/conversations?account=message_account_999999", nil, 404, nil)

	var page MessagePage
	ts.call("GET", "/messages?conversation="+conversations[0].ID, nil, 200, &page)
	if len(page.Messages) != 1 || len(page.Messages[0].Embeds) != 1 || page.Messages[0].Embeds[0].Fields[0].Value != "3 of 10" || page.Messages[0].Buttons[0].Label != "Join" {
		t.Fatalf("message = %+v, want its embed and button", page.Messages)
	}

	// A source that sends no card keeps the stored one.
	batch.Messages[0].Embeds, batch.Messages[0].Buttons, batch.Messages[0].Body = nil, nil, "text only"
	batch.Messages[0].SourceHash = "second"
	ts.call("POST", "/imports/"+run.ID+"/batches", ImportBatch{Messages: batch.Messages}, 200, nil)
	ts.call("GET", "/messages?conversation="+conversations[0].ID, nil, 200, &page)
	if len(page.Messages[0].Embeds) != 1 || len(page.Messages[0].Buttons) != 1 || page.Messages[0].Body != "text only" {
		t.Fatalf("after a card-less import the message is %+v, want the card kept", page.Messages[0])
	}

	var accounts []Account
	ts.call("GET", "/accounts", nil, 200, &accounts)
	if len(accounts) != 1 || accounts[0].ID != bot.ID {
		t.Fatalf("accounts = %+v", accounts)
	}
}

func TestAnImportThatDoesNotKnowTheUserLeavesThemMarked(t *testing.T) {
	ts := newTestServer(t)
	var run Import
	ts.call("POST", "/imports", ImportStart{Importer: "test"}, 201, &run)
	ts.call("POST", "/imports/"+run.ID+"/batches", ImportBatch{Conversations: []ImportConversation{{Platform: "discord", Source: "discord", SourceRef: "c1", IsGroup: true,
		Participants: []Participant{{Handle: "100", DisplayName: "Me Self", IsSelf: true}}}}}, 200, nil)
	ts.call("POST", "/imports/"+run.ID+"/batches", ImportBatch{Conversations: []ImportConversation{{Platform: "discord", Source: "discord", SourceRef: "c1", IsGroup: true,
		Participants: []Participant{{Handle: "100", DisplayName: "Me Self"}}}}}, 200, nil)
	var conversations []Conversation
	ts.call("GET", "/conversations", nil, 200, &conversations)
	if !conversations[0].Participants[0].IsSelf {
		t.Fatalf("participants = %+v, want the user still marked", conversations[0].Participants)
	}
}
