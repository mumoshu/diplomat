package diplomat

import (
	"context"
	"fmt"
	"github.com/nlopes/slack"
	"github.com/rs/xid"
	"gopkg.in/go-playground/webhooks.v5/github"
	gogithub "github.com/google/go-github/v25/github"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"strings"
	"sync"
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

type GitHubIssueConfig struct {
	WebhookEndpoint   string
	WebhookSecret     string
	IssueNumber       int
	GitHubAccessToken string
	GitHubOwner       string
	GitHubRepo        string
}

func (cfg GitHubIssueConfig) Apply(ctx ApplyContext, engine *WorkflowEngine) {
	ctx2 := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.GitHubAccessToken},
	)
	tc := oauth2.NewClient(ctx2, ts)
	client := gogithub.NewClient(tc)

	hook, err := github.New(github.Options.Secret(cfg.WebhookSecret))
	if err != nil {
		panic(err)
	}

	engine.AddCommunicationChannel(&GitHubCommunicationChannel{
		DiplomatClient:       ctx.DiplomatClient,
		githubClient:          client,
		hook: hook,
		WebhookEndpoint: cfg.WebhookEndpoint,
		GitHubOwner: cfg.GitHubOwner,
		GitHubRepo: cfg.GitHubRepo,
		IssueNumber: cfg.IssueNumber,
	})
}

func EnableGitHubIssue(webhookEndpoint, webhookSecret, accessToken, owner, repo string, issueNumber int) GitHubIssueConfig {
	return GitHubIssueConfig{
		WebhookEndpoint: webhookEndpoint,
		WebhookSecret: webhookSecret,
		GitHubAccessToken: accessToken,
		GitHubOwner: owner,
		GitHubRepo: repo,
		IssueNumber: issueNumber,
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
	for _, ch := range wf.communicationChannels {
		if err := ch.Notify(n); err != nil {
			return err
		}
	}
	return nil
}

func mergeChannels(cs []<-chan string) <-chan string {
	out := make(chan string)
	var wg sync.WaitGroup
	wg.Add(len(cs))
	for _, c := range cs {
		go func(c <-chan string) {
			for v := range c {
				out <- v
			}
			wg.Done()
		}(c)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func (wf *WorkflowEngine) Select(sel Selection) (<-chan string, error) {
	results := []<-chan string{}
	for _, ch := range wf.communicationChannels {
		res, err := ch.Select(sel)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return mergeChannels(results), nil
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

type GitHubCommunicationChannel struct {
	DiplomatClient *Client
	githubClient   *gogithub.Client
	hook *github.Webhook

	WebhookEndpoint   string
	IssueNumber       int
	GitHubOwner       string
	GitHubRepo        string
}

func (ch *GitHubCommunicationChannel) Notify(n Notification) error {
	gh := ch.githubClient
	ctx := context.Background()
	comment := &gogithub.IssueComment{
		Body: &n.Text,
	}
	created, response, err := gh.Issues.CreateComment(ctx, ch.GitHubOwner, ch.GitHubRepo, ch.IssueNumber, comment)
	if err != nil {
		return err
	}
	fmt.Printf("created %v, response %v", created, response)
	return nil
}

func (ch *GitHubCommunicationChannel) Select(sel Selection) (<-chan string, error) {
	notification := Notification{
		Text: fmt.Sprintf("Comment `/select [%s]` to select one of %s", strings.Join(sel.Options, "|"), strings.Join(sel.Options, ", ")),
	}
	if err := ch.Notify(notification); err != nil {
		return nil, err
	}
	resch := make(chan string)

	hook := ch.hook
	condition := OnURL(ch.WebhookEndpoint).Where("issue", "number").EqInt(ch.IssueNumber)
	handler := func(w http.ResponseWriter, r *http.Request) {
		payload, err := hook.Parse(r, github.IssueCommentEvent)
		if err != nil {
			if err == github.ErrEventNotFound {
				// ok event wasn;t one of the ones asked to be parsed
			}
		}
		switch typed := payload.(type) {
		case github.IssueCommentPayload:
			fmt.Printf("comment %s: %s\n", typed.Action, typed.Comment.Body)
			if strings.Index("/select ", typed.Comment.Body) > -1 {

			}
			splits := strings.Split(typed.Comment.Body, "/select ")
			if splits[0] != "" {
				fmt.Printf("unexpected comment body: %s", typed.Comment.Body)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			selected := splits[1]
			for _, s := range sel.Options {
				if s == selected {
					resch <- s
					if err := ch.DiplomatClient.StopServing(condition); err != nil {
						log.Fatal(fmt.Errorf("stop serving failed: %v", err))
					}
					return
				}
			}
			fmt.Printf("unexpected selection: %s", selected)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	if err := ch.DiplomatClient.ServeHTTP(ch.WebhookEndpoint, condition, FuncHttpHandler{handler}); err != nil {
		return nil, err
	}
	return resch, nil
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
