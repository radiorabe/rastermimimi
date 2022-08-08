package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"sort"
	"sync/atomic"
	"time"
)

type App struct {
	config AppConfig

	errors atomic.Value
}

func NewApp(config AppConfig) *App {
	return &App{config: config}
}

func (a *App) Load() {
	grid := make(map[string]gridSlot)
	errors := make(gridErrors, 0)

	grid, errors = a.loadWebsite(grid, errors)
	grid = a.loadLibretime(grid)

	errors = a.checkForErrors(grid, errors)

	log.Printf("Loaded %d grid entries and %d errors.\n", len(grid), len(errors))

	a.errors.Store(errors)
}

func (a *App) GetErrors() gridErrors {
	return a.errors.Load().(gridErrors)
}

func (a *App) appendError(errors gridErrors, message string, start LocalTime, slot gridSlot) gridErrors {
	errors = append(errors, gridError{
		Message: message,
		Time:    start.GetTime(),
		Slot:    slot,
	})
	return errors
}

func (a *App) loadWebsite(grid map[string]gridSlot, errors gridErrors) (map[string]gridSlot, gridErrors) {
	// get config
	url := a.config.websiteURL
	duration := a.config.duration

	start := time.Now()
	end := start.AddDate(0, 0, duration)
	resp, err := http.Get(
		fmt.Sprintf(
			"%s/wp-admin/admin-ajax.php?action=eventorganiser-fullcal&start=%s&end=%s&timeformat=%s",
			url,
			start.Format("2006-01-02"),
			end.Format("2006-01-02"),
			"G%3Ai",
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
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
				errors = a.appendError(
					errors,
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
	return grid, errors
}

func (a *App) loadLibretime(grid map[string]gridSlot) map[string]gridSlot {
	// get config
	url := a.config.libretimeURL
	duration := a.config.duration

	// read from libretime calendar
	resp, err := http.Get(fmt.Sprintf("%s/api/live-info-v2?days=%d&shows=600000000", url, duration+1))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
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
	return grid
}

func (a *App) checkForErrors(grid map[string]gridSlot, errors gridErrors) gridErrors {
	for _, slot := range grid {
		// skip past events
		if slot.WebsiteEventOrganizerCalendarEvent.Title != "" && slot.WebsiteEventOrganizerCalendarEvent.Start.Before(time.Now()) {
			continue
		}
		if slot.WebsiteEventOrganizerCalendarEvent.Title != "" && slot.LibreTimeLiveInfoV2Show.Name == "" {
			if slot.WebsiteEventOrganizerCalendarEvent.Title != "Klangbecken" {
				errors = a.appendError(errors,
					fmt.Sprintf("%s fehlt in LibreTime.",
						slot.WebsiteEventOrganizerCalendarEvent.Title),
					slot.WebsiteEventOrganizerCalendarEvent.Start, slot)
			}
		} else if slot.LibreTimeLiveInfoV2Show.Name != "" && slot.WebsiteEventOrganizerCalendarEvent.Title == "" {
			if slot.LibreTimeLiveInfoV2Show.Name != "Klangbecken" {
				errors = a.appendError(errors,
					fmt.Sprintf("%s fehlt auf Web Seite.",
						slot.LibreTimeLiveInfoV2Show.Name),
					slot.LibreTimeLiveInfoV2Show.Starts, slot)
			}
		} else if slot.WebsiteEventOrganizerCalendarEvent.Title != slot.LibreTimeLiveInfoV2Show.Name {
			errors = a.appendError(errors,
				fmt.Sprintf("Titel auf Webseite (%s) stimmt nicht mit LibreTime (%s) überein.",
					slot.WebsiteEventOrganizerCalendarEvent.Title, slot.LibreTimeLiveInfoV2Show.Name),
				slot.WebsiteEventOrganizerCalendarEvent.Start, slot)
		}

		// skip events not on both sides for further checks
		if slot.WebsiteEventOrganizerCalendarEvent.Title == "" || slot.LibreTimeLiveInfoV2Show.Name == "" {
			continue
		}

		if slot.LibreTimeLiveInfoV2Show.URL != slot.WebsiteEventOrganizerCalendarEvent.URL {
			if slot.WebsiteEventOrganizerCalendarEvent.URL == "#" {
				errors = a.appendError(errors,
					fmt.Sprintf("Keine URL für Sendung %s auf Website hinterlegt.",
						slot.WebsiteEventOrganizerCalendarEvent.Title),
					slot.WebsiteEventOrganizerCalendarEvent.Start, slot)
			} else {
				errors = a.appendError(errors,
					fmt.Sprintf("URL auf Webseite (%s) stimmt nicht mit LibreTime (%s) überein.",
						slot.WebsiteEventOrganizerCalendarEvent.URL, slot.LibreTimeLiveInfoV2Show.URL),
					slot.WebsiteEventOrganizerCalendarEvent.Start, slot)
			}
		}

		// we allow a difference of > 10 minutes at the end to account for preproduced broadcasts that go into overtime
		if !slot.LibreTimeLiveInfoV2Show.Ends.Equal(slot.WebsiteEventOrganizerCalendarEvent.End.Time) && slot.LibreTimeLiveInfoV2Show.Ends.Time.Sub(slot.WebsiteEventOrganizerCalendarEvent.End.Time) > time.Minute*10 {
			// ignore shows that end at exactly 23:59:00 since they are an artifact of how we display shows on our wordpress page
			if slot.WebsiteEventOrganizerCalendarEvent.End.Format("15:04:05") != "23:59:00" {
				errors = a.appendError(errors,
					fmt.Sprintf("Ende auf Webseite (%s) stimmt nicht mit LibreTime (%s) überein.",
						slot.WebsiteEventOrganizerCalendarEvent.End.Format("02 Jan 2006 15:04:05"),
						slot.LibreTimeLiveInfoV2Show.Ends.Format("02 Jan 2006 15:04:05")),
					slot.WebsiteEventOrganizerCalendarEvent.Start, slot)
			}
		}

	}
	sort.Sort(errors)
	return errors
}
