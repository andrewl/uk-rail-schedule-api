package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-stomp/stomp/v3"
	slogchi "github.com/samber/slog-chi"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
)

type APIStatus struct {
	ScheduleFileCount int64
	VSTPCount         int64
}

// Loads the schedule data from file into the database db
func refreshSchedules(filename string, db *gorm.DB) {

	if isRefreshingDatabase() {
		logger.Info("Not going to load - schedule feed is already loading in another process")
		return
	}

	startRefreshingDatabase()

	defer endRefreshingDatabase()

	// Open the source schedule file for reading
	file, err := os.Open(filename)
	if err != nil {
		logger.Error("Error opening schedule feed file. Cannot load.", "error", err)
		return
	}
	defer file.Close()

	// Create a scanner to read lines from the file
	scanner := bufio.NewScanner(file)

	// The first line should be the metadata
	scanner.Scan()
	line := scanner.Text()

	var scheduleFeedRecord ScheduleFeedRecord
	// Unmarshal the JSON line into the Schedule struct - if we can't then we're a bit stuck
	if err := json.Unmarshal([]byte(line), &scheduleFeedRecord); err != nil {
		logger.Error("Error unmarshaling JSON:", "error", err)
		return
	}

	//If the record we've just read in is not metadata then we're a bit stuck
	if !scheduleFeedRecord.IsMetadata() {
		logger.Error("Error record in feed file is not metadata, cannot continue loading feed file", "records", scheduleFeedRecord)
		return
	}

	var laterTimetable Timetable
	if err := db.Where("timestamp >= ?", scheduleFeedRecord.Timetable.Timestamp).First(&laterTimetable).Error; err == nil {
		logger.Info("There is a later timetable existing than that in the schedule feed file", "timetable", laterTimetable)
		return
	}

	// We'll batch the inserts
	schedules := []Schedule{}
	tiplocs := []Tiploc{}

	var existingSchedule Schedule

	// Iterate over each line in the file
	for scanner.Scan() {

		// Create a new Schedule instance to unmarshal the JSON into
		var scheduleFeedRecord ScheduleFeedRecord

		// Read the line
		line := scanner.Text()

		// Unmarshal the JSON line into the Schedule struct
		if err := json.Unmarshal([]byte(line), &scheduleFeedRecord); err != nil {
			logger.Error("Error unmarshaling scheduleFeedRecord JSON", "error", err)
			continue // Skip this line and move to the next one
		}

		if scheduleFeedRecord.IsSchedule() {
			schedule := scheduleFeedRecord.JSONScheduleV1.ToSchedule()
			schedule.AugmentSchedule()

			if err := db.Where("combined_id = ?", schedule.CombinedID).First(&existingSchedule).Error; err != nil {
				schedule.ID = existingSchedule.ID
			}

			schedules = append(schedules, schedule)
			if len(schedules) == 10 {
				db.Save(&schedules)
				schedules = nil
			}
		}

		if scheduleFeedRecord.IsTiploc() {
			tiplocs = append(tiplocs, scheduleFeedRecord.Tiploc)
			if len(tiplocs) == 10 {
				db.Save(&tiplocs)
				tiplocs = nil
			}
		}

	}

	if len(schedules) > 0 {
		db.Save(&schedules)
	}

	if len(tiplocs) > 0 {
		db.Save(&tiplocs)
	}

	// lets insert the latest timetable
	db.Create(&scheduleFeedRecord.Timetable)

}

// Process a STOMP message	
// This is a blocking function - it will wait until a message is received on the subscription
// and then process it
// If there is an error then it will return the error
func processStompMessage(subscription *stomp.Subscription, db *gorm.DB) error {
	logger.Debug("Waiting for a message from STOMP subscription")
	msg := <-subscription.C
	var vstpMsg VSTPStompMsg
	if msg != nil && msg.Body != nil {
		logger.Debug("Got a message from VSTP subscription")
		os.WriteFile("/tmp/vstp-msg.json", msg.Body, 0644)
		if err := json.Unmarshal(msg.Body, &vstpMsg); err != nil {
			logger.Error("Error decoding STOMP message json", "error", err, "msg.Body", msg.Body)
			return err
		}
		schedule := vstpMsg.VSTPCIFMsgV1.VSTPSchedule.ToSchedule()
		schedule.AugmentSchedule()
		logger.Debug("Inserting schedule into db from STOMP message")
		db.Create(&schedule)
	} else {
		logger.Error("STOMP message body is empty - will stop consuming more messages", "msg", msg)
		return msg.Err
	}
	return nil
}


