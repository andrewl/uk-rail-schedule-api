package sync_test

import (
	"fmt"
	"testing"

	"uk-rail-schedule-api/internal/schedule"
	internalsync "uk-rail-schedule-api/internal/sync"
)

// validVSTPJSON is a minimal VSTP message with a parseable timestamp.
const validVSTPJSON = `{
  "VSTPCIFMsgV1": {
    "timestamp": "1697155200000",
    "classification": "train movement",
    "owner": "Network Rail",
    "originMsgId": "test-001",
    "Sender": {
      "organisation": "Network Rail",
      "application": "TSIA",
      "component": "TSIA",
      "userID": "",
      "sessionID": ""
    },
    "schedule": {
      "CIF_train_uid": "T99999",
      "transaction_type": "Create",
      "train_status": "P",
      "CIF_stp_indicator": "N",
      "schedule_start_date": "2023-10-13",
      "schedule_end_date": "2023-10-13",
      "schedule_days_runs": "0000100",
      "CIF_speed": "100",
      "schedule_segment": [
        {
          "signalling_id": "5T99",
          "CIF_train_service_code": "99999000",
          "CIF_train_category": "EE",
          "CIF_power_type": "EMU",
          "schedule_location": [
            {
              "location": { "tiploc": { "tiploc_id": "WATRLOO" } },
              "CIF_activity": "TB",
              "scheduled_departure_time": "080000",
              "public_departure_time": "0800"
            },
            {
              "location": { "tiploc": { "tiploc_id": "CLPHMJN" } },
              "CIF_activity": "TF",
              "scheduled_arrival_time": "085500",
              "public_arrival_time": "0855"
            }
          ]
        }
      ]
    }
  }
}`

func TestInsertVSTPFromBytes_ValidJSON(t *testing.T) {
	db := setupTestDB(t)

	err := internalsync.InsertVSTPFromBytes([]byte(validVSTPJSON), db)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var count int64
	db.Model(&schedule.Schedule{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 schedule inserted, got %d", count)
	}
}

func TestInsertVSTPFromBytes_ScheduleFields(t *testing.T) {
	db := setupTestDB(t)

	if err := internalsync.InsertVSTPFromBytes([]byte(validVSTPJSON), db); err != nil {
		t.Fatal(err)
	}

	var sch schedule.Schedule
	db.First(&sch)

	if sch.CIFTrainUID != "T99999" {
		t.Errorf("expected CIFTrainUID 'T99999', got %q", sch.CIFTrainUID)
	}
	if sch.SignallingID != "5T99" {
		t.Errorf("expected SignallingID '5T99', got %q", sch.SignallingID)
	}
	if sch.Source != "VSTP" {
		t.Errorf("expected Source 'VSTP', got %q", sch.Source)
	}
}

func TestInsertVSTPFromBytes_InvalidJSON(t *testing.T) {
	db := setupTestDB(t)

	err := internalsync.InsertVSTPFromBytes([]byte("not valid json {{{"), db)
	if err == nil {
		t.Error("expected an error for invalid JSON, got nil")
	}

	var count int64
	db.Model(&schedule.Schedule{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 schedules after invalid JSON, got %d", count)
	}
}

func TestInsertVSTPFromBytes_InvalidTimestamp(t *testing.T) {
	db := setupTestDB(t)

	invalidTimestampJSON := fmt.Sprintf(`{
  "VSTPCIFMsgV1": {
    "timestamp": "not-a-number",
    "schedule": {}
  }
}`)

	err := internalsync.InsertVSTPFromBytes([]byte(invalidTimestampJSON), db)
	if err == nil {
		t.Error("expected an error for non-numeric timestamp, got nil")
	}
}

func TestInsertVSTPFromBytes_MultipleCallsInsertMultipleSchedules(t *testing.T) {
	db := setupTestDB(t)

	for i := 0; i < 3; i++ {
		if err := internalsync.InsertVSTPFromBytes([]byte(validVSTPJSON), db); err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	var count int64
	db.Model(&schedule.Schedule{}).Count(&count)
	if count != 3 {
		t.Errorf("expected 3 schedules after 3 inserts, got %d", count)
	}
}
