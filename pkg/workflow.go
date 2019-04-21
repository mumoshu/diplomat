package diplomat

import (
	"fmt"
	"github.com/nlopes/slack"
	"github.com/rs/xid"
	"os"
)

func NewID() string {
	return xid.New().String()
}

type Workflow struct {
	DiplotmatServerRef *RemoteServerRef

	SlackBotToken string
	SlackInteractionsEndpoint string
	SlackChannel string
	SlackVerificationToken string

	id string
	diplomatClient     *Client
	slackClient        *slack.Client
	communicationChannels []CommunicationChannel
}

func StartWorkflow(opts Workflow) error {
	wfID := NewID()

	srvRef := opts.DiplotmatServerRef

	cli, err := srvRef.Connect(wfID)
	if err != nil {
		return err
	}

	if opts.SlackBotToken == "" {
		opts.SlackBotToken = os.Getenv("BOT_USER_OAUTH_ACCESS_TOKEN")
	}

	if opts.SlackInteractionsEndpoint == "" {
		return fmt.Errorf("SlackInteractionsEndpoint can not be omitted")
	}

	if opts.SlackChannel == "" {
		return fmt.Errorf("SlackChannel can not be omitted")
	}

	if opts.SlackVerificationToken == "" {
		return fmt.Errorf("SlackVerificationToken can not be omitted")
	}

	slackClient := slack.New(opts.SlackBotToken)

	comChannels := []CommunicationChannel{}
	slackComCh := &SlackCommunicationChannel{
		DiplomatClient: cli,
		SlackClient: slackClient,
		InteractionsEndpoint: opts.SlackInteractionsEndpoint,
		SlackChannel: opts.SlackChannel,
		VerificationToken: opts.SlackVerificationToken,
	}
	comChannels = append(comChannels, slackComCh)

	wf := &Workflow{
		id:                    wfID,
		diplomatClient:        cli,
		slackClient:           slackClient,
		communicationChannels: comChannels,
	}

	return wf.Start()
}

type Selection struct {
	Options []string
}

func (wf *Workflow) Start() error {
	selection, err := wf.Select(Selection{Options: []string{"foo", "bar"}})
	if err != nil {
		return err
	}

	select {
	case sel := <-selection:
		fmt.Printf("selected: %s\n", sel)
	}

	return nil
}

func (wf *Workflow) Select(sel Selection) (<-chan string, error) {
	return wf.communicationChannels[0].Select(sel)
}

type CommunicationChannel interface {
	Select(sel Selection) (<-chan string, error)
}

type SlackCommunicationChannel struct {
	DiplomatClient       *Client
	SlackClient          *slack.Client
	InteractionsEndpoint string
	SlackChannel         string
	VerificationToken    string
}

func (comch *SlackCommunicationChannel) Select(sel Selection) (<-chan string, error) {
	ch := make(chan string)
	jobID := NewID()

	// Send selection
	selectOptions := []slack.AttachmentActionOption{}
	for _, o := range sel.Options {
		actionOpt := slack.AttachmentActionOption{
			Text:  o,
			Value: o,
		}
		selectOptions = append(selectOptions, actionOpt)
	}
	attachment := slack.Attachment{
		Text:       "please select one",
		Color:      "#f9a41b",
		CallbackID: jobID,
		Actions: []slack.AttachmentAction{
			{
				Name:    actionSelect,
				Type:    "select",
				Options: selectOptions,
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

	respChannel, respTs, err := comch.SlackClient.PostMessage(comch.SlackChannel, slack.MsgOptionPostMessageParameters(params), slack.MsgOptionAttachments(attachment))
	if err != nil {
		return nil, err
	}
	fmt.Printf("respCHannel=%s, respTs=%s", respChannel, respTs)

	// Wait for selection
	condition := OnURL(comch.InteractionsEndpoint).Parameter("payload").Where("callback_id").EqString(jobID)
	handler := func(interaction slack.InteractionCallback) (*slack.Message, error) {
		selected := interaction.Actions[0].Name
		ch <- selected
		response := interaction.OriginalMessage
		response.ReplaceOriginal = true
		response.Text = fmt.Sprintf("selected %s", selected)

		if err := comch.DiplomatClient.StopServing(condition); err != nil {
			return nil, err
		}

		return &response, nil
	}
	slaHandler := slackInteractionsToHttpHandler(handler, comch.VerificationToken)
	if err := comch.DiplomatClient.ServeHTTP(comch.InteractionsEndpoint, condition, slaHandler); err != nil {
		return nil, err
	}

	return ch, nil
}