// Load VSTP data from the VSTP feed
func loadVSTP(db *gorm.DB) {

	url, username, password := getStompConnectionDetails()
	if url == "" {
		logger.Info("VSTP stomp url is empty - will NOT load from VSTP feed")
		return
	}

	var stompConn *stomp.Conn
	var sub *stomp.Subscription
	err := errors.New("")
	timeout := 1
	max_timeout := 60

	for {

		if stompConn == nil {

			logger.Debug("Dialling a new STOMP connection", "url", url, "username", username)

			stompConn, err = stomp.Dial("tcp", url,
				stomp.ConnOpt.HeartBeat(10*60*time.Second, 10*60*time.Second),
				stomp.ConnOpt.Login(username, password))

			// no connection - backoff and retry
			if err != nil {
				logger.Warn(fmt.Sprintf("Could not connect to stomp. Pausing for %d seconds before retrying", timeout))
				time.Sleep(time.Duration(timeout) * time.Second)
				timeout = timeout * 2
				if timeout > max_timeout {
					timeout = max_timeout
				}
			}

			if err == nil {

				defer stompConn.Disconnect()

				sub, err = stompConn.Subscribe("/topic/VSTP_ALL", stomp.AckClient)

				if err != nil {
					logger.Error("There was an error connecting to STOMP server - disconnecting", "err", err)
					sub.Unsubscribe()
					stompConn.Disconnect()
					stompConn = nil
				}

			}
		}

		if sub != nil {

			err := processStompMessage(sub, db)

			if err != nil {
				logger.Error("There was an error processing message. Disconnecting from STOMP server", "err", err)
				if sub.Active() {
					stompConn.Disconnect()
				}
				stompConn = nil
			}
		}
	}
}

type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string `json:"status"`          // user-level status message
	AppCode    int64  `json:"code,omitempty"`  // application-specific error code
	ErrorText  string `json:"error,omitempty"` // application-level error message, for debugging
}

func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

var ErrNotFound = &ErrResponse{HTTPStatusCode: 404, StatusText: "Resource not found."}
var ErrUnprocessable = &ErrResponse{HTTPStatusCode: 422, StatusText: "Unprocessable entity."}
var db *gorm.DB
var logger *slog.Logger

// Refreshing the database is a long running process - this variable and the following functions are
// called when it starts and when it ends
var refreshingDatabase = false

func isRefreshingDatabase() bool {
	return refreshingDatabase
}

func startRefreshingDatabase() {
	logger.Info("start refreshing database")
	refreshingDatabase = true
}

func endRefreshingDatabase() {

	if db != nil && shouldDeleteExpiredSchedulesAfterRefresh() {
		logger.Debug("Deleting expired schedules")
		currentTimestamp := time.Now().Unix()
		db.Delete(&Schedule{}, "schedule_end_date_ts < ?", currentTimestamp)

	} else {
		logger.Debug("Not deleting expired schedules from database")
	}

	logger.Info("end refreshing database")
	refreshingDatabase = false
}

func getScheduleFeedFilename() string {
	return getConfigValue("schedule_feed_filename")
}

func getDatabaseFilename() string {
	return getConfigValue("database")
}

func getStompConnectionDetails() (url string, login string, password string) {
	url = getConfigValue("stomp_url")
	login = getConfigValue("stomp_login")
	password = getConfigValue("stomp_password")
	return url, login, password
}

// Should we delete expired schedules from the database after a refresh?
func shouldDeleteExpiredSchedulesAfterRefresh() bool {
	return getConfigValue("delete_expired_schedules_on_refresh") == "yes"
}

func getConfigValue(key string) (value string) {
	value, _ = viper.Get(key).(string)
	return value
}

