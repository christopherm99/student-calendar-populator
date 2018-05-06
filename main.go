package main

import (
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"crypto/rand"
	"time"
	"encoding/base64"
	"context"
	"encoding/json"

	"google.golang.org/api/calendar/v3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"rsc.io/pdf"
	"html/template"
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
	text := pdfReader.Page(1).Content().Text
	fmt.Println(text)
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
	fmt.Println(lines)
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
			fmt.Printf("SHORT: %v\n", short)
			re := regexp.MustCompile("[0-9]|LA|4-ART|Gym")
			strRooms := short[re.FindStringIndex(short)[0]:]
			fmt.Println("StrRooms: ", strRooms)
			if strings.Contains(strRooms, "4-ART") {
				for i := 0; i+1 < len(strRooms); i += 5 {
					rooms = append(rooms, "4-ART")
				}
			} else if strings.Contains(strRooms, "Gym") {
				for i := 0; i+1 < len(strRooms); i += 3 {
					rooms = append(rooms, "Gym")
				}
			} else {
				for i := 0; i+1 < len(strRooms); i += 2 {
					rooms = append(rooms, string([]rune(strRooms)[i:i+2]))
				}
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
			fmt.Printf("Class number: %v\nDay: %v\nRoom: %v\n", i, day, room)
			schedule.classes[i].semester1[day].room = room
			schedule.classes[i].semester2[day].room = room
		}
	}
	return schedule
}

// ===== Handling and Serving Website =====

func homePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

func verifyPage(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	file, header, err := r.FormFile("pdf")
	if err != nil {
		t, _ := template.ParseFiles("error.html")
		t.Execute(w, err)
		return
	}
	PDFReader, err := pdf.NewReader(file, header.Size)
	if err != nil {
		t, _ := template.ParseFiles("error.html")
		t.Execute(w, err)
		return
	}
	if PDFReader == nil {
		t, _ := template.ParseFiles("error.html")
		t.Execute(w, "invalid memory address or nil pointer dereference")
		return
	}
	if PDFReader.NumPage() != 1 {
		t, _ := template.ParseFiles("error.html")
		t.Execute(w, "This doesn't seem to be a schedule")
		return
	}
	sched := genSchedule(PDFReader)
	state := randToken()
	schedules[state] = sched
	t, err := template.ParseFiles("verify.html")
	if err != nil {
		panic(err)
	}
	data := struct {
		State   string
		Classes []string
	}{
		State: state,
	}
	for _, class := range sched.classes {
		data.Classes = append(data.Classes, class.name)
	}
	t.Execute(w, data)
}

func exportPage(w http.ResponseWriter, r *http.Request) {
	state := r.FormValue("state")
	t, err := template.ParseFiles("export.html")
	if err != nil {
		panic(err)
	}
	t.Execute(w, state)
}

func loginPage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, genAuthURL(r.FormValue("state")), http.StatusTemporaryRedirect)
}

