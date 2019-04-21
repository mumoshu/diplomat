package diplomat

import (
	"github.com/nlopes/slack"
)

const (
	// action is used for slack attament action.
	actionSelect = "select"
	actionStart  = "start"
	actionCancel = "cancel"
)

// Add bot user(perhaps necessary
// https://api.slack.com/apps/<ID>/bots?
// And you must install app to your team to get the token
//  https://api.slack.com/apps/<ID>/install-on-team?
// And finally enable interactive messages. otherwise sending interactive msgs fails
// https://api.slack.com/apps/<APP ID>/interactive-messages
type SlackConnection struct {
	Client *slack.Client
}

func (conn SlackConnection) SendSelection(channel string, text string, callbackID string) (string, string, error) {
	client := conn.Client
	// value is passed to message handler when request is approved.
	attachment := slack.Attachment{
		Text:       text,
		Color:      "#f9a41b",
		CallbackID: callbackID,
		Actions: []slack.AttachmentAction{
			{
				Name: actionSelect,
				Type: "select",
				Options: []slack.AttachmentActionOption{
					{
						Text:  "Asahi Super Dry",
						Value: "Asahi Super Dry",
					},
					{
						Text:  "Kirin Lager Beer",
						Value: "Kirin Lager Beer",
					},
					{
						Text:  "Sapporo Black Label",
						Value: "Sapporo Black Label",
					},
					{
						Text:  "Suntory Malts",
						Value: "Suntory Malts",
					},
					{
						Text:  "Yona Yona Ale",
						Value: "Yona Yona Ale",
					},
				},
			},

			{
				Name:  actionCancel,
				Text:  "Cancel",
				Type:  "button",
				Style: "danger",
			},
		},
	}

	params := slack.PostMessageParameters{
		Markdown: true,
	}

	return client.PostMessage(channel, slack.MsgOptionPostMessageParameters(params), slack.MsgOptionAttachments(attachment))
}
