package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
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

type Weather struct {
	ID       int    `storm:"id,increment=0"`
	Username string `storm:"unique"`
	City     string
}

var supportedCommands = []string{
	".quote",
	".addquote",
	".weather",
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
			case ".weather":
				fetchWeatherForLocation(db, e.Nick, e.Message(), con)
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

func fetchWeatherForLocation(db *storm.DB, username string, message string, con *irc.Connection) {

	var weatherQuery Weather
	err := db.One("Username", username, &weatherQuery)

	if err != nil || weatherQuery.Username == " " || !strings.Contains(message, "~") {
		con.Privmsg(chanName, username+" It doesn't look like you have configured a location - please add a location with command .weather ~San Francisco~")
	} else if strings.Contains(message, "~") {
		// If the user is configuring a location
		locationString := strings.NewReplacer("~", "", "~", "")
		weatherConfig := Weather{Username: username, City: locationString.Replace(message)}

		err := db.Save(&weatherConfig)
		if err != nil {
			log.Fatal("Failed to save")
		}
	} else {
		// Username exists...grab city and pipe into query and return
		resp, err := http.Get("wttr.in/~" + weatherQuery.City)
		if err != nil {
			log.Fatal("HTTP Error Occured...")
		}

		defer resp.Body.Close()

		// Possibly deal with Readall err
		if resp.StatusCode == http.StatusOK {
			bodyBytes, _ := ioutil.ReadAll(resp.Body)
			bodyString := string(bodyBytes)

			con.Privmsg(chanName, bodyString)
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
