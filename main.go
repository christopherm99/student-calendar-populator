package main

import (
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"rsc.io/pdf"
	"strconv"
	"strings"
	"golang.org/x/oauth2"
	"crypto/rand"
	"google.golang.org/api/calendar/v3"
	"time"
	"encoding/base64"
	"context"
	"html/template"
	"encoding/json"
	"golang.org/x/oauth2/google"
)

// ===== PDF Handling =====

type times struct {
	day    int
	period int
	room   string
}

type class struct {
	name      string
	semester1 []times
	semester2 []times
}

type schedule struct {
	classes []class
}

func genSchedule(pdfReader *pdf.Reader) schedule {
	if pdfReader.NumPage() != 1 {
		panic(fmt.Sprintf("This PDF has %v pages not 1.", pdfReader.NumPage()))
	}
	text := pdfReader.Page(1).Content().Text
	file, err := os.Open("courses.csv")
	if err != nil {
		panic(err)
	}
	coursesReader := csv.NewReader(file)
	coursesSlice, err := coursesReader.ReadAll()
	if err != nil {
		panic(err)
	}
	var courses map[string]class
	courses = make(map[string]class)
	for _, x := range coursesSlice {
		var newClass class
		newClass.name = x[0]
		x[2] = strings.Replace(x[2], "", "", -1)
		x[3] = strings.Replace(x[3], "", "", -1)
		f := func(c rune) bool {
			return string(c) == ":"
		}
		newClass.semester1 = make([]times, len(strings.FieldsFunc(x[2], f)))
		newClass.semester2 = make([]times, len(strings.FieldsFunc(x[2], f)))
		for i, strTime := range strings.FieldsFunc(x[2], f) {
			day, err := strconv.Atoi(string([]rune(strTime)[1:3]))
			if err != nil {
				panic(err)
			}
			period, err := strconv.Atoi(string([]rune(strTime)[4:]))
			if err != nil {
				panic(err)
			}
			newClass.semester1[i].day = day
			newClass.semester1[i].period = period
		}
		for i, strTime := range strings.FieldsFunc(x[3], f) {
			day, err := strconv.Atoi(string([]rune(strTime)[1:3]))
			if err != nil {
				panic(err)
			}
			period, err := strconv.Atoi(string([]rune(strTime)[4:]))
			if err != nil {
				panic(err)
			}
			newClass.semester2[i].day = day
			newClass.semester2[i].period = period
		}
		courses[x[1]] = newClass
	}
	prevY := -1.0
	var lines []string
	for _, x := range text {
		if x.Y != prevY {
			lines = append(lines, x.S)
		} else {
			lines[len(lines)-1] = lines[len(lines)-1] + x.S
		}
		prevY = x.Y
	}
	var unparsedClasses []string
	for _, line := range lines {
		matched, _ := regexp.MatchString("[0-9]{3}-[0-9]{2}", line)
		if matched {
			unparsedClasses = append(unparsedClasses, line)
		}
	}
	unparsedClasses = unparsedClasses[1 : len(unparsedClasses)-1]
	var classes []struct {
		code  string
		rooms []string
	}
	for _, text := range unparsedClasses {
		hyphen := strings.Index(text, "-")
		var rooms []string
		if !strings.Contains(text, "/") {
			short := strings.Replace(text[hyphen+4:len([]rune(text))], "S", "", -1)
			re := regexp.MustCompile("[0-9]|LA")
			strRooms := short[re.FindStringIndex(short)[0]:]
			for i := 0; i+1 < len(strRooms); i += 2 {
				rooms = append(rooms, string([]rune(strRooms)[i:i+2]))
			}
		}
		var newClass struct {
			code  string
			rooms []string
		}
		newClass.code = string([]rune(text)[hyphen-3 : hyphen+3])
		newClass.rooms = rooms
		classes = append(classes, newClass)
	}
	var schedule schedule
	for i, class := range classes {
		schedule.classes = append(schedule.classes, courses[class.code])
		for day, room := range classes[i].rooms {
			schedule.classes[i].semester1[day].room = room
			schedule.classes[i].semester2[day].room = room
		}
	}
	return schedule
}

// ===== Handling and Serving Website =====

func homePage(w http.ResponseWriter, r *http.Request) {
	p, err := ioutil.ReadFile("index.html")
	if err != nil {
		panic(err)
	}
	w.Write(p)
}

func uploadPage(w http.ResponseWriter, r *http.Request) {
	fmt.Println("function uploadPage called.")
	r.ParseMultipartForm(32 << 20)
	file, header, err := r.FormFile("pdf")
	if err != nil {
		panic(err)
	}
	PDFReader, err := pdf.NewReader(file, header.Size)
	sched := genSchedule(PDFReader)
	state := randToken()
	schedules[state] = sched
	t, err := template.ParseFiles("login.html")
	if err != nil {
		panic(err)
	}
	t.Execute(w, genAuthURL(state))
}

func authPage(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		panic(err)
	}
	fmt.Println(r.Form)
	tok, err := conf.Exchange(context.Background(), r.FormValue("code"))
	if err != nil {
		panic(err)
	}
	genCalendar(schedules[r.FormValue("state")], tok)
}

// ===== Google API Integration =====

var conf *oauth2.Config
var schedules map[string]schedule

type Credentials struct {
	Web struct {
		Cid     string `json:"client_id"`
		Csecret string `json:"client_secret"`
	} `json:"web"`
}

