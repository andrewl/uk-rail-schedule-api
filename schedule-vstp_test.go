package main

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestUnmarshalJSONAndAugmentSchedule(t *testing.T) {
	// Read the JSON file
	jsonData, err := ioutil.ReadFile(filepath.Join("test-fixtures", "vstp.json"))
	if err != nil {
		t.Fatal("Failed to read JSON file:", err)
	}

	// Define a VSTPCIFMsgV1 struct
	var vstpMsg VSTPStompMsg

	// Unmarshal the JSON data into the struct
	if err := json.Unmarshal(jsonData, &vstpMsg); err != nil {
		t.Fatal("Failed to unmarshal JSON:", err)
	}

	// Call ToSchedule() on the struct
	schedule := vstpMsg.VSTPCIFMsgV1.VSTPSchedule.ToSchedule()

	// Call AugmentSchedule() on the result
	schedule.AugmentSchedule()

	// Assert that TransactionType is "Create"
	expect(schedule.CIFTrainUID, "CIFTrainUID", "03478", t)
	expect(schedule.TransactionType, "TransactionType", "Create", t)
	expect(schedule.CIFStpIndicator, "CIFStpIndicator", "N", t)
	expect(schedule.CIFPowerType, "CIFPowerType", "EMU", t)
	expect(schedule.CIFTrainCategory, "CIFTrainCategory", "EE", t)
	expect(schedule.SignallingID, "SignallingID", "5B94", t)
	expect(schedule.ScheduleDaysRuns, "ScheduleDaysRuns", "0000100", t)
	expect(schedule.ScheduleStartDate, "ScheduleStartDate", "2023-10-13", t)
	expect(schedule.ScheduleEndDate, "ScheduleEndDate", "2023-10-13", t)
	expect(schedule.ScheduleLocation[0].TiplocCode, "Location 0 TiplocCode", "ROCKFRY", t)
	expect(schedule.ScheduleLocation[0].RecordIdentity, "Location 0 LocationType", "TB", t)
	expect(schedule.ScheduleLocation[0].Departure, "Location 0 Departure", "235600", t)

}

func expect(value any, name string, expected_value any, t *testing.T) bool {

	if value != expected_value {
		t.Errorf("Expected %s to be '%s', but got '%s'", name, expected_value, value)
		return false
	}

	return true
}
