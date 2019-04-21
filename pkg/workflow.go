package diplomat

import (
	"fmt"
	"github.com/nlopes/slack"
	"github.com/rs/xid"
)

func NewID() string {
	return xid.New().String()
}

type workflowEngineConfig struct {
	diplotmatServerRef *RemoteServerRef
}

type WorkflowEngine struct {
	id                    string
	diplomatClient        *Client
	communicationChannels []CommunicationChannel
}

type EngineOpt interface {
	Apply(ApplyContext, *WorkflowEngine)
}

type SlackConfig struct {
	SlackBotToken             string
	SlackInteractionsEndpoint string
	SlackChannel              string
	SlackVerificationToken    string
}

func (cfg SlackConfig) Apply(ctx ApplyContext, engine *WorkflowEngine) {
	slackClient := slack.New(cfg.SlackBotToken)
	engine.AddCommunicationChannel(&SlackCommunicationChannel{
		DiplomatClient:       ctx.DiplomatClient,
		SlackClient:          slackClient,
		InteractionsEndpoint: cfg.SlackInteractionsEndpoint,
		SlackChannel:         cfg.SlackChannel,
		VerificationToken:    cfg.SlackVerificationToken,
	})
}

func EnableSlack(botToken, interactionsEndpoint, channel, verificationToken string) SlackConfig {
	return SlackConfig{
		SlackBotToken:             botToken,
		SlackInteractionsEndpoint: interactionsEndpoint,
		SlackChannel:              channel,
		SlackVerificationToken:    verificationToken,
	}
}

func NewEngineConfig(srvRef *RemoteServerRef) workflowEngineConfig {
	return workflowEngineConfig{
		diplotmatServerRef: srvRef,
	}
}

func NewWorkflowEngine(cfg workflowEngineConfig, op ...EngineOpt) (*WorkflowEngine, error) {
	wfID := NewID()

	srvRef := cfg.diplotmatServerRef

	cli, err := srvRef.Connect(wfID)
	if err != nil {
		return nil, err
	}

	engine := &WorkflowEngine{
		id:                    wfID,
		diplomatClient:        cli,
		communicationChannels: []CommunicationChannel{},
	}

	for _, o := range op {
		ctx := ApplyContext{DiplomatClient: cli,}
		o.Apply(ctx, engine)
	}

	return engine, nil
}

type ApplyContext struct {
	DiplomatClient *Client
}

type Notification struct {
	Text string
}

type Selection struct {
	Options []string
}

func (wf *WorkflowEngine) AddCommunicationChannel(ch CommunicationChannel) {
	wf.communicationChannels = append(wf.communicationChannels, ch)
}

func (wf *WorkflowEngine) Notify(n Notification) error {
	return wf.communicationChannels[0].Notify(n)
}

func (wf *WorkflowEngine) Select(sel Selection) (<-chan string, error) {
	return wf.communicationChannels[0].Select(sel)
}

type Workflow interface {
	Run(wf *WorkflowEngine) error
}

type MyWorkflow struct {
}

func (me *MyWorkflow) Run(wf *WorkflowEngine) error {
	if err := wf.Notify(Notification{Text: "workflow starting!"}); err != nil {
		return err
	}

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

type CommunicationChannel interface {
	Notify(n Notification) error
	Select(sel Selection) (<-chan string, error)
}

type SlackCommunicationChannel struct {
	DiplomatClient       *Client
	SlackClient          *slack.Client
	InteractionsEndpoint string
	SlackChannel         string
	VerificationToken    string
}

func (comch *SlackCommunicationChannel) Notify(n Notification) error {
	attachment := slack.Attachment{
		Text:  n.Text,
		Color: "#f9a41b",
	}
	params := slack.PostMessageParameters{
		Markdown: true,
	}

	respChannel, respTs, err := comch.SlackClient.PostMessage(comch.SlackChannel, slack.MsgOptionPostMessageParameters(params), slack.MsgOptionAttachments(attachment))
	if err != nil {
		return err
	}
	fmt.Printf("respCHannel=%s, respTs=%s", respChannel, respTs)
	return nil
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
		selected := interaction.Actions[0].SelectedOptions[0].Value
		ch <- selected
		response := interaction.OriginalMessage
		response.ReplaceOriginal = true
		response.Text = fmt.Sprintf("selected %s", selected)
		response.Attachments = []slack.Attachment{}

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
