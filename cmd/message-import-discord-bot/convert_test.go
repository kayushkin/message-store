package main

import (
	"encoding/json"
	"testing"
)

func TestABotCardKeepsItsEmbedAndButtonsAndAReplyNamesItsTarget(t *testing.T) {
	var message discordMessage
	raw := `{"id":"20","channel_id":"5","type":19,"author":{"id":"900","username":"helper","bot":true},"content":"",
		"timestamp":"2026-01-02T10:00:00.000000+00:00","edited_timestamp":"2026-01-02T11:00:00.000000+00:00",
		"embeds":[{"title":"Picnic","fields":[{"name":"Going","value":"3 of 10","inline":true}],"footer":{"text":"Saturday"}}],
		"components":[{"type":1,"components":[{"type":2,"label":"Join","emoji":{"name":"✅"}},{"type":2,"label":"Map","url":"https://example.com"}]}],
		"message_reference":{"message_id":"19"}}`
	if err := json.Unmarshal([]byte(raw), &message); err != nil {
		t.Fatal(err)
	}
	converted := convertMessage(message)
	if converted.Kind != "text" || !converted.Edited || converted.ReplyToSourceRef != "19" || converted.SenderHandle != "900" {
		t.Fatalf("converted = %+v", converted)
	}
	if len(converted.Embeds) != 1 || converted.Embeds[0].Fields[0].Value != "3 of 10" || converted.Embeds[0].FooterText != "Saturday" {
		t.Fatalf("embeds = %+v", converted.Embeds)
	}
	if len(converted.Buttons) != 2 || converted.Buttons[0].Emoji != "✅" || converted.Buttons[1].URL != "https://example.com" {
		t.Fatalf("buttons = %+v", converted.Buttons)
	}
	if converted.ConversationSourceRef != "5" || converted.SourceRef != "20" || converted.SourceHash == "" {
		t.Fatalf("keys = %+v", converted)
	}
}

func TestALineDiscordWroteIsASystemLine(t *testing.T) {
	converted := convertMessage(discordMessage{ID: "1", ChannelID: "5", Type: 18, Author: discordUser{ID: "7"}})
	if converted.Kind != "system" {
		t.Fatalf("kind = %s, want system", converted.Kind)
	}
}

func TestSnowflakesCountFromDiscordsEpoch(t *testing.T) {
	// 1420070400000 + 1000 ms is 1000<<22.
	if got := snowflakeAt(1420070401000); got != "4194304000" {
		t.Fatalf("snowflakeAt = %s", got)
	}
	if !snowflakeLess("999", "1000") || snowflakeLess("1000", "999") {
		t.Fatal("snowflakes compare as numbers")
	}
}
