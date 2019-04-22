package basicworkflow

import (
	"fmt"
	"log"
	"os"

	d "github.com/mumoshu/diplomat/pkg"
	"github.com/codeskyblue/go-sh"
)

func main() {
	extHost := os.Getenv("EXT_HOST")
	githubWebhookEndpoint := fmt.Sprintf("http://%s/webhook/github", extHost)
	githubWebhookSecret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	githubAccessToken := os.Getenv("GITHUB_TOKEN")
	githubOwner := "mumoshu"
	githubRepo := "diplomat-test"
	githubIssueNumber := 1

	slackBotToken := os.Getenv("BOT_USER_OAUTH_ACCESS_TOKEN")
	slackVerificationToken := os.Getenv("VERIFICATION_TOKEN")
	slackIntUrl := fmt.Sprintf("http://%s/webhook/slack-interactive", extHost)

	realm := "channel1"
	netAddr := "0.0.0.0"
	wsPort := 8000
	srvRef := d.NewWsServerRef(realm, netAddr, wsPort)

	engineCfg := d.NewEngineConfig(srvRef)
	enableSlack := d.EnableSlack(
		slackBotToken,
		slackIntUrl,
		"#playground",
		slackVerificationToken,
	)
	enableGHIssue := d.EnableGitHubIssue(
		githubWebhookEndpoint,
		githubWebhookSecret,
		githubAccessToken,
		githubOwner,
		githubRepo,
		githubIssueNumber,
	)
	wf := &myBespokeWorkflow{}
	engine, err := d.NewWorkflowEngine(engineCfg, enableSlack, enableGHIssue)
	if err != nil {
		log.Fatal("workflow engine failed: %v", err)
	}
	if err := wf.Run(engine); err != nil {
		log.Fatal("workflow run failed: %v", err)
	}
}

type myBespokeWorkflow struct{
	triggerURL string
	triggerCondition string
}

func (wf *myBespokeWorkflow) Run(engine *d.WorkflowEngine) error {
	engine.Subscribe(wf.triggerURL, "body.json containers foo.bar=1", func(event d.Event) error {
		// This send the message "workflow starting!" to the specified Slack channel and GitHub Issue
		if err := engine.Notify(d.Notification{Text: "workflow starting!"}); err != nil {
			return err
		}

		// This send the message to ask selecting either "foo" or "bar" in the specified Slack channel and GitHub Issue
		selectedOption, err := engine.Select(d.Selection{Options: []string{"foo", "bar"}})
		if err != nil {
			return err
		}

		var fooOrBar string
		select {
		case fooOrBar = <-selectedOption:
			fmt.Printf("selected: %s\n", fooOrBar)
		}

		// Run local commands with codeskyblue/go-sh
		if err := sh.Command("echo", "hello\tworld").Command("cut", "-f2").Run(); err != nil {
			log.Fatal(err)
		}

		// Create a new Kubernetes pod and run a command in it
		if err := sh.Command("kubectl", "run", "--restart=Never", "--image", "alpine:3.9", "myalpinerunner", "--", "sleep", "1000").Run(); err != nil {
			log.Fatal(err)
		}

		// Run a command in the already running pod w/ kubectl-iexec(https://github.com/gabeduke/kubectl-iexec)
		if err := sh.Command("kubectl", "iexec", "myalpinerunner", "cat", "/etc/hosts"); err != nil {
			log.Fatal(err)
		}
	})

	return nil
}