// Open the database and create it if it doesn't exist
func openDB(databaseFilename string) bool {
	err := errors.New("")
	database_is_new := false
	// If the database doesn't exist then attempt to create it by importing from schedule.json
	if _, err := os.Stat(databaseFilename); errors.Is(err, os.ErrNotExist) {
		database_is_new = true
		logger.Info("Database doesn't exist - creating", "databaseFilename", databaseFilename)
	}

	db, err = gorm.Open(sqlite.Open(databaseFilename), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	if database_is_new {
		db.AutoMigrate(&ScheduleLocation{}, &Schedule{}, &Tiploc{}, &Timetable{})
	}

	return true
}

func main() {

	//set some default configuration
	viper.SetDefault("stomp_url", "publicdatafeeds.networkrail.co.uk:61618")
	viper.SetDefault("database", "ukra.db")
	viper.SetDefault("schedule_feed_filename", "schedule.json")
	viper.SetDefault("log_filename", "")
	viper.SetDefault("listen_on", "127.0.0.1:3333")
	viper.SetDefault("delete_expired_schedules_on_refresh", "no")

	//load in config
	viper.SetConfigName("config")
	viper.AddConfigPath("/etc/ukra/")
	viper.AddConfigPath("$HOME/.ukra")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %w", err))
	}

	var logOutput *os.File
	if getConfigValue("log_filename") != "" {

		logOutput, err = os.OpenFile(getConfigValue("log_filename"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			panic("Error opening log file: " + err.Error())
		}

		defer logOutput.Close()

	} else {

		logOutput = os.Stderr

	}

	logHandler := slog.NewTextHandler(logOutput, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	})

	logger = slog.New(logHandler)

	openDB(getDatabaseFilename())
	go refreshSchedules(getScheduleFeedFilename(), db)
	go loadVSTP(db)

	// OK we've got this far and have a valid database - let's serve some requests
	r := chi.NewRouter()

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(slogchi.New(logger))
	r.Use(middleware.Recoverer)
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(60 * time.Second))

	// REST routes for "schedule" resource
	r.Route("/schedules/{identifierType}/{identifier}", func(r chi.Router) {
		r.Use(schedulesCtx)
		r.Get("/", getSchedules)
	})

	r.Route("/refresh", func(r chi.Router) {
		r.Get("/", runRefresh)
	})

	r.Route("/status", func(r chi.Router) {
		r.Use(statusCtx)
		r.Get("/", getStatus)
	})

	logger.Info("Serving data", "ip address and port", getConfigValue("listen_on"))

	err = http.ListenAndServe(getConfigValue("listen_on"), r)

	if err != nil {
		logger.Error("Failure to serve requests", "error", err)
		os.Exit(1)
	}
}

func statusCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, err := dbGetStatus()
		if err != nil {
			//http.Error(w, http.StatusText(500), 500)
			http.Error(w, err.Error(), 500)
			return
		}
		ctx := context.WithValue(r.Context(), "status", status)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func schedulesCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identifierType := chi.URLParam(r, "identifierType")
		identifier := chi.URLParam(r, "identifier")
		var date string
		if r.URL.Query().Has("date") {
			date = r.URL.Query().Get("date")

		} else {
			currentTime := time.Now()
			date = currentTime.Format("2006-01-02")
		}
		var toc string
		if r.URL.Query().Has("toc") {
			toc = r.URL.Query().Get("toc")

		} else {
			toc = "any"
		}
		var location string
		if r.URL.Query().Has("location") {
			location = r.URL.Query().Get("location")

		} else {
			location = "any"
		}

		schedules, err := dbGetSchedules(identifierType, identifier, date, toc, location)
		if err != nil {
			//http.Error(w, http.StatusText(500), 500)
			http.Error(w, err.Error(), 500)
			return
		}
		if len(schedules) == 0 {
			http.Error(w, http.StatusText(404), 404)
			return
		}
		ctx := context.WithValue(r.Context(), "schedules", schedules)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func runRefresh(w http.ResponseWriter, r *http.Request) {
	if isRefreshingDatabase() {
		w.WriteHeader(409)
		render.JSON(w, r, "Database already being refreshed. Please try again later")
	} else {
		go refreshSchedules(getScheduleFeedFilename(), db)
		w.WriteHeader(201)
		render.JSON(w, r, "Refreshing")
	}
}

func getStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status, ok := ctx.Value("status").(APIStatus)
	if !ok {
		render.Render(w, r, ErrUnprocessable)
		return
	}
	render.JSON(w, r, status)
}

func getSchedules(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	schedules, ok := ctx.Value("schedules").([]Schedule)
	if !ok {
		render.Render(w, r, ErrUnprocessable)
		return
	}
	render.JSON(w, r, schedules)
}

