package main

import (
	"rsc.io/pdf"
	"fmt"
	"regexp"
	strings "strings"
	"encoding/csv"
	"os"
	"strconv"
	"net/http"
	"io"
)

// ===== PDF Handling =====

type times struct {
	day int
	period int
	room string
}

type class struct {
	name string
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
			//fmt.Println("New line.")
		} else {
			//fmt.Println("New character on line.")
			lines[len(lines)-1] = lines[len(lines)-1] + x.S
		}
		prevY = x.Y
	}
	var unparsedClasses []string
	for _, line := range lines {
		//fmt.Println(line)
		matched, _ := regexp.MatchString("[0-9]{3}-[0-9]{2}", line)
		if matched {
			unparsedClasses = append(unparsedClasses, line)
		}
	}
	unparsedClasses = unparsedClasses[1:len(unparsedClasses)-1]
	var classes []struct{
		code string
		rooms []string
	}
	for _, text := range unparsedClasses {
		//fmt.Println(text)
		hyphen := strings.Index(text, "-")
		var rooms []string
		if !strings.Contains(text, "/") {
			short := strings.Replace(text[hyphen+4:len([]rune(text))], "S", "", -1)
			re := regexp.MustCompile("[0-9]|LA")
			strRooms := short[re.FindStringIndex(short)[0]:]
			for i := 0; i + 1 < len(strRooms); i+=2 {
				rooms = append(rooms, string([]rune(strRooms)[i:i+2]))
			}
		}
		var newClass struct {
			code string
			rooms []string
		}
		newClass.code = string([]rune(text)[hyphen-3:hyphen+3])
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

func uploadPage(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	file, _, err := r.FormFile("pdf")
	if err != nil {
		panic(err)
	}

	f, err := os.OpenFile("./temp/Schedule.pdf", os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
	}

	io.Copy(f, file)
	file.Close()
	f.Close()

	pdfReader, err := pdf.Open("./temp/Schedule.pdf")
	if err != nil {
		panic(err)
	}
	sched := genSchedule(pdfReader)
	for _, class := range sched.classes {
		strSched = fmt.Printf("%v\n\n%v")
	}
	fmt.Fprintf(w, "%v", )
}

// ===== Google API Integration =====

func main() {
	http.HandleFunc("/upload", uploadPage)
}
