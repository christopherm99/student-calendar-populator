# Student Calendar Generator
Christopher Milan, 2018
## Overview
Contains golang server, and html templates.
WORK IN PROGRESS
## Setting it up
### Install golang:
Follow the steps listed at [the golang website.](https://golang.org/doc/install)
### Install dependencies:
This code relies on a few golang libraries, please install them before running the code.
* PDF reading library:
  * Found at https://github.com/rsc/pdf/
  * Install by running ```go get rsc.io/pdf```
* Golang OAuth library:
  * Found at https://github.com/golang/oauth2/
  * Install by running ```go get golang.org/x/oauth2```
* Google Calendar API:
  * Found at https://github.com/google/google-api-go-client/tree/master/calendar/v3/
  * Install by running ```go get google.golang.org/api/calendar/v3```
### Download the files:
* Make a dedicated directory for this project (eg. ```mkdir student-calendar```). 
* Move to that directory (eg. ```cd student-calendar```).
* Clone the files from github: ```git clone https://github.com/christopherm99/student-calendar-populator.git```.
### Run the program:
Either compile main.go to machine code (```go build```) or run the program directly (```go run main.go```). Now open a browser and visit ```localhost:8080```. The website should be online.
## Troubleshooting
### Likely errors:
Double check that all of the the dependencies are installed. Also check you golang version. This code has been tested on golang version 1.10.1 (the most recent at the time of this writing), but it may still work on other versions.
### Still having errors?
Open a new issue on github, and make sure to include the output of the program and the output of ```go env```.
