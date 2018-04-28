# Student Calendar Generator
Christopher Milan, 2018
## Overview
Contains Golang server backend, HTML templates, and other necessary files.
## Setting it up
### Install Golang:
Follow the steps listed at [the Golang website.](https://golang.org/doc/install)
### Install dependencies:
This code relies on a few golang libraries, please install them before running the code.
* PDF reading library:
  * See at https://github.com/rsc/pdf/
  * Install by running ```go get rsc.io/pdf```
* Golang OAuth library:
  * See at https://github.com/golang/oauth2/
  * Install by running ```go get golang.org/x/oauth2```
* Google Calendar API:
  * See at https://github.com/google/google-api-go-client/tree/master/calendar/v3/
  * Install by running ```go get google.golang.org/api/calendar/v3```
### Download the files:
* Make a dedicated directory for this project (eg. ```mkdir student-calendar```). 
* Move to that directory (eg. ```cd student-calendar```).
* Clone the files from GitHub: ```git clone https://github.com/christopherm99/student-calendar-populator.git```.
### Set up the Google API:

Visit your [Google Cloud Platform Credentials Page](https://console.cloud.google.com/apis/credentials) and either create or select an existing project. Then click "Create credentials", and select OAuth client ID. Then select Web application, give it a name, add http://localhost:8080/auth (or whatever domain this will be running on) to the authorized redirect URIs, and hit create. After that, head back to the Credentials screen, and click the download icon next to the client ID you just created. Rename this file to "client_secret.json", and move it into the directory with the main.go file. 

### Run the program:

Either compile main.go to machine code (```go build```) or run the program directly (```go run main.go```). Now open a browser and visit ```localhost:8080```. The website should be running.
## Troubleshooting

### Likely errors:
Double check that all of the the dependencies are installed. Also check you golang version. This code has been tested on golang version 1.10.1 (the most recent at the time of this writing), but it may still work on other versions.
### Still having errors?
Open a new issue on github, and make sure to include the output of the program and the output of ```go env```.
