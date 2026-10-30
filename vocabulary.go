package messagestore

// Platforms are the apps a conversation happened on.
var Platforms = []string{"sms", "skype", "facebook-messenger", "discord", "whatsapp", "telegram", "signal", "slack", "instagram", "google-chat", "matrix"}

// MessageKinds are what a message row can be.
var MessageKinds = []string{
	"text",    // words, with or without attachments
	"media",   // only attachments
	"call",    // a voice or video call, call_seconds long (0: missed or unanswered)
	"sticker", // a sticker, body its name
	"system",  // a line the platform wrote: someone joined, the title changed
}

// ActivityGroupings are the keys GET /activity can count by.
var ActivityGroupings = []string{"person", "conversation", "platform"}

// ActivityIntervals are the bucket sizes GET /activity can count in.
var ActivityIntervals = []string{"day", "week", "month", "year"}

// Vocabulary is GET /vocabulary.
type Vocabulary struct {
	Platforms         []string `json:"platforms"`
	MessageKinds      []string `json:"message_kinds"`
	ActivityGroupings []string `json:"activity_groupings"`
	ActivityIntervals []string `json:"activity_intervals"`
}

func vocabulary() Vocabulary {
	return Vocabulary{Platforms: Platforms, MessageKinds: MessageKinds, ActivityGroupings: ActivityGroupings, ActivityIntervals: ActivityIntervals}
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
