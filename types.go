package main

import (
	"fmt"
	"time"
)

type AppConfig struct {
	duration int // in days

	libretimeURL string
	websiteURL   string

	listenAddr string
}

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

// gridError is a single error at a specific time with a message and a pointer to the grid slot.
type gridError struct {
	Message string
	Time    time.Time
	Slot    gridSlot
}

// gridErrors are sortable collections of gridError.
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
