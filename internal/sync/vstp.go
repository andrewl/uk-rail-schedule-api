package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strconv"
	"time"
	"uk-rail-schedule-api/internal/schedule"

	gostomp "github.com/go-stomp/stomp/v3"
	"gorm.io/gorm"
)

// ListenForVSTP connects to the Network Rail STOMP server and processes incoming VSTP messages,
// writing each to disk and inserting it into the database. It retries on connection failure with
// exponential backoff.
func ListenForVSTP(db *gorm.DB, stompURL, login, password, dataDir string) {
	var stompConn *gostomp.Conn
	var sub *gostomp.Subscription
	var err error
	timeout := 1
	maxTimeout := 60

	for {
		if stompConn == nil {
			slog.Debug("Dialling a new STOMP connection", "url", stompURL, "username", login)

			stompConn, err = gostomp.Dial("tcp", stompURL,
				gostomp.ConnOpt.HeartBeat(10*60*time.Second, 10*60*time.Second),
				gostomp.ConnOpt.Login(login, password))

			if err != nil {
				slog.Warn(fmt.Sprintf("Could not connect to stomp. Pausing for %d seconds before retrying", timeout))
				time.Sleep(time.Duration(timeout) * time.Second)
				timeout = timeout * 2
				if timeout > maxTimeout {
					timeout = maxTimeout
				}
				continue
			}

			defer stompConn.Disconnect()
			sub, err = stompConn.Subscribe("/topic/VSTP_ALL", gostomp.AckClient)
			if err != nil {
				slog.Error("There was an error subscribing to STOMP topic - disconnecting", "err", err)
				sub.Unsubscribe()
				stompConn.Disconnect()
				stompConn = nil
				continue
			}
		}

		if sub != nil {
			if err := processVSTPMessage(sub, db, dataDir); err != nil {
				slog.Error("There was an error processing message. Disconnecting from STOMP server", "err", err)
				if sub.Active() {
					stompConn.Disconnect()
				}
				stompConn = nil
			}
		}
	}
}

func processVSTPMessage(subscription *gostomp.Subscription, db *gorm.DB, dataDir string) error {
	slog.Debug("Waiting for a message from STOMP subscription")
	msg := <-subscription.C
	if msg == nil || msg.Body == nil {
		slog.Error("STOMP message body is empty - will stop consuming more messages", "msg", msg)
		return errors.New("STOMP message body is empty")
	}

	slog.Debug("Got a message from VSTP subscription")

	// Persist the raw message to disk so it can be replayed after a database deletion
	filename := path.Join(dataDir, "vstp-"+strconv.FormatInt(time.Now().Unix(), 10)+".json")
	os.WriteFile(filename, msg.Body, 0644)

	if err := InsertVSTPFromBytes(msg.Body, db); err != nil {
		slog.Error("Failed to insert vstp message", "error", err)
		return err
	}
	return nil
}

// InsertVSTPFromBytes parses a raw VSTP STOMP message body and inserts it into the database.
func InsertVSTPFromBytes(data []byte, db *gorm.DB) error {
	var vstpMsg schedule.VSTPStompMsg

	if err := json.Unmarshal(data, &vstpMsg); err != nil {
		slog.Error("Error decoding STOMP message json", "error", err)
		return err
	}

	parsedTimestamp, err := strconv.ParseInt(vstpMsg.VSTPCIFMsgV1.Timestamp, 10, 64)
	if err != nil {
		slog.Error("Error parsing VSTP timestamp", "error", err, "timestamp", vstpMsg.VSTPCIFMsgV1.Timestamp)
		return err
	}

	sch := vstpMsg.VSTPCIFMsgV1.VSTPSchedule.ToSchedule(time.Unix(parsedTimestamp/1000, 0))
	sch.AugmentSchedule()
	db.Create(&sch)
	return nil
}
