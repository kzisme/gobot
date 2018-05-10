package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/asdine/storm"
	"github.com/thoj/go-ircevent"
)

var chanName = "#ChannelName"

type Quote struct {
	ID         int `storm:"id,increment=0"`
	Username   string
	QuotedText string
	SentAt     time.Time
}

func main() {

	con := irc.IRC("BotNAme", "BotName")
	err := con.Connect("irc.freenode.net:6667")
	if err != nil {
		fmt.Println("Connection Failed")
		return
	}

	db, err := storm.Open("my.db")
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	con.AddCallback("001", func(e *irc.Event) {
		con.Join(chanName)
	})

	con.AddCallback("JOIN", func(e *irc.Event) {
		con.Privmsg(chanName, "Join Message...")
	})

	con.AddCallback("PRIVMSG", func(e *irc.Event) {

		fmt.Println(e.Message())
		if strings.HasPrefix(e.Message(), ".quote") {
			addQuote(db, e.Nick, e.Message(), time.Now())
		}
	})

	con.Loop()
}

func addQuote(db *storm.DB, username string, quotedText string, sentAt time.Time) error {
	dbInsert := Quote{Username: username, QuotedText: quotedText, SentAt: sentAt}

	err := db.Save(&dbInsert)
	if err != nil {
		log.Fatal("Failed to save")
	}
	return nil
}