func combineDateAndTime(date int64, wtt_time string) (timestamp int64, err error) {

	currentTime := time.Now()

	if date != 0 {
		currentTime = time.Unix(date, 0)
	}

	// Parse the "HHMM" string into separate hours and minutes
	hours, err := strconv.Atoi(wtt_time[:2])
	if err != nil {
		return timestamp, errors.New(fmt.Sprintf("Error parsing hours: %s", err))
	}

	minutes, err := strconv.Atoi(wtt_time[2:4])
	if err != nil {
		return timestamp, errors.New(fmt.Sprintf("Error parsing minutes: %s", err))
	}

	// Combine the current date with the parsed hours and minutes
	newTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), hours, minutes, 0, 0, currentTime.Location())

	// Convert the newTime to a Unix timestamp (integer)
	timestamp = newTime.Unix()

	return timestamp, nil
}

func dbGetStatus() (APIStatus, error) {

	var status APIStatus

	if db == nil {
		logger.Error("Cannot get schedules - db does not exist")
		return status, errors.New("db is nil")
	}

	sqlError := db.Raw("select count(*) from schedules where source = 'Feed'").Scan(&status.ScheduleFileCount).Error

	if sqlError != nil {
		return status, errors.New("There was an error running the sql: " + sqlError.Error())
	}

	sqlError = db.Raw("select count(*) from schedules where source = 'VSTP'").Scan(&status.VSTPCount).Error

	if sqlError != nil {
		return status, errors.New("There was an error running the sql: " + sqlError.Error())
	}

	return status, nil
}

