package main

import (
	"github.com/mumoshu/diplomat/pkg"
	"github.com/nlopes/slack"
	"os"
)

func main() {
	cli := slack.New(os.Getenv("BOT_USER_OAUTH_ACCESS_TOKEN"))
	diplomat.SlackConnection{
		Client: cli,
	}.SendSelection("#playground", "mytext", "mycallback1")
}
