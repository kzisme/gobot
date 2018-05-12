package main

import (
	"fmt"
	"log"
	"math/rand"
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

var supportedCommands = []string{
	".quote",
	".addquote",
}

func main() {

	con := irc.IRC("BotName", "BotName")
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

	// Add general logging (messages without commands)
	con.AddCallback("PRIVMSG", func(e *irc.Event) {
		if containsCommand(supportedCommands, strings.Fields(e.Message())[0]) {
			switch strings.Fields(e.Message())[0] {
			case ".quote":
				findSingleQuote(db, con)
			case ".addquote":
				addQuote(db, e.Nick, e.Message(), time.Now())

			}
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

func findSingleQuote(db *storm.DB, con *irc.Connection) {
	var quoteQuery Quote

	quoteCount, err := db.Count(&quoteQuery)
	if err == nil {
		var randomID = rand.Intn(quoteCount)

		err := db.One("ID", randomID, &quoteQuery)
		if err != nil {
			log.Fatal("Query Error Occured")
		} else {
			fmt.Println(err)

			con.Privmsg(chanName, "Quote added by: "+quoteQuery.Username+" : "+"On "+quoteQuery.SentAt.Format("01-02-2006")+" ~ "+strings.Join(strings.Fields(quoteQuery.QuotedText)[1:], " "))
		}
	}
}

func containsCommand(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
