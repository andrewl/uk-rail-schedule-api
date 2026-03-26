package config

import (
	"errors"
	"log/slog"
	"os"
	"path"
)

func GetScheduleFeedFilename() string {
	filename := os.Getenv("SCHEDULE_FEED_FILENAME")
	if filename == "" {
		slog.Debug("No SCHEDULE_FEED_FILENAME environment variable set - defaulting to ./data/schedule.json")
		filename = "./data/schedule.json"
	}
	return filename
}

func GetDataDir() string {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		slog.Debug("No DATA_DIR environment variable set - defaulting to ./data")
		dir = "./data"
	}
	return dir
}

func GetDatabaseFilename() string {
	return path.Join(GetDataDir(), "/ukra.db")
}

func GetStompConnectionDetails() (err error, url string, login string, password string) {
	url = os.Getenv("NR_STOMP_URL")
	if url == "" {
		slog.Debug("No NR_STOMP_URL environment variable set - defaulting to publicdatafeeds.networkrail.co.uk:61618")
		url = "publicdatafeeds.networkrail.co.uk:61618"
	}
	login = os.Getenv("NR_STOMP_LOGIN")
	password = os.Getenv("NR_STOMP_PASSWORD")
	if login == "" || password == "" {
		err = errors.New("NR_STOMP_LOGIN and NR_STOMP_PASSWORD environment variables must be set to connect to the VSTP feed")
		slog.Error(err.Error())
		return err, url, login, password
	}
	return nil, url, login, password
}

func GetHTTPListenAddress() string {
	addr := os.Getenv("LISTEN_ON")
	if addr == "" {
		slog.Debug("No LISTEN_ON environment variable set - defaulting to localhost:1333")
		addr = "localhost:1333"
	}
	return addr
}

func ShouldDeleteExpiredSchedulesAfterRefresh() bool {
	return os.Getenv("DELETE_EXPIRED_SCHEDULES_ON_REFRESH") == "yes"
}
