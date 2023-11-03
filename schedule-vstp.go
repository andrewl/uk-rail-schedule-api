package main

import (
	"fmt"
	"strconv"
	"time"
)

type VSTPStompMsg struct {
	VSTPCIFMsgV1 VSTPCIFMsgV1 `json:"VSTPCIFMsgV1"`
}
type VSTPCIFMsgV1 struct {
	VSTPSchedule   VSTPSchedule `json:"schedule"`
	VSTPSender     VSTPSender   `json:"Sender"`
	Classification string       `json:"classification"`
	Timestamp      string       `json:"timestamp"`
	Owner          string       `json:"owner"`
	OriginMsgID    string       `json:"originMsgId"`
}

type VSTPSchedule struct {
	ScheduleSegment []struct {
		ScheduleLocation []struct {
			Location struct {
				Tiploc struct {
					TiplocID string `json:"tiploc_id"`
				} `json:"tiploc"`
			} `json:"location"`
			ScheduledPassTime       string `json:"scheduled_pass_time"`
			ScheduledDepartureTime  string `json:"scheduled_departure_time"`
			ScheduledArrivalTime    string `json:"scheduled_arrival_time"`
			PublicDepartureTime     string `json:"public_departure_time"`
			PublicArrivalTime       string `json:"public_arrival_time"`
			CIFPath                 string `json:"CIF_path,omitempty"`
			CIFActivity             string `json:"CIF_activity,omitempty"`
			CIFPathingAllowance     string `json:"CIF_pathing_allowance,omitempty"`
			CIFEngineeringAllowance string `json:"CIF_engineering_allowance,omitempty"`
			CIFPerformanceAllowance string `json:"CIF_performance_allowance,omitempty"`
			CIFLine                 string `json:"CIF_line,omitempty"`
		} `json:"schedule_location"`
		SignallingID        string `json:"signalling_id"`
		CIFTrainServiceCode string `json:"CIF_train_service_code"`
		CIFTrainCategory    string `json:"CIF_train_category"`
		CIFPowerType        string `json:"CIF_power_type"`
	} `json:"schedule_segment"`
	TransactionType     string `json:"transaction_type"`
	TrainStatus         string `json:"train_status"`
	ScheduleStartDate   string `json:"schedule_start_date"`
	ScheduleEndDate     string `json:"schedule_end_date"`
	ScheduleDaysRuns    string `json:"schedule_days_runs"`
	ApplicableTimetable string `json:"applicable_timetable"`
	CIFTrainUID         string `json:"CIF_train_uid"`
	CIFStpIndicator     string `json:"CIF_stp_indicator"`
	CIFSpeed            string `json:"CIF_speed,omitempty"`
}

type VSTPSender struct {
	Organisation string `json:"organisation"`
	Application  string `json:"application"`
	Component    string `json:"component"`
	UserID       string `json:"userID"`
	SessionID    string `json:"sessionID"`
}

func (s *VSTPSchedule) ToSchedule() (sch Schedule) {

	sch.Source = "VSTP"

	// Move the scheduling segment fields into the schedule segment
	sch.TransactionType = s.TransactionType
	sch.CIFStpIndicator = s.CIFStpIndicator
	sch.CIFTrainUID = s.CIFTrainUID
	sch.ScheduleDaysRuns = s.ScheduleDaysRuns
	sch.ScheduleStartDate = s.ScheduleStartDate
	sch.ScheduleEndDate = s.ScheduleEndDate
	speed, err := strconv.Atoi(s.CIFSpeed)
	if err != nil {
		sch.CIFSpeed = ""
	}
	if err == nil {
		sch.CIFSpeed = fmt.Sprintf("%.0f", float64(speed)/2.24)
	}

	if len(s.ScheduleSegment) > 0 {
		sch.SignallingID = s.ScheduleSegment[0].SignallingID
		sch.CIFTrainCategory = s.ScheduleSegment[0].CIFTrainCategory
		sch.CIFTrainServiceCode = s.ScheduleSegment[0].CIFTrainServiceCode
		sch.CIFPowerType = s.ScheduleSegment[0].CIFPowerType
		sch.ScheduleLocation = make([]ScheduleLocation, len(s.ScheduleSegment[0].ScheduleLocation))
		for i := 0; i < len(sch.ScheduleLocation); i++ {
			sch.ScheduleLocation[i].PublicArrival = s.ScheduleSegment[0].ScheduleLocation[i].PublicArrivalTime
			sch.ScheduleLocation[i].PublicDeparture = s.ScheduleSegment[0].ScheduleLocation[i].PublicDepartureTime
			sch.ScheduleLocation[i].Arrival = s.ScheduleSegment[0].ScheduleLocation[i].ScheduledArrivalTime
			sch.ScheduleLocation[i].Departure = s.ScheduleSegment[0].ScheduleLocation[i].ScheduledDepartureTime
			sch.ScheduleLocation[i].Pass = s.ScheduleSegment[0].ScheduleLocation[i].CIFPath
			sch.ScheduleLocation[i].RecordIdentity = s.ScheduleSegment[0].ScheduleLocation[i].CIFActivity
			sch.ScheduleLocation[i].LocationType = s.ScheduleSegment[0].ScheduleLocation[i].CIFActivity
			sch.ScheduleLocation[i].TiplocCode = s.ScheduleSegment[0].ScheduleLocation[i].Location.Tiploc.TiplocID
		}
	}

	/*
		sch.TractionClass = s.NewScheduleSegment[0].TractionClass
		sch.UicCode = s.NewScheduleSegment[0].UicCode

		sch.CIFTrainCategoryDescription = s(schedule[0].CIFTrainCategory)
		sch.CIFOperatingCharacteristicsDescription = s(schedule[0].CIFOperatingCharacteristics)
		sch.CIFPowerTypeDescription = s(schedule[0].CIFPowerType)
		sch.TrainStatusDescription = s(schedule[0].TrainStatus)
		sch.CIFTimingLoadDescription = s(schedule[0].CIFTimingLoad, schedule.CIFPowerType)
		sch.AtocCodeDescription = s(schedule[0].AtocCode)
	*/

	// Deefine the layout format that matches the input string
	layout := "2006-01-02 15:04:05"
	// Parse the input string into a time.Time object
	ts, err := time.Parse(layout, sch.ScheduleStartDate+" 00:00:00")
	if err != nil {
		fmt.Println("Failed to parse start date for schedule :", err)
	}
	if err == nil {
		sch.ScheduleStartDateTS = ts.Unix()
	}

	ts, err = time.Parse(layout, sch.ScheduleEndDate+" 23:59:59")
	if err != nil {
		fmt.Println("Failed to parse end date for schedule :", err)
	}
	if err == nil {
		sch.ScheduleEndDateTS = ts.Unix()
	}

	return sch

}
