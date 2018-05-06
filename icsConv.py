import sys
import datetime
import time
import json
from ics import Calendar, Event
filename = sys.argv[1]
jfile = open(filename, "r")
if jfile.mode == "r":
    jdata = jfile.read()
jfile.close()
data = json.loads(jdata)
c = Calendar()
for meeting in data:
    if meeting["Name"] != "":
        newEvent = Event()
        newEvent.name = meeting["Name"]
        newEvent.begin = meeting["Beginning"]
        newEvent.end = meeting["Ending"]
        newEvent.description = "Meets in room " + meeting["Room"] + "."
        c.events.append(newEvent)
print(str(c))