func dbGetSchedules(identifierType string, identifier string, date string, toc string, location string) ([]Schedule, error) {

	var schedules []Schedule
	var tiploc Tiploc

	if db == nil {
		logger.Error("Cannot get schedules - db does not exist")
		return schedules, errors.New("db is nil")
	}

	var identifier_filter string
	var day_filter string
	var atoc_filter string
	var location_filter string
	var start_date int64
	var end_date int64

	// Work out what sort of identifier we have (headcode or train uid) and filter by it
	if identifierType == "headcode" || identifierType == "signallingid" {
		identifier_filter = fmt.Sprintf("signalling_id = \"%s\"", identifier)
	}

	if identifierType == "ciftrainuid" || identifierType == "trainuid" {
		identifier_filter = fmt.Sprintf("cif_train_uid = \"%s\"", identifier)
	}

	// If we don't have a filter then bail out
	if identifier_filter == "" {
		logger.Error("Failed to understand how to identifier")
		return schedules, errors.New("Failed to understand how to identifier - need to set either headcode, signallingid, ciftrainuid or trainuid")
	}

	// Parse the date and get the day of week in order to filter
	ts, err := time.Parse("2006-01-02", date)
	if err != nil {
		logger.Error("Failed to parse date " + date)
		return schedules, errors.New("Failed to parse date " + date)
	}

	start_date = ts.Unix()
	end_date = start_date + 86399
	dow := int(ts.Weekday())

	if dow == 0 {
		dow = 7
	}
	day_filter = fmt.Sprintf(" and substr(schedule_days_runs, %d, 1) = \"1\" ", dow)

	// If we've passed in a specific toc, then filter on it
	if toc != "any" {
		atoc_filter = fmt.Sprintf(" and atoc_code = \"%s\" ", toc)
	}

	// If we've passed in a specific toc, then filter on it
	if location != "any" {
		//need to construct this as a subselect because of a bug in gorm when using joins
		location_filter = fmt.Sprintf(" and id in (select schedule_id from schedule_locations where schedule_locations.tiploc_code = \"%s\")", location)
	}

	logger.Debug("filters", "identifier_filter", identifier_filter, "start_date", start_date, "end_date", end_date, "day_filter", day_filter, "atoc_filter", atoc_filter, "location_filter", location_filter)

	/* This query applies the following rules as described here in order to ensure that any cancellations or short-term
	planning schedule is selected ahead of any permanent schedule.

	Schedule validities are between a start date and an end date, and on particular days of the week. They each have a Short Term Planning (STP) indicator field as follows:

	C - Planned cancellation: the schedule does not apply on this date, and the train will not run. Typically seen on public holidays when an alternate schedule applies, or on Christmas Day.
	N - STP schedule: similar to a permanent schedule, but planned through the Short Term Planning process and not capable of being overlaid
	O - Overlay schedule: an alteration to a permanent schedule
	P - Permanent schedule: a schedule planned through the Long Term Planning proces

	Permanent ('P') schedules can be overlaid by another schedule with the same UID - either a Variation ('O') or Cancellation Variation ('C'). For any particular day, of all the schedules for that UID valid on that day, the 'C' or 'O' schedule is the one which applies. Conveniently, it also means that the lowest alphabetical STP indicator wins - 'C' and 'O' are both lower in the alphabet than 'P'.

	This process allows a different schedule to be valid on particular days, or the service to not be valid on that day. For example, a schedule may be valid Monday - Friday each day of the year, but have a Cancellation Variation on Christmas Day and Boxing Day only.
	*/

	sqlError := db.Raw("SELECT * FROM schedules WHERE (cif_stp_indicator = 'P' or cif_stp_indicator = 'N') AND "+identifier_filter+" AND schedule_start_date_ts <= ? AND schedule_end_date_ts >= ? "+day_filter+atoc_filter+location_filter, start_date, end_date).Scan(&schedules).Error

	if sqlError != nil {
		return nil, errors.New("There was an error running the sql to get the schedules: " + sqlError.Error())
	}

	/* Because we used raw sql in the above query we didn't automatically load the schedule locations. This does that */
	for idx := range schedules {
		db.Find(&schedules[idx].ScheduleLocation, "schedule_id = ?", schedules[idx].ID)
		logger.Debug("schedule", "idx", idx, "schedule_id", schedules[idx].ID, "locations", len(schedules[idx].ScheduleLocation))
	}

	var overlays []Schedule

	sqlError = db.Raw("SELECT * FROM schedules WHERE source=\"VSTP\" AND (cif_stp_indicator = 'O' or cif_stp_indicator = 'C') AND "+identifier_filter+" AND schedule_start_date_ts <= ? AND schedule_end_date_ts >= ? "+day_filter+atoc_filter+location_filter, start_date, end_date).Scan(&overlays).Error

	if sqlError != nil {
		return nil, errors.New("There was an error running the sql to get the overlays: " + sqlError.Error())
	}

	/* Because we used raw sql in the above query we didn't automatically load the schedule locations. This does that */
	for idx := range overlays {
		db.Find(&overlays[idx].ScheduleLocation, "schedule_id = ?", overlays[idx].ID)
	}

	/* Take each schedule and attempt to apply them */
	for idx := range schedules {
		schedules[idx].ApplyOverlays(overlays, start_date)
	}

	//add the names of the origin and destination stations to the main schedule object
	for idx, s := range schedules {
		for _, l := range s.ScheduleLocation {

			// LO - Originating location - location where the train service starts from
			// TB - Train Begins (from vstp feeds)
			if l.RecordIdentity == "LO" || l.RecordIdentity == "TB" {
				originTiplocCode := l.TiplocCode
				db.Where("tiploc_code = ?", originTiplocCode).First(&tiploc)
				schedules[idx].Origin = tiploc.TpsDescription
				schedules[idx].TimeOfDepartureFromOriginTS, _ = combineDateAndTime(ts.Unix(), l.Departure)
			}

			// LT - Termination location - location where the train service ends
			// TF - Train Finishes (from vstp feeds)
			if l.RecordIdentity == "LT" || l.RecordIdentity == "TF" {
				destinationTiplocCode := l.TiplocCode
				db.Where("tiploc_code = ?", destinationTiplocCode).First(&tiploc)
				schedules[idx].Destination = tiploc.TpsDescription
				schedules[idx].TimeOfArrivalAtDestinationTS, _ = combineDateAndTime(ts.Unix(), l.Arrival)
			}
		}

		//if the TimeOfArrival is earlier than TimeOfDeparture it's probably because arrival is the next day (these
		//figures are only stored in hours and minutes). If this is the case add 24 hours onto the time of arrival
		if schedules[idx].TimeOfArrivalAtDestinationTS < schedules[idx].TimeOfDepartureFromOriginTS {
			schedules[idx].TimeOfArrivalAtDestinationTS += 86400
		}

	}

	if len(schedules) > 0 {
		return schedules, nil
	}

	return nil, nil
}
