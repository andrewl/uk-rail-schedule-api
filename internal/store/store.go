package store

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"time"
	"uk-rail-schedule-api/internal/schedule"

	"gorm.io/gorm"
)

// londonLocation is the Europe/London timezone, used when interpreting WTT times.
// WTT times in the CIF schedule feed are always UK local time (GMT/BST).
var londonLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		panic(fmt.Sprintf("failed to load Europe/London timezone: %v", err))
	}
	londonLocation = loc
}

// APIStatus holds summary counts returned by the /status endpoint.
type APIStatus struct {
	Version                      string
	ScheduleFileCount            int64
	VSTPCount                    int64
	EarliestVSTP                 string
	LatestVSTP                   string
	VSTPCountLastHalfHour        int64
	VSTPCountLastHour            int64
	VSTPCountLastSixHours        int64
	VSTPCountLastTwentyFourHours int64
}

// Store wraps a GORM database handle and provides schedule query methods.
type Store struct {
	DB      *gorm.DB
	Version string
}

func New(db *gorm.DB, version string) *Store {
	return &Store{DB: db, Version: version}
}

func (s *Store) GetStatus() (APIStatus, error) {
	var status APIStatus
	status.Version = s.Version

	if s.DB == nil {
		return status, errors.New("could not connect to database")
	}

	type sqlErr struct {
		dest *interface{}
		sql  string
	}

	queries := []struct {
		sql  string
		dest interface{}
	}{
		{"select count(*) from schedules where source = 'Feed'", &status.ScheduleFileCount},
		{"select count(*) from schedules where source = 'VSTP'", &status.VSTPCount},
		{"select coalesce(min(published_at), '') from schedules where source = 'VSTP'", &status.EarliestVSTP},
		{"select coalesce(max(published_at), '') from schedules where source = 'VSTP'", &status.LatestVSTP},
		{"select count(*) from schedules where source = 'VSTP' and published_at > datetime('now', '-30 minutes')", &status.VSTPCountLastHalfHour},
		{"select count(*) from schedules where source = 'VSTP' and published_at > datetime('now', '-60 minutes')", &status.VSTPCountLastHour},
		{"select count(*) from schedules where source = 'VSTP' and published_at > datetime('now', '-360 minutes')", &status.VSTPCountLastSixHours},
		{"select count(*) from schedules where source = 'VSTP' and published_at > datetime('now', '-1440 minutes')", &status.VSTPCountLastTwentyFourHours},
	}

	for _, q := range queries {
		if err := s.DB.Raw(q.sql).Scan(q.dest).Error; err != nil {
			return status, fmt.Errorf("error running sql: %w", err)
		}
	}

	return status, nil
}

