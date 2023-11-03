package main

// All lines in the schedule record contain one of three types
// JSONTimetableV1 (metadata)
// JSONScheduleV1 (a schedule)
// Tiploc (a location)
type ScheduleFeedRecord struct {
	Timetable      Timetable      `json:"JsonTimetableV1,omitempty"`
	JSONScheduleV1 JSONScheduleV1 `json:"JsonScheduleV1,omitempty"`
	Tiploc         Tiploc         `json:"TiplocV1,omitempty"`
}

type Tiploc struct {
	TransactionType string `json:"transaction_type"`
	TiplocCode      string `gorm:"index" json:"tiploc_code"`
	Nalco           string `json:"nalco"`
	Stanox          string `json:"stanox"`
	CrsCode         string `json:"crs_code"`
	Description     string `json:"description"`
	TpsDescription  string `json:"tps_description"`
}

type Timetable struct {
	Classification string            `json:"classification"`
	Timestamp      int               `json:"timestamp"`
	Owner          string            `json:"owner"`
	Sender         TimetableSender   `json:"Sender" gorm:"-:all"`
	Metadata       TimetableMetadata `json:"Metadata" gorm:"-:all"`
}

type TimetableSender struct {
	Organisation string `json:"organisation"`
	Application  string `json:"application"`
	Component    string `json:"component"`
}
type TimetableMetadata struct {
	Type     string `json:"type"`
	Sequence int    `json:"sequence"`
}

type JSONScheduleV1 struct {
	CIFBankHolidayRunning string `json:"CIF_bank_holiday_running"`
	CIFStpIndicator       string `json:"CIF_stp_indicator"`
	CIFTrainUID           string `json:"CIF_train_uid"`
	ApplicableTimetable   string `json:"applicable_timetable"`
	AtocCode              string `json:"atoc_code"`
	NewScheduleSegment    struct {
		TractionClass string `json:"traction_class"`
		UicCode       string `json:"uic_code"`
	} `json:"new_schedule_segment"`
	ScheduleDaysRuns string `json:"schedule_days_runs"`
	ScheduleEndDate  string `json:"schedule_end_date"`
	ScheduleSegment  struct {
		SignallingID                string             `json:"signalling_id"`
		CIFTrainCategory            string             `json:"CIF_train_category"`
		CIFHeadcode                 string             `json:"CIF_headcode"`
		CIFCourseIndicator          int                `json:"CIF_course_indicator"`
		CIFTrainServiceCode         string             `json:"CIF_train_service_code"`
		CIFBusinessSector           string             `json:"CIF_business_sector"`
		CIFPowerType                string             `json:"CIF_power_type"`
		CIFTimingLoad               string             `json:"CIF_timing_load"`
		CIFSpeed                    string             `json:"CIF_speed"`
		CIFOperatingCharacteristics string             `json:"CIF_operating_characteristics"`
		CIFTrainClass               string             `json:"CIF_train_class"`
		CIFSleepers                 string             `json:"CIF_sleepers"`
		CIFReservations             string             `json:"CIF_reservations"`
		CIFConnectionIndicator      string             `json:"CIF_connection_indicator"`
		CIFCateringCode             string             `json:"CIF_catering_code"`
		CIFServiceBranding          string             `json:"CIF_service_branding"`
		ScheduleLocation            []ScheduleLocation `json:"schedule_location"`
	} `json:"schedule_segment"`
	ScheduleStartDate string `json:"schedule_start_date"`
	TrainStatus       string `json:"train_status"`
	TransactionType   string `json:"transaction_type"`
}

func (s *ScheduleFeedRecord) IsMetadata() bool {
	return s.Timetable.Classification != ""
}

func (s *ScheduleFeedRecord) IsSchedule() bool {
	return s.JSONScheduleV1.CIFTrainUID != ""
}

func (s *ScheduleFeedRecord) IsTiploc() bool {
	return s.Tiploc.TiplocCode != ""
}

func (s *JSONScheduleV1) ToSchedule() (sch Schedule) {

	sch.Source = "Feed"

	sch.CIFBankHolidayRunning = s.CIFBankHolidayRunning
	sch.CIFStpIndicator = s.CIFStpIndicator
	sch.CIFTrainUID = s.CIFTrainUID
	sch.ApplicableTimetable = s.ApplicableTimetable
	sch.AtocCode = s.AtocCode
	sch.ScheduleDaysRuns = s.ScheduleDaysRuns
	sch.ScheduleEndDate = s.ScheduleEndDate
	sch.ScheduleStartDate = s.ScheduleStartDate
	sch.TrainStatus = s.TrainStatus
	sch.TransactionType = s.TransactionType

	// Move the scheduling segment fields into the schedule segment
	sch.SignallingID = s.ScheduleSegment.SignallingID
	sch.CIFTrainCategory = s.ScheduleSegment.CIFTrainCategory
	sch.CIFTrainCategory = s.ScheduleSegment.CIFTrainCategory
	sch.CIFHeadcode = s.ScheduleSegment.CIFHeadcode
	sch.CIFCourseIndicator = s.ScheduleSegment.CIFCourseIndicator
	sch.CIFTrainServiceCode = s.ScheduleSegment.CIFTrainServiceCode
	sch.CIFBusinessSector = s.ScheduleSegment.CIFBusinessSector
	sch.CIFPowerType = s.ScheduleSegment.CIFPowerType
	sch.CIFTimingLoad = s.ScheduleSegment.CIFTimingLoad
	sch.CIFSpeed = s.ScheduleSegment.CIFSpeed
	sch.CIFOperatingCharacteristics = s.ScheduleSegment.CIFOperatingCharacteristics
	sch.CIFTrainClass = s.ScheduleSegment.CIFTrainClass
	sch.CIFSleepers = s.ScheduleSegment.CIFSleepers
	sch.CIFHeadcode = s.ScheduleSegment.CIFHeadcode
	sch.CIFCourseIndicator = s.ScheduleSegment.CIFCourseIndicator
	sch.CIFTrainServiceCode = s.ScheduleSegment.CIFTrainServiceCode
	sch.CIFBusinessSector = s.ScheduleSegment.CIFBusinessSector
	sch.CIFPowerType = s.ScheduleSegment.CIFPowerType
	sch.CIFTimingLoad = s.ScheduleSegment.CIFTimingLoad
	sch.CIFSpeed = s.ScheduleSegment.CIFSpeed
	sch.CIFOperatingCharacteristics = s.ScheduleSegment.CIFOperatingCharacteristics
	sch.CIFTrainClass = s.ScheduleSegment.CIFTrainClass
	sch.CIFSleepers = s.ScheduleSegment.CIFSleepers
	sch.CIFReservations = s.ScheduleSegment.CIFReservations
	sch.CIFConnectionIndicator = s.ScheduleSegment.CIFConnectionIndicator
	sch.CIFCateringCode = s.ScheduleSegment.CIFCateringCode
	sch.CIFServiceBranding = s.ScheduleSegment.CIFServiceBranding

	sch.ScheduleLocation = make([]ScheduleLocation, len(s.ScheduleSegment.ScheduleLocation))
	copy(sch.ScheduleLocation, s.ScheduleSegment.ScheduleLocation)

	sch.TractionClass = s.NewScheduleSegment.TractionClass
	sch.UicCode = s.NewScheduleSegment.UicCode

	return sch
}