func genAuthURL(state string) string {
	fmt.Println(conf.AuthCodeURL(state, oauth2.AccessTypeOffline))
	return conf.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

type User struct {
	Sub string `json:"sub"`
	Name string `json:"name"`
	GivenName string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Profile string `json:"profile"`
	Picture string `json:"picture"`
	Email string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Gender string `json:"gender"`
}

// Generates CSRF Token
// Taken from https://skarlso.github.io/2016/06/12/google-signin-with-go/
func randToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

func genCalendar(sched schedule, tok *oauth2.Token) {
	client := conf.Client(context.Background(), tok)
	srv, err := calendar.New(client)
	newCalEntry := calendar.CalendarListEntry{
		Description: "Calendar created by CWS Calendar Populator.",
		Id: "CWS-Schedule",
	}
	srv.CalendarList.Insert(&newCalEntry)
	for _, class := range sched.classes {
		for _, classTime := range class.semester1 {
			fmtDT := findDay(time.Weekday(classTime.day)).Format(time.RFC3339)
			date := string([]rune(fmtDT)[:strings.Index(fmtDT, "T")])
			if err == nil {
				panic(err)
			}
			beginning, length := lookupTime(classTime.day, classTime.period)

			begH, begM, _ := beginning.Clock()

			endH, endM, _ := beginning.Add(length).Clock()

			event := &calendar.Event {
				Summary: class.name,
				Description: fmt.Sprintf("Meets in room %v.", classTime.room),
				Start: &calendar.EventDateTime{
					DateTime: fmt.Sprintf("%vT%v:%v-05:00", date, begH, begM),
					TimeZone: "America/New_York",
				},
				End: &calendar.EventDateTime{
					DateTime: fmt.Sprintf("%vT%v:%v-05:00", date, endH, endM),
				},
				Recurrence: []string{"RRULE:FREQ=WEEKLY;"},
			}
			_, err = srv.Events.Insert("CWS-Schedule", event).Do()
			if err == nil {
				panic(err)
			}
		}
	}
}

func chooseTime(hour, min int) time.Time {
	return time.Date(0, 0, 0, hour, min, 0, 0, time.FixedZone("UTC", 0))
}

func lookupTime(weekday, period int) (time.Time, time.Duration) {
	var length time.Duration
	var err error
	if period == 3 && (weekday == 2 || weekday == 4) {
		if weekday == 2 {
			length, err = time.ParseDuration("55m")
			if err == nil {
				panic(err)
			}
		} else {
			length, err = time.ParseDuration("50m")
			if err == nil {
				panic(err)
			}
		}
	} else {
		length, err = time.ParseDuration("40m")
		if err == nil {
			panic(err)
		}
	}
	if period == 1 {
		startTime := chooseTime(8, 30)
		if err == nil {
			panic(err)
		}
		return startTime, length
	} else if period == 2 {
		startTime := chooseTime(9, 15)
		if err == nil {
			panic(err)
		}
		return startTime, length
	} else if period == 3 {
		if weekday == 2 {
			startTime := chooseTime(10, 35)
			if err == nil {
				panic(err)
			}
			return startTime, length
		} else {
			startTime := chooseTime(10, 25)
			if err == nil {
				panic(err)
			}
			return startTime, length
		}
	} else if period == 4 {
		if weekday == 2 {
			startTime := chooseTime(11, 35)
			if err == nil {
				panic(err)
			}
			return startTime, length
		} else if weekday == 4 {
			startTime := chooseTime(11, 20)
			if err == nil {
				panic(err)
			}
			return startTime, length
		} else {
			startTime := chooseTime(11, 10)
			if err == nil {
				panic(err)
			}
			return startTime, length
		}
	} else if period == 5 {
		if weekday == 2 {
			startTime := chooseTime(12, 20)
			if err == nil {
				panic(err)
			}
			return startTime, length
		} else if weekday == 4 {
			startTime := chooseTime(1, 55)
			if err == nil {
				panic(err)
			}
			return startTime, length
		} else {
			startTime := chooseTime(11, 55)
			if err == nil {
				panic(err)
			}
			return startTime, length
		}
	} else if period == 6 {
		if weekday == 2 {
			startTime := chooseTime(1, 45)
			if err == nil {
				panic(err)
			}
			return startTime, length
		} else if weekday == 4 {
			startTime := chooseTime(2, 40)
			if err == nil {
				panic(err)
			}
			return startTime, length
		} else {
			startTime := chooseTime(1, 20)
			if err == nil {
				panic(err)
			}
			return startTime, length
		}
	} else {
		if weekday == 2 {
			startTime := chooseTime(2, 30)
			if err == nil {
				panic(err)
			}
			return startTime, length
		} else if weekday == 4 {
			startTime := chooseTime(3, 25)
			if err == nil {
				panic(err)
			}
			return startTime, length
		} else {
			startTime := chooseTime(2, 5)
			if err == nil {
				panic(err)
			}
			return startTime, length
		}
	}
}

func findDay(weekday time.Weekday) time.Time {
	testingTime := time.Now()
	for {
		if testingTime.Weekday() == weekday {
			return testingTime
		}
		testingTime = testingTime.AddDate(0, 0, 1)
	}
}

func main() {
	secret, err := ioutil.ReadFile("./client_secret.json")
	if err != nil {
		panic(err)
	}

	var c Credentials
	json.Unmarshal(secret, &c)
	conf = &oauth2.Config{
		ClientID: c.Web.Cid,
		ClientSecret: c.Web.Csecret,
		RedirectURL: "http://localhost:8080/auth",
		Scopes: []string{"https://www.googleapis.com/auth/userinfo.email", calendar.CalendarScope},
		Endpoint: google.Endpoint,
	}
	schedules = make(map[string]schedule)
	http.HandleFunc("/upload", uploadPage)
	http.HandleFunc("/", homePage)
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	http.HandleFunc("/auth", authPage)
	http.ListenAndServe(":8080", nil)
}