func (s *Store) GetSchedules(headcode, date, toc, tiplocId string, hidePassedTrains bool) ([]schedule.Schedule, error) {
	var schedules []schedule.Schedule
	var tiploc schedule.Tiploc

	if s.DB == nil {
		return schedules, errors.New("db is nil")
	}

	var headcodeFilter string
	if headcode == "" {
		headcodeFilter = "1=1"
	} else {
		headcodeFilter = fmt.Sprintf("signalling_id = \"%s\"", headcode)
	}

	ts, err := time.Parse("2006-01-02", date)
	if err != nil {
		slog.Error("Failed to parse date", "date", date)
		return schedules, fmt.Errorf("failed to parse date %s", date)
	}

	startDate := ts.Unix()
	endDate := startDate + 86399
	dow := int(ts.Weekday())
	if dow == 0 {
		dow = 7
	}
	dayFilter := fmt.Sprintf(" and substr(schedule_days_runs, %d, 1) = \"1\" ", dow)

	var atocFilter string
	if toc != "any" {
		atocFilter = fmt.Sprintf(" and atoc_code = \"%s\" ", toc)
	}

	var tiplocFilter string
	if tiplocId != "" && tiplocId != "any" {
		tiplocFilter = fmt.Sprintf(" and id in (select schedule_id from schedule_locations where schedule_locations.tiploc_code = \"%s\")", tiplocId)
	}

	slog.Debug("filters",
		"headcode_filter", headcodeFilter,
		"start_date", startDate, "end_date", endDate,
		"day_filter", dayFilter,
		"atoc_filter", atocFilter,
		"tiploc_filter", tiplocFilter,
	)

	/* Query applies STP indicator rules:
C - Planned cancellation (train won't run)
N - STP schedule (cannot be overlaid)
O - Overlay schedule (alteration to permanent)
P - Permanent schedule
For any date, 'C' or 'O' beats 'P' (lowest alphabetical STP wins). */
	sqlErr := s.DB.Debug().Raw(
		"SELECT * FROM schedules WHERE (cif_stp_indicator = 'P' or cif_stp_indicator = 'N') AND "+
		headcodeFilter+" AND schedule_start_date_ts <= ? AND schedule_end_date_ts >= ? "+
		dayFilter+atocFilter+tiplocFilter,
		startDate, endDate,
		).Scan(&schedules).Error

	if sqlErr != nil {
		return nil, fmt.Errorf("error querying schedules: %w", sqlErr)
	}

	for idx := range schedules {
		s.DB.Debug().Model(&schedule.ScheduleLocation{}).Preload("Tiploc").Find(&schedules[idx].ScheduleLocation, "schedule_id = ?", schedules[idx].ID)
		slog.Debug("schedule", "idx", idx, "schedule_id", schedules[idx].ID, "locations", len(schedules[idx].ScheduleLocation))
	}

	var overlays []schedule.Schedule
	sqlErr = s.DB.Raw(
		"SELECT * FROM schedules WHERE source=\"VSTP\" AND (cif_stp_indicator = 'O' or cif_stp_indicator = 'C') AND "+
		headcodeFilter+" AND schedule_start_date_ts <= ? AND schedule_end_date_ts >= ? "+
		dayFilter+atocFilter+tiplocFilter,
		startDate, endDate,
		).Scan(&overlays).Error

	if sqlErr != nil {
		return nil, fmt.Errorf("error querying overlays: %w", sqlErr)
	}

	for idx := range overlays {
		s.DB.Find(&overlays[idx].ScheduleLocation, "schedule_id = ?", overlays[idx].ID)
	}

	for idx := range schedules {
		schedules[idx].ApplyOverlays(overlays, startDate)
	}

	// Enrich schedules with origin/destination station names and departure/arrival timestamps
	for idx, sch := range schedules {
		for _, l := range sch.ScheduleLocation {
			// LO - Originating location; TB - Train Begins (VSTP)
			if l.RecordIdentity == "LO" || l.RecordIdentity == "TB" {
				s.DB.Where("tiploc_code = ?", l.TiplocCode).First(&tiploc)
				schedules[idx].Origin = tiploc.TpsDescription
				schedules[idx].TimeOfDepartureFromOriginTS, _ = combineDateAndTime(ts.Unix(), l.Departure)
				schedules[idx].TimeOfDepartureFromOrigin = formatWTTTime(l.Departure)
			}
			// LT - Termination location; TF - Train Finishes (VSTP)
			if l.RecordIdentity == "LT" || l.RecordIdentity == "TF" {
				s.DB.Where("tiploc_code = ?", l.TiplocCode).First(&tiploc)
				schedules[idx].Destination = tiploc.TpsDescription
				schedules[idx].TimeOfArrivalAtDestinationTS, _ = combineDateAndTime(ts.Unix(), l.Arrival)
				schedules[idx].TimeOfArrivalAtDestination = formatWTTTime(l.Arrival)
			}
		}

		// If arrival is earlier than departure it's probably next-day; add 24 hours
		if schedules[idx].TimeOfArrivalAtDestinationTS < schedules[idx].TimeOfDepartureFromOriginTS {
			schedules[idx].TimeOfArrivalAtDestinationTS += 86400
		}
	}

	if hidePassedTrains {
		now := time.Now().Unix()
		filtered := schedules[:0]
		for _, sch := range schedules {
			if tiplocId != "" && tiplocId != "any" {
				// Find the scheduled time at the requested TIPLOC; keep if not yet passed.
				var tiplocTime int64
				for _, loc := range sch.ScheduleLocation {
					if loc.TiplocCode != tiplocId {
						continue
					}
					t := loc.Departure
					if t == "" {
						t = loc.Pass
					}
					if t == "" {
						t = loc.Arrival
					}
					if t != "" {
						tiplocTime, _ = combineDateAndTime(ts.Unix(), t)
					}
					break
				}
				if tiplocTime == 0 || tiplocTime >= now {
					filtered = append(filtered, sch)
				}
			} else {
				// No TIPLOC filter: keep if the train hasn't yet reached its destination.
				if sch.TimeOfArrivalAtDestinationTS == 0 || sch.TimeOfArrivalAtDestinationTS >= now {
					filtered = append(filtered, sch)
				}
			}
		}
		schedules = filtered
	}

	// Sort schedules in time order. When a TIPLOC is active, sort by the time
	// the train is at that TIPLOC (Arrival → Pass → Departure). Otherwise sort
	// by departure from origin.
	sortTiploc := ""
	if tiplocId != "" && tiplocId != "any" {
		sortTiploc = tiplocId
	}

	// getSortKey returns the sort timestamp for a schedule. Because sort.Slice
	// swaps elements of schedules in-place, the key must be computed from the
	// schedule value at the current index rather than from a pre-built array
	// (which would go out of sync with the slice as elements are moved).
	getSortKey := func(sch schedule.Schedule) int64 {
		if sortTiploc != "" {
			for _, loc := range sch.ScheduleLocation {
				if loc.TiplocCode != sortTiploc {
					continue
				}
				t := loc.Arrival
				if t == "" {
					t = loc.Pass
				}
				if t == "" {
					t = loc.Departure
				}
				if t != "" {
					key, _ := combineDateAndTime(ts.Unix(), t)
					return key
				}
				break
			}
			return 0
		}
		return sch.TimeOfDepartureFromOriginTS
	}

	sort.Slice(schedules, func(i, j int) bool {
		ki := getSortKey(schedules[i])
		kj := getSortKey(schedules[j])
		if ki == 0 {
			return false
		}
		if kj == 0 {
			return true
		}
		return ki < kj
	})

	if len(schedules) > 0 {
		return schedules, nil
	}
	return nil, nil
}

