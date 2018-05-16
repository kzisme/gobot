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

var chanName = "#redlight"

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
	".addweather",
}

func main() {

	con := irc.IRC("BotName", "BotName")
	err := con.Connect("irc.crushandrun.net:6667")
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
			case ".addweather":
				addWeatherLocation(db, e.Nick, e.Message(), con)
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

	if err != nil || weatherQuery.Username == " " {
		con.Privmsg(chanName, username+" It doesn't look like you have configured a location - please add a location with command .weather ~San Francisco~")
	} else {
		// Username exists...grab city and pipe into query and return

		resp, err := http.Get("http://wttr.in/~" + weatherQuery.City + "?0TQ")
		if err != nil {
			fmt.Println(weatherQuery.City)

			log.Fatal(err.Error())
		}

		defer resp.Body.Close()

		// Possibly deal with Readall err
		if resp.StatusCode == http.StatusOK {
			bodyBytes, _ := ioutil.ReadAll(resp.Body)
			bodyString := string(bodyBytes)

			var responseString = getStringBetweenTags(bodyString, "<pre>", "</pre>")

			con.Privmsg(chanName, weatherQuery.Username+" "+"-"+" "+" It Is currently"+" "+getCurrentWeatherCondition(responseString)+" "+"and"+" "+getCurrentTemp(responseString)+" "+"in"+" "+weatherQuery.City)
		}
	}
}

func addWeatherLocation(db *storm.DB, username string, message string, con *irc.Connection) {
	if strings.Contains(message, "~") {
		locationString := strings.Split(message, "~")
		weatherConfig := Weather{Username: username, City: locationString[1]}

		fmt.Println(weatherConfig)

		//Fix this for updates if user already exists
		err := db.Save(&weatherConfig)
		if err != nil {
			log.Fatal("Failed to save")
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

func getStringBetweenTags(str string, startTag string, endTag string) (result string) {
	s := strings.Index(str, startTag)
	if s == -1 {
		return
	}
	s += len(startTag)
	e := strings.Index(str, endTag)
	return str[s:e]
}

func getCurrentTemp(in string) string {

	degreeRuneIndex := strings.IndexRune(in, '°')
	var spacesFound int
	var tempValue string
	for i := degreeRuneIndex; i >= 0; i-- {
		if in[i] == ' ' {
			spacesFound++
		}
		if spacesFound == 2 {
			// Offset where we found the space by 1 and where we have our degree rune index by 1 to trim the spaces we know about
			tempValue = in[i+1 : degreeRuneIndex-1]
		}
	}

	spaceRuneIndex := strings.IndexRune(in[degreeRuneIndex:], ' ')
	tempUnits := in[degreeRuneIndex:(degreeRuneIndex + spaceRuneIndex)]

	return tempValue + " " + tempUnits
}

func getCurrentWeatherCondition(str string) (condition string) {
	// Maybe find unicode/emoji chars and build this []string into a struct of some sort?
	conditions := []string{"Clear",
		"Sunny",
		"Partly cloudy",
		"Cloudy",
		"Overcast",
		"Mist",
		"Patchy rain possible",
		"Patchy snow possible",
		"Patchy sleet possible",
		"Patchy freezing drizzle possible",
		"Thundery outbreaks possible",
		"Blowing snow",
		"Blizzard",
		"Fog",
		"Freezing fog",
		"Patchy light drizzle",
		"Light drizzle",
		"Freezing drizzle",
		"Heavy freezing drizzle",
		"Patchy light rain",
		"Light rain",
		"Moderate rain at times",
		"Moderate rain",
		"Heavy rain at times",
		"Heavy rain",
		"Light freezing rain",
		"Moderate or heavy freezing rain",
		"Light sleet",
		"Moderate or heavy sleet",
		"Patchy light snow",
		"Light snow",
		"Patchy moderate snow",
		"Moderate snow",
		"Patchy heavy snow",
		"Heavy snow",
		"Ice pellets",
		"Light rain shower",
		"Moderate or heavy rain shower",
		"Torrential rain shower",
		"Light sleet showers",
		"Light snow showers",
		"Moderate or heavy sleet showers",
		"Moderate or heavy snow showers",
		"Patchy light rain with thunder",
		"Moderate or heavy rain with thunder",
		"Patchy light snow with thunder",
		"Moderate or heavy snow with thunder"}

	var returnedStr = ""
	for _, substr := range conditions {
		if strings.Contains(str, substr) {
			returnedStr += substr
		}
	}
	return returnedStr
}
