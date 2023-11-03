package main

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestUnmarshalJSONAndAugmentJSONSchedule(t *testing.T) {
	// Read the JSON file
	jsonData, err := ioutil.ReadFile(filepath.Join("test-fixtures", "feed.json"))
	if err != nil {
		t.Fatal("Failed to read JSON file:", err)
	}

	var scheduleFeedRecord ScheduleFeedRecord

	// Unmarshal the JSON data into the struct
	if err := json.Unmarshal(jsonData, &scheduleFeedRecord); err != nil {
		t.Fatal("Failed to unmarshal JSON:", err)
	}

	schedule := scheduleFeedRecord.JSONScheduleV1.ToSchedule()

	// Call AugmentSchedule() on the result
	schedule.AugmentSchedule()

	// Assert that TransactionType is "Create"
	expect(schedule.TransactionType, "TransactionType", "Create", t)
	expect(schedule.CIFStpIndicator, "CIFStpIndicator", "P", t)
	expect(schedule.CIFPowerType, "CIFPowerType", "DMU", t)
	expect(schedule.CIFTrainCategory, "CIFTrainCategory", "OO", t)
	expect(schedule.SignallingID, "SignallingID", "2A20", t)
	expect(schedule.ScheduleDaysRuns, "ScheduleDaysRuns", "0000001", t)
	expect(schedule.ScheduleStartDate, "ScheduleStartDate", "2023-05-21", t)
	expect(schedule.ScheduleEndDate, "ScheduleEndDate", "2023-12-03", t)
	expect(schedule.ScheduleLocation[0].TiplocCode, "Location 0 TiplocCode", "DRBY", t)
	expect(schedule.ScheduleLocation[0].RecordIdentity, "Location 0 LocationType", "LO", t)
	expect(schedule.ScheduleLocation[0].Departure, "Location 0 Departure", "0756", t)

}
