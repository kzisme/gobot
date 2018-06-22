package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/asdine/storm"
	"github.com/thoj/go-ircevent"
)

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

type LoggedMessage struct {
	ID       int `storm:"id,increment=0"`
	Channel  string
	Username string
	Message  string
	SentAt   string
}

var supportedCommands = []string{
	".quote",
	".addquote",
	".weather",
	".addweather",
	".seen",
	".test",
}

func main() {
	db, err := storm.Open("my.db")
	if err != nil {
		log.Fatal("DB Error: " + err.Error())
	}

	defer db.Close()
	go RunWebServer(db)

	con := irc.IRC("BotName", "BotName")
	err = con.Connect("irc.freenode.net:6667")
	if err != nil {
		fmt.Println("Connection Failed")
		return
	}

	con.AddCallback("001", func(e *irc.Event) {
		con.Join(e.Arguments[0])
	})

	con.AddCallback("JOIN", func(e *irc.Event) {
		con.Privmsg(e.Arguments[0], "Join Message...")
	})

	con.AddCallback("INVITE", func(e *irc.Event) {
		con.Join(e.Arguments[1])
	})

	// Add general logging (messages without commands)
	con.AddCallback("PRIVMSG", func(e *irc.Event) {
		if containsCommand(supportedCommands, strings.Fields(e.Message())[0]) {
			switch strings.Fields(e.Message())[0] {
			case ".quote":
				findSingleQuote(e.Arguments[0], db, con)
			case ".addquote":
				addQuote(db, e.Nick, e.Message(), time.Now())
			case ".weather":
				fetchWeatherForLocation(e.Arguments[0], db, e.Nick, e.Message(), con)
			case ".addweather":
				addWeatherLocation(e.Arguments[0], db, e.Nick, e.Message(), con)
			case ".seen":
				findUserLastSeen(e.Message(), e.Arguments[0], db, con)
			case ".test":
				fmt.Printf("%s", e.Tags)
			}
		} else {
			logMessage(db, e.Nick, e.Arguments[0], e.Message(), time.Now())
		}
	})

	con.Loop()
}

func RunWebServer(db *storm.DB) {
	tmpl := template.Must(template.ParseFiles("templates/layout.html"))

	//TODO: Handle errors from this
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var loggedMessages []LoggedMessage

		err := db.All(&loggedMessages)
		if err != nil {
			log.Fatal("Handle Func DB Error: " + err.Error())
		}

		fmt.Println(len(loggedMessages))

		tmpl.Execute(w, loggedMessages)
	})

	http.ListenAndServe(":8081", nil)
}

func findUserLastSeen(userToFind string, channel string, db *storm.DB, con *irc.Connection) {
	var userLastSeen []LoggedMessage

	if len(strings.Fields(userToFind)) == 2 {
		err := db.Find("Username", strings.Join(strings.Fields(userToFind)[1:2], " "), &userLastSeen, storm.Reverse(), storm.Limit(1))
		if err != nil {

			fmt.Println(err)
			con.Privmsg(channel, "User Not Found...")

		} else {
			t, err := time.Parse("01-02-2006", userLastSeen[0].SentAt)

			if err != nil {
				fmt.Println(err)
			}

			con.Privmsg(channel, "User: "+userLastSeen[0].Username+" "+"was seen on"+" "+t.Format("01-02-2006")+" "+"Message:"+" "+userLastSeen[0].Message)
		}
	} else {
		con.Privmsg(channel, "Unknown syntax - Please use the following syntax: '.seen username'")
	}
}

func logMessage(db *storm.DB, username string, chanName string, message string, sentAt time.Time) {
	logInsert := LoggedMessage{Channel: chanName, Username: username, Message: message, SentAt: sentAt.Format("01-02-2006")}

	err := db.Save(&logInsert)
	if err != nil {
		log.Fatal("Failed to save")
	}
}

func addQuote(db *storm.DB, username string, quotedText string, sentAt time.Time) error {
	dbInsert := Quote{Username: username, QuotedText: quotedText, SentAt: sentAt}

	err := db.Save(&dbInsert)
	if err != nil {
		log.Fatal("Failed to save")
	}

	return nil
}

func findSingleQuote(channel string, db *storm.DB, con *irc.Connection) {
	var quoteQuery Quote

	quoteCount, err := db.Count(&quoteQuery)
	if err == nil {
		var randomID = rand.Intn(quoteCount)

		err := db.One("ID", randomID, &quoteQuery)
		if err != nil {
			log.Fatal("Query Error Occured")
		} else {
			fmt.Println(err)

			con.Privmsg(channel, "Quote added by: "+quoteQuery.Username+" : "+"On "+quoteQuery.SentAt.Format("01-02-2006")+" ~ "+strings.Join(strings.Fields(quoteQuery.QuotedText)[1:], " "))
		}
	}
}

func fetchWeatherForLocation(channel string, db *storm.DB, username string, message string, con *irc.Connection) {
	var weatherQuery Weather
	err := db.One("Username", username, &weatherQuery)

	if err != nil || weatherQuery.Username == " " {
		con.Privmsg(channel, username+" It doesn't look like you have configured a location - please add a location with command .weather ~San Francisco~")
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

			con.Privmsg(channel, weatherQuery.Username+" "+"-"+" "+" The current weather condition is"+" "+getCurrentWeatherCondition(responseString)+" "+"and"+" "+getCurrentTemp(responseString)+" "+"in"+" "+weatherQuery.City)
		}
	}
}

func addWeatherLocation(channel string, db *storm.DB, username string, message string, con *irc.Connection) {
	if strings.Contains(message, "~") {
		locationString := strings.Split(message, "~")
		weatherConfig := Weather{Username: username, City: locationString[1]}

		fmt.Println(weatherConfig)

		//Fix this for updates if user already exists
		err := db.Save(&weatherConfig)
		if err != nil {
			log.Fatal("Failed to save")
		}
		con.Privmsg(channel, "You have successfully configured your location!")
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
		"Haze",
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