// combineDateAndTime combines a Unix date timestamp with a WTT time string "HHMM" or "HHMMSS".
// WTT times are always UK local time (GMT/BST), so Europe/London is used when constructing
// the combined timestamp to correctly account for daylight saving time.
func combineDateAndTime(date int64, wttTime string) (int64, error) {
	if len(wttTime) < 4 {
		return 0, fmt.Errorf("wtt time too short: %q", wttTime)
	}

	var currentTime time.Time
	if date != 0 {
		currentTime = time.Unix(date, 0).In(londonLocation)
	} else {
		currentTime = time.Now().In(londonLocation)
	}

	hours, err := strconv.Atoi(wttTime[:2])
	if err != nil {
		return 0, fmt.Errorf("error parsing hours: %w", err)
	}

	minutes, err := strconv.Atoi(wttTime[2:4])
	if err != nil {
		return 0, fmt.Errorf("error parsing minutes: %w", err)
	}

	newTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), hours, minutes, 0, 0, londonLocation)
	return newTime.Unix(), nil
}

// formatWTTTime formats a WTT time string "HHMM" or "HHMMSS" as "HH:MM".
func formatWTTTime(wttTime string) string {
	if len(wttTime) < 4 {
		return ""
	}
	return wttTime[:2] + ":" + wttTime[2:4]
}
