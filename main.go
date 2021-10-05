/**
 * rastermimimi reads calendar data from various sources and displays then as html.
 */

package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"text/template"
	"time"
	_ "time/tzdata"
)

type LocalTime struct {
	time.Time
}

func (t *LocalTime) UnmarshalJSON(b []byte) error {
	_, offset := time.Now().Local().Zone()
	s := string(b)
	// get rid of leading and trailing " from JSON"
	s = s[1 : len(s)-1]
	// replace " " with "T" so it works for LibreTime as well as web
	s = s[0:10] + `T` + s[11:]
	// add local zone to the end ssince we always work with local time
	s = fmt.Sprintf("%s+%02d00", s, offset/3600)
	p, err := time.Parse(`2006-01-02T15:04:05Z0700`, s)
	if err != nil {
		return err
	}
	t.Time = p
	return nil
}
func (t *LocalTime) GetTime() time.Time {
	return time.Time(t.Time)
}

type WebsiteEventOrganizerCalendarEvent struct {
	AllDay      bool      `json:"allDay"`
	Category    []string  `json:"category"`
	ClassName   []string  `json:"className"`
	Description string    `json:"description"`
	End         LocalTime `json:"end"`
	Organiser   int       `json:"organiser"`
	Start       LocalTime `json:"start"`
	Tags        []string  `json:"tags"`
	TextColor   string    `json:"textColor"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
}

type LibreTimeLiveInfoV2Show struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Genre       string    `json:"genre"`
	ID          int       `json:"id"`
	InstanceID  int       `json:"instance_id"`
	Record      int       `json:"record"`
	URL         string    `json:"url"`
	ImagePath   string    `json:"image_path"`
	Starts      LocalTime `json:"starts"`
	Ends        LocalTime `json:"ends"`
}
type LibreTimeLiveInfoV2Shows struct {
	Previous []interface{}             `json:"previous"`
	Current  LibreTimeLiveInfoV2Show   `json:"current"`
	Next     []LibreTimeLiveInfoV2Show `json:"next"`
}
type LibreTimeLiveInfoV2 struct {
	Station map[string]string        `json:"station"`
	Tracks  interface{}              `json:"tracks"`
	Shows   LibreTimeLiveInfoV2Shows `json:"shows"`
	Sources map[string]string        `json:"sources"`
}

type gridSlot struct {
	WebsiteEventOrganizerCalendarEvent WebsiteEventOrganizerCalendarEvent
	LibreTimeLiveInfoV2Show            LibreTimeLiveInfoV2Show
}

type gridError struct {
	Message string
	Time    time.Time
	Slot    gridSlot
}

type gridErrors []gridError

func (g gridErrors) Len() int {
	return len(g)
}
func (g gridErrors) Less(i, j int) bool {
	return g[i].Time.Before(g[j].Time)
}
func (g gridErrors) Swap(i, j int) {
	g[i], g[j] = g[j], g[i]
}

// string => gridSlot map
var grid map[string]gridSlot
var errors gridErrors
var websiteURL string
var libretimeURL string
var duration int

func main() {
	websiteURL = "https://rabe.ch"
	libretimeURL = "https://airtime.service.int.rabe.ch"
	// check next 2 months
	duration = 2 * 30

	reloadState()

	indexTpl := `<!DOCTYPE html>
	<title>Raster Mimimi</title>
	<style>
		html, body {
			font-family: sans-serif;
			font-size: 0.9em;
		}
		article {
			border: 1px solid gray;
			padding: 1em;
			margin-bottom: 0.5em;
		}
		section {
			margin-left: 2em;
		}
		section.lead {
			font-size: 1.5em;
			margin-left: 0;
		}
	</style>
	<h1>technisches Programmraster Mimimi</h1>
	<nav><a href="/refresh">Refresh...</a> | <a href="#" onclick="alert('ja nid usdrucke!, so vowäge umwäut!')">Drucken...</a> | <a href="https://github.com/radiorabe/rastermimimi">GitHub...</a></nav>
	<p>Dieses Tool hat in den nächsten 60 Tagen {{.|len}} Mimimis gefunden.</p>
	{{$otime := "02 Jan 2006 15:04:05"}}
	{{range .}}
	{{$time := .Time.Format "02 Jan 2006 15:04:05"}}
	{{if ne $time $otime}}
	<hr>
	<h2>{{$time}}</h2>
	{{end}}
	<article>
		<section class="lead">{{.Message}}</section>
		<!-- {{.}} -->
		{{if .Slot.WebsiteEventOrganizerCalendarEvent.Title}}
		<section>
		<h3>Web: {{.Slot.WebsiteEventOrganizerCalendarEvent.Title}}</h3>
		<p>{{.Slot.WebsiteEventOrganizerCalendarEvent.Start.Format "02 Jan 2006 15:04:05"}} - {{.Slot.WebsiteEventOrganizerCalendarEvent.End.Format "02 Jan 2006 15:04:05"}}</p>
		<p><b>URL:</b> <code>{{.Slot.WebsiteEventOrganizerCalendarEvent.URL}}</code></p>
		</section>
		{{end}}
		{{if .Slot.LibreTimeLiveInfoV2Show.Name}}
		<section>
		<h3>LibreTime: {{.Slot.LibreTimeLiveInfoV2Show.Name}}</h3>
		<p>{{.Slot.LibreTimeLiveInfoV2Show.Starts.Format "02 Jan 2006 15:04:05"}} - {{.Slot.LibreTimeLiveInfoV2Show.Ends.Format "02 Jan 2006 15:04:05"}}</p>
		<p><b>URL:</b> <code>{{.Slot.LibreTimeLiveInfoV2Show.URL}}</code></p>
		</section>
		{{end}}
	</article>
	{{$otime = $time}}
	{{end}}
	`
	tmpl := template.Must(template.New("index").Parse(indexTpl))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		err := tmpl.Execute(w, errors)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
	http.HandleFunc("/refresh", func(rw http.ResponseWriter, r *http.Request) {
		reloadState()
		http.Redirect(rw, r, "/", http.StatusTemporaryRedirect)
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func reloadState() {
	grid = make(map[string]gridSlot)
	errors = make(gridErrors, 0)

	// load data from sources and write loading errors to errors
	readFromWebsite(websiteURL, time.Now(), duration)
	readFromLibretime(libretimeURL, duration)

	// find errors, writes to "errors" as a side effect
	checkForErrors()
}

func readFromWebsite(url string, start time.Time, duration int) {
	startReq := start.Format("2006-01-02")
	endReq := start.AddDate(0, 0, duration).Format("2006-01-02")
	resp, err := http.Get(
		fmt.Sprintf(
			"%s/wp-admin/admin-ajax.php?action=eventorganiser-fullcal&start=%s&end=%s&timeformat=%s",
			url,
			startReq,
			endReq,
			"G%3Ai",
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	// parse JSON
	var events []WebsiteEventOrganizerCalendarEvent
	err = json.Unmarshal(body, &events)
	if err != nil {
		log.Fatal(err)
	}
	for _, event := range events {
		start := event.Start.String()
		slot, ok := grid[start]
		if ok {
			if slot.WebsiteEventOrganizerCalendarEvent.Title != "" {
				appendError(
					fmt.Sprintf("Mehrfacheintrag auf Webseite: %s (neu: %s)",
						slot.WebsiteEventOrganizerCalendarEvent.Title, event.Title),
					slot.WebsiteEventOrganizerCalendarEvent.Start, slot)
			}
			slot.WebsiteEventOrganizerCalendarEvent = event
			grid[start] = slot
		} else {
			grid[start] = gridSlot{
				WebsiteEventOrganizerCalendarEvent: event,
			}
		}
	}
}

func readFromLibretime(url string, duration int) {
	// read from libretime calendar
	resp, err := http.Get(fmt.Sprintf("%s/api/live-info-v2?days=%d&shows=600000000", url, duration+1))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	// parse JSON
	var liveInfo LibreTimeLiveInfoV2
	err = json.Unmarshal(body, &liveInfo)
	if err != nil {
		log.Fatal(err)
	}
	liveInfo.Shows.Next = append(liveInfo.Shows.Next, liveInfo.Shows.Current)
	for _, ltShow := range liveInfo.Shows.Next {
		start := ltShow.Starts.String()
		slot, ok := grid[start]

		if ok {
			if slot.LibreTimeLiveInfoV2Show.Name != "" {
				log.Panicf("slot %s already has a libretime title", start)
			}
			ltShow.Name = html.UnescapeString(ltShow.Name)
			slot.LibreTimeLiveInfoV2Show = ltShow
			grid[start] = slot
		} else {
			grid[start] = gridSlot{
				LibreTimeLiveInfoV2Show: ltShow,
			}
		}
	}
}

func checkForErrors() {
	for _, slot := range grid {
		// skip past events
		if slot.WebsiteEventOrganizerCalendarEvent.Title != "" && slot.WebsiteEventOrganizerCalendarEvent.Start.Before(time.Now()) {
			continue
		}
		if slot.WebsiteEventOrganizerCalendarEvent.Title != "" && slot.LibreTimeLiveInfoV2Show.Name == "" {
			if slot.WebsiteEventOrganizerCalendarEvent.Title != "Klangbecken" {
				appendError(fmt.Sprintf("%s fehlt im LibreTime.",
					slot.WebsiteEventOrganizerCalendarEvent.Title),
					slot.WebsiteEventOrganizerCalendarEvent.Start, slot)
			}
		} else if slot.LibreTimeLiveInfoV2Show.Name != "" && slot.WebsiteEventOrganizerCalendarEvent.Title == "" {
			if slot.LibreTimeLiveInfoV2Show.Name != "Klangbecken" {
				appendError(
					fmt.Sprintf("%s fehlt auf der Webseite.",
						slot.LibreTimeLiveInfoV2Show.Name),
					slot.LibreTimeLiveInfoV2Show.Starts, slot)
			}
		} else if slot.WebsiteEventOrganizerCalendarEvent.Title != slot.LibreTimeLiveInfoV2Show.Name {
			appendError(
				fmt.Sprintf("Titel auf der Webseite (%s) stimmt nicht mit LibreTime (%s) überein.",
					slot.WebsiteEventOrganizerCalendarEvent.Title, slot.LibreTimeLiveInfoV2Show.Name),
				slot.WebsiteEventOrganizerCalendarEvent.Start, slot)
		}

		// skip events not on both sides for further checks
		if slot.WebsiteEventOrganizerCalendarEvent.Title == "" || slot.LibreTimeLiveInfoV2Show.Name == "" {
			continue
		}

		if slot.LibreTimeLiveInfoV2Show.URL != slot.WebsiteEventOrganizerCalendarEvent.URL {
			if slot.WebsiteEventOrganizerCalendarEvent.URL == "#" {
				appendError(
					fmt.Sprintf("Sendung %s hat keine URL auf der Webseite.",
						slot.WebsiteEventOrganizerCalendarEvent.Title),
					slot.WebsiteEventOrganizerCalendarEvent.Start, slot)
			} else {
				appendError(
					fmt.Sprintf("URL auf der Webseite (%s) stimmt nicht mit LibreTime (%s) überein.",
						slot.WebsiteEventOrganizerCalendarEvent.URL, slot.LibreTimeLiveInfoV2Show.URL),
					slot.WebsiteEventOrganizerCalendarEvent.Start, slot)
			}
		}

		// we allow a difference of > 10 minutes at the end to account for preproduced broadcasts that go into overtime
		if !slot.LibreTimeLiveInfoV2Show.Ends.Equal(slot.WebsiteEventOrganizerCalendarEvent.End.Time) && slot.LibreTimeLiveInfoV2Show.Ends.Time.Sub(slot.WebsiteEventOrganizerCalendarEvent.End.Time) > time.Minute*10 {
			appendError(
				fmt.Sprintf("Ende auf der Webseite (%s) stimmt nicht mit LibreTime (%s) überein.",
					slot.WebsiteEventOrganizerCalendarEvent.End.Format("02 Jan 2006 15:04:05"),
					slot.LibreTimeLiveInfoV2Show.Ends.Format("02 Jan 2006 15:04:05")),
				slot.WebsiteEventOrganizerCalendarEvent.Start, slot)
		}

		// check if description is set on web (archiv.rabe.ch uses this information)
		if slot.WebsiteEventOrganizerCalendarEvent.Description == "" {
			appendError(
				fmt.Sprintf("Beschreibung für Sendung %s fehlt auf der Webseite.",
					slot.WebsiteEventOrganizerCalendarEvent.Title),
				slot.WebsiteEventOrganizerCalendarEvent.Start, slot)
		}

	}
	sort.Sort(errors)
}

func appendError(message string, start LocalTime, slot gridSlot) {
	errors = append(errors, gridError{
		Message: message,
		Time:    start.GetTime(),
		Slot:    slot,
	})
}