func authPage(w http.ResponseWriter, r *http.Request) {
	state := r.FormValue("state")
	found := false
	for key := range schedules {
		if !found {
			if key == state {
				found = true
			}
		}
	}
	if !found {
		fmt.Println("CSRF Attack Identified.")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	code := r.FormValue("code")
	tok, err := conf.Exchange(context.Background(), code)
	if err != nil {
		fmt.Printf("Oauth exchange failed with error %v.\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(tok))

	defer delete(schedules, state)
	srv, err := calendar.New(client)
	if err != nil {
		fmt.Println(err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	genCalendar(schedules[state], srv)
	http.ServeFile(w, r, "auth.html")
}

func icsPage(w http.ResponseWriter, r *http.Request) {
	state := r.FormValue("state")
	found := false
	for key := range schedules {
		if !found {
			if key == state {
				found = true
			}
		}
	}
	if !found {
		fmt.Println("CSRF Attack Identified.")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	defer delete(schedules, state)
	sched := schedules[state]
	var data []struct{
		Beginning string
		Ending string
		Room string
		Name string
	}
	data = make([]struct{Beginning string; Ending string; Room string; Name string}, len(sched.classes)*5)
	i := 0
	for _, class := range sched.classes {
		for _, meeting := range class.semester1 {
			beg, duration := lookupTime(meeting.day, meeting.period)
			end := beg.Add(duration)
			begH, begM, _ := beg.Clock()
			endH, endM, _ := end.Clock()
			var begStr, endStr string
			if begH < 10 {
				begStr = fmt.Sprintf("0%v:", begH)
			} else {
				begStr = fmt.Sprintf("%v:", begH)
			}
			if begM < 10 {
				begStr = fmt.Sprintf("%v0%v:00", begStr, begM)
			} else {
				begStr = fmt.Sprintf("%v%v:00", begStr, begM)
			}
			if endH < 10 {
				endStr = fmt.Sprintf("0%v:", endH)
			} else {
				endStr = fmt.Sprintf("%v:", endH)
			}
			if endM < 10 {
				endStr = fmt.Sprintf("%v0%v:00", endStr, endM)
			} else {
				endStr = fmt.Sprintf("%v%v:00", endStr, endM)
			}
			fmtDT := findDay(time.Weekday(meeting.day)).Format(time.RFC3339)
			date := string([]rune(fmtDT)[:strings.Index(fmtDT, "T")])
			begDT := date + " " + begStr
			endDT := date + " " + endStr
			data[i].Beginning = begDT
			data[i].Ending = endDT
			data[i].Room = meeting.room
			data[i].Name = class.name
			i++
			fmt.Printf("Beginning: %v\nEnding: %v\nRoom: %v\nName: %v\n", begDT, endDT, meeting.room, class.name)
		}
	}
	fmt.Println(data)
	jdata, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(fmt.Sprintf("%v.json", state), jdata, 0644)
	if err != nil {
		panic(err)
	}
	defer os.Remove(fmt.Sprintf("%v.json", state))
	cmd := exec.Command("./icsConv.py", fmt.Sprintf("%v.json", state))
	ics, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	w.Write(ics)
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
	return conf.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// Generates CSRF Token
// Taken from https://skarlso.github.io/2016/06/12/google-signin-with-go/
func randToken() string {
	tok := "/"
	for strings.Contains(tok, "/") {
		b := make([]byte, 32)
		rand.Read(b)
		tok = base64.StdEncoding.EncodeToString(b)
	}
	return tok
}

func genCalendar(sched schedule, srv *calendar.Service) {
	for _, class := range sched.classes {
		for _, classTime := range class.semester1 {
			fmtDT := findDay(time.Weekday(classTime.day)).Format(time.RFC3339)
			date := string([]rune(fmtDT)[:strings.Index(fmtDT, "T")])
			beginning, length := lookupTime(classTime.day, classTime.period)

			begH, begM, _ := beginning.Clock()
			var begStr string
			if begH < 10 {
				begStr = fmt.Sprintf("0%v:", begH)
			} else {
				begStr = fmt.Sprintf("%v:", begH)
			}
			if begM < 10 {
				begStr = fmt.Sprintf("%v0%v:00", begStr, begM)
			} else {
				begStr = fmt.Sprintf("%v%v:00", begStr, begM)
			}

			endH, endM, _ := beginning.Add(length).Clock()
			var endStr string
			if endH < 10 {
				endStr = fmt.Sprintf("0%v:", endH)
			} else {
				endStr = fmt.Sprintf("%v:", endH)
			}
			if endM < 10 {
				endStr = fmt.Sprintf("%v0%v:00", endStr, endM)
			} else {
				endStr = fmt.Sprintf("%v%v:00", endStr, endM)
			}

			event := &calendar.Event{
				Summary:     class.name,
				Description: fmt.Sprintf("Meets in room %v.", classTime.room),
				Start: &calendar.EventDateTime{
					DateTime: fmt.Sprintf("%vT%v", date, begStr),
					TimeZone: "America/New_York",
				},
				End: &calendar.EventDateTime{
					DateTime: fmt.Sprintf("%vT%v", date, endStr),
					TimeZone: "America/New_York",
				},
				Recurrence: []string{"RRULE:FREQ=WEEKLY;"},
			}
			_, err := srv.Events.Insert("primary", event).Do()
			if err != nil {
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
			if err != nil {
				panic(err)
			}
		} else {
			length, err = time.ParseDuration("50m")
			if err != nil {
				panic(err)
			}
		}
	} else {
		length, err = time.ParseDuration("40m")
		if err != nil {
			panic(err)
		}
	}
	if period == 1 {
		startTime := chooseTime(8, 30)
		if err != nil {
			panic(err)
		}
		return startTime, length
	} else if period == 2 {
		startTime := chooseTime(9, 15)
		if err != nil {
			panic(err)
		}
		return startTime, length
	} else if period == 3 {
		if weekday == 2 {
			startTime := chooseTime(10, 35)
			if err != nil {
				panic(err)
			}
			return startTime, length
		} else {
			startTime := chooseTime(10, 25)
			if err != nil {
				panic(err)
			}
			return startTime, length
		}
	} else if period == 4 {
		if weekday == 2 {
			startTime := chooseTime(11, 35)
			if err != nil {
				panic(err)
			}
			return startTime, length
		} else if weekday == 4 {
			startTime := chooseTime(11, 20)
			if err != nil {
				panic(err)
			}
			return startTime, length
		} else {
			startTime := chooseTime(11, 10)
			if err != nil {
				panic(err)
			}
			return startTime, length
		}
	} else if period == 5 {
		if weekday == 2 {
			startTime := chooseTime(12, 20)
			if err != nil {
				panic(err)
			}
			return startTime, length
		} else if weekday == 4 {
			startTime := chooseTime(13, 55)
			if err != nil {
				panic(err)
			}
			return startTime, length
		} else {
			startTime := chooseTime(11, 55)
			if err != nil {
				panic(err)
			}
			return startTime, length
		}
	} else if period == 6 {
		if weekday == 2 {
			startTime := chooseTime(13, 45)
			if err != nil {
				panic(err)
			}
			return startTime, length
		} else if weekday == 4 {
			startTime := chooseTime(14, 40)
			if err != nil {
				panic(err)
			}
			return startTime, length
		} else {
			startTime := chooseTime(13, 20)
			if err != nil {
				panic(err)
			}
			return startTime, length
		}
	} else {
		if weekday == 2 {
			startTime := chooseTime(14, 30)
			if err != nil {
				panic(err)
			}
			return startTime, length
		} else if weekday == 4 {
			startTime := chooseTime(15, 25)
			if err != nil {
				panic(err)
			}
			return startTime, length
		} else {
			startTime := chooseTime(14, 5)
			if err != nil {
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
		ClientID:     c.Web.Cid,
		ClientSecret: c.Web.Csecret,
		RedirectURL:  "http://localhost:8080/auth",
		Scopes:       []string{calendar.CalendarScope},
		Endpoint:     google.Endpoint,
	}
	schedules = make(map[string]schedule)
	http.HandleFunc("/", homePage)
	http.HandleFunc("/verify", verifyPage)
	http.HandleFunc("/auth", authPage)
	http.HandleFunc("/login", loginPage)
	http.HandleFunc("/export", exportPage)
	http.HandleFunc("/ics", icsPage)
	http.Handle("/trouble", http.RedirectHandler("https://www.github.com/christopherm99/student-calendar-populator", 300))
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	http.ListenAndServe(":8080", nil)
}
