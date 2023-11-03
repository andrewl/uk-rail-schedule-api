package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Schedule struct {
	ID uint64 `gorm:"primaryKey"`
	//Trains are sets of schedules identified by a common UID. A schedule for a specific train service can be uniquely identified by UID, Start Date and STP Indicator. (from https://wiki.openraildata.com/index.php?title=SCHEDULE)
	CombinedID string `gorm:"index"`
	//This is 'Feed' for the schedule feed file, or 'VSTP' for records sourced from VSTP
	Source string `json:"source,omitempty"`

	CIFBankHolidayRunning  string `gorm:"index" json:"CIF_bank_holiday_running,omitempty"`
	CIFStpIndicator        string `gorm:"index" json:"CIF_stp_indicator,omitempty"`
	CIFTrainUID            string `gorm:"index" json:"CIF_train_uid,omitempty"`
	ApplicableTimetable    string `json:"applicable_timetable,omitempty"`
	AtocCode               string `json:"atoc_code,omitempty"`
	AtocCodeDescription    string `json:"atoc_code_description,omitempty"`
	ScheduleDaysRuns       string `json:"schedule_days_runs,omitempty"`
	ScheduleEndDate        string `json:"schedule_end_date,omitempty"`
	ScheduleStartDate      string `json:"schedule_start_date,omitempty"`
	TrainStatus            string `json:"train_status,omitempty"`
	TrainStatusDescription string `json:"train_status_description,omitempty"`
	TransactionType        string `json:"transaction_type,omitempty"`

	//NewScheduleSegment section
	TractionClass string `json:"traction_class,omitempty"`
	UicCode       string `json:"uic_code,omitempty"`

	//Schedule Segment section
	SignallingID                           string             `gorm:"index" json:"signalling_id,omitempty"`
	CIFTrainCategory                       string             `json:"CIF_train_category,omitempty"`
	CIFTrainCategoryDescription            string             `json:"CIF_train_category_description,omitempty"`
	CIFHeadcode                            string             `json:"CIF_headcode,omitempty"`
	CIFCourseIndicator                     int                `json:"CIF_course_indicator,omitempty"`
	CIFTrainServiceCode                    string             `json:"CIF_train_service_code,omitempty"`
	CIFBusinessSector                      string             `json:"CIF_business_sector,omitempty"`
	CIFPowerType                           string             `json:"CIF_power_type,omitempty"`
	CIFPowerTypeDescription                string             `json:"CIF_power_type_description,omitempty"`
	CIFTimingLoad                          string             `json:"CIF_timing_load,omitempty"`
	CIFTimingLoadDescription               string             `json:"CIF_timing_load_description,omitempty"`
	CIFSpeed                               string             `json:"CIF_speed,omitempty"`
	CIFOperatingCharacteristics            string             `json:"CIF_operating_characteristics,omitempty"`
	CIFOperatingCharacteristicsDescription string             `json:"CIF_operating_characteristics_description,omitempty"`
	CIFTrainClass                          string             `json:"CIF_train_class,omitempty"`
	CIFSleepers                            string             `json:"CIF_sleepers,omitempty"`
	CIFReservations                        string             `json:"CIF_reservations,omitempty"`
	CIFConnectionIndicator                 string             `json:"CIF_connection_indicator,omitempty"`
	CIFCateringCode                        string             `json:"CIF_catering_code,omitempty"`
	CIFServiceBranding                     string             `json:"CIF_service_branding,omitempty"`
	ScheduleLocation                       []ScheduleLocation `json:"schedule_location,omitempty"`

	// Derived fields
	ScheduleStartDateTS int64 `json:"schedule_end_date_ts"`
	ScheduleEndDateTS   int64 `json:"schedule_from_date_ts"`

	Origin                       string `json:"origin,omitempty"`
	Destination                  string `json:"destination,omitempty"`
	TimeOfDepartureFromOriginTS  int64  `json:"time_of_departure_from_origin_ts"`
	TimeOfArrivalAtDestinationTS int64  `json:"time_of_arrival_at_destination_ts"`
}

type ScheduleLocation struct {
	ID                   int    `gorm:"primaryKey"`
	ScheduleID           uint64 `gorm:"index"`
	LocationType         string `json:"location_type,omitempty"`
	RecordIdentity       string `json:"record_identity,omitempty"`
	TiplocCode           string `gorm:"index" json:"tiploc_code,omitempty"`
	TiplocInstance       string `json:"tiploc_instance,omitempty"`
	Departure            string `json:"departure,omitempty"`
	PublicDeparture      string `json:"public_departure,omitempty"`
	Platform             string `json:"platform,omitempty"`
	Line                 string `json:"line,omitempty"`
	EngineeringAllowance string `json:"engineering_allowance,omitempty"`
	PathingAllowance     string `json:"pathing_allowance,omitempty"`
	PerformanceAllowance string `json:"performance_allowance,omitempty"`
	Arrival              string `json:"arrival,omitempty"`
	PublicArrival        string `json:"public_arrival,omitempty"`
	Pass                 string `json:"pass,omitempty"`
	Path                 string `json:"path,omitempty"`
}

// Define a struct to represent the code-description mapping
type TrainCategoryDescription struct {
	Code        string
	Description string
}

// Define a list of CodeDescription entries
var trainCategoryDescriptions = []TrainCategoryDescription{
	{"OL", "London Underground/Metro Service"},
	{"OU", "Unadvertised Ordinary Passenger"},
	{"OO", "Ordinary Passenger"},
	{"OS", "Staff Train"},
	{"OW", "Mixed"},
	{"XC", "Channel Tunnel"},
	{"XD", "Sleeper (Europe Night Services)"},
	{"XI", "International"},
	{"XR", "Motorail"},
	{"XU", "Unadvertised Express"},
	{"XX", "Express Passenger"},
	{"XZ", "Sleeper (Domestic)"},
	{"BR", "Bus – Replacement due to engineering work"},
	{"BS", "Bus – WTT Service"},
	{"SS", "Ship"},
	{"EE", "Empty Coaching Stock (ECS)"},
	{"EL", "ECS, London Underground/Metro Service"},
	{"ES", "ECS & Staff"},
	{"JJ", "Postal"},
	{"PM", "Post Office Controlled Parcels"},
	{"PP", "Parcels"},
	{"PV", "Empty NPCCS"},
	{"DD", "Departmental"},
	{"DH", "Civil Engineer"},
	{"DI", "Mechanical & Electrical Engineer"},
	{"DQ", "Stores"},
	{"DT", "Test"},
	{"DY", "Signal & Telecommunications Engineer"},
	{"ZB", "Locomotive & Brake Van"},
	{"ZZ", "Light Locomotive"},
	{"J2", "RfD Automotive (Components)"},
	{"H2", "RfD Automotive (Vehicles)"},
	{"J3", "RfD Edible Products (UK Contracts)"},
	{"J4", "RfD Industrial Minerals (UK Contracts)"},
	{"J5", "RfD Chemicals (UK Contracts)"},
	{"J6", "RfD Building Materials (UK Contracts)"},
	{"J8", "RfD General Merchandise (UK Contracts)"},
	{"H8", "RfD European"},
	{"J9", "RfD Freightliner (Contracts)"},
	{"H9", "RfD Freightliner (Other)"},
	{"A0", "Coal (Distributive)"},
	{"E0", "Coal (Electricity) MGR"},
	{"B0", "Coal (Other) and Nuclear"},
	{"B1", "Metals"},
	{"B4", "Aggregates"},
	{"B5", "Domestic and Industrial Waste"},
	{"B6", "Building Materials (TLF)"},
	{"B7", "Petroleum Products"},
	{"H0", "RfD European Channel Tunnel (Mixed Business)"},
	{"H1", "RfD European Channel Tunnel Intermodal"},
	{"H3", "RfD European Channel Tunnel Automotive"},
	{"H4", "RfD European Channel Tunnel Contract Services"},
	{"H5", "RfD European Channel Tunnel Haulmark"},
	{"H6", "RfD European Channel Tunnel Joint Venture"},
}

// Function to get the description for a given code
func GetTrainCategoryDescription(code string) string {
	for _, cd := range trainCategoryDescriptions {
		if cd.Code == code {
			return cd.Description
		}
	}
	return "Description not found"
}

// Define a struct to represent the code-description mapping
type OperatingCharacteristicDescription struct {
	Code        string
	Description string
}

// Define a list of CodeDescription entries
var operatingCharacteristicDescriptions = []OperatingCharacteristicDescription{
	{"B", "Vacuum Braked"},
	{"C", "Timed at 100 m.p.h."},
	{"D", "DOO (Coaching stock trains)"},
	{"E", "Conveys Mark 4 Coaches"},
	{"G", "Trainman (Guard) required"},
	{"M", "Timed at 110 m.p.h."},
	{"P", "Push/Pull train"},
	{"Q", "Runs as required"},
	{"R", "Air conditioned with PA system"},
	{"S", "Steam Heated"},
	{"Y", "Runs to Terminals/Yards as required"},
	{"Z", "May convey traffic to SB1C gauge. Not to be diverted from booked route without authority."},
}

// Function to convert an Operating Characteristic code to its description
func GetOperatingCharacteristicDescription(code string) string {
	var descriptions []string
	for _, cd := range operatingCharacteristicDescriptions {
		if strings.Contains(code, cd.Code) {
			descriptions = append(descriptions, cd.Description)
		}
	}
	return strings.Join(descriptions, ", ")
}

// Define a struct to represent the Power Type code-description mapping
type PowerTypeDescription struct {
	Code        string
	Description string
}

// Define a list of PowerTypeDescription entries
var powerTypeDescriptions = []PowerTypeDescription{
	{"D", "Diesel"},
	{"DEM", "Diesel Electric Multiple Unit"},
	{"DMU", "Diesel Mechanical Multiple Unit"},
	{"E", "Electric"},
	{"ED", "Electro-Diesel"},
	{"EML", "EMU plus D, E, ED locomotive"},
	{"EMU", "Electric Multiple Unit"},
	{"HST", "High Speed Train"},
}

// Function to get the description for a given Power Type code
func GetPowerTypeDescription(code string) string {
	for _, pd := range powerTypeDescriptions {
		if pd.Code == code {
			return pd.Description
		}
	}
	return "Description not found"
}

// Define a struct to represent the Train Status code-description mapping
type TrainStatusDescription struct {
	Code        string
	Description string
}

// Define a list of TrainStatusDescription entries
var trainStatusDescriptions = []TrainStatusDescription{
	{"B", "Bus (Permanent)"},
	{"F", "Freight (Permanent - WTT)"},
	{"P", "Passenger & Parcels (Permanent - WTT)"},
	{"S", "Ship (Permanent)"},
	{"T", "Trip (Permanent)"},
	{"1", "STP Passenger & Parcels"},
	{"2", "STP Freight"},
	{"3", "STP Trip"},
	{"4", "STP Ship"},
	{"5", "STP Bus"},
}

// Function to get the description for a given Train Status code
func GetTrainStatusDescription(code string) string {
	for _, tsd := range trainStatusDescriptions {
		if tsd.Code == code {
			return tsd.Description
		}
	}
	return "Description not found"
}

// Define a struct to represent the Timing Load code-description mapping
type TimingLoadDescription struct {
	PowerTypes  []string
	Code        string
	Description string
}

// Define a list of TimingLoadDescription entries
var timingLoadDescriptions = []TimingLoadDescription{
	{[]string{"DMU"}, "69", "Class 172/0, 172/1 or 172/2"},
	{[]string{"DMU"}, "A", "Class 141 to 144"},
	{[]string{"DMU"}, "E", "Class 158, 168, 170 or 175"},
	{[]string{"DMU"}, "N", "Class 165/0"},
	{[]string{"DMU"}, "S", "Class 150, 153, 155 or 156"},
	{[]string{"DMU"}, "T", "Class 165/1 or 166"},
	{[]string{"DMU"}, "V", "Class 220 or 221"},
	{[]string{"DMU"}, "X", "Class 159"},
	{[]string{"DMU"}, "D1", "DMU (Power Car + Trailer)"},
	{[]string{"DMU"}, "D2", "DMU (2 Power Cars + Trailer)"},
	{[]string{"DMU"}, "D3", "DMU (Power Twin)"},
	{[]string{"EMU"}, "AT", "Accelerated Timings"},
	{[]string{"EMU"}, "E", "Class 458"},
	{[]string{"EMU"}, "0", "Class 380"},
	{[]string{"EMU"}, "506", "Class 350/1 (110 mph)"},
	{[]string{"D", "E", "ED"}, "325 (E)", "Class 325 Electric Parcels Unit"},
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

// Function to get the description for a given Timing Load code
func GetTimingLoadDescription(code string, powerType string) string {

	if code == "" {
		return ""
	}

	//first off deal with the special codes that aren't simple maps
	intCode, err := strconv.Atoi(code)

	if err == nil && (powerType == "D" || powerType == "E" || powerType == "ED" || (powerType == "EMU" && intCode != 0 && intCode != 506)) {
		if powerType == "EMU" {
			return "Class " + code
		}

		return code + " tonnes load"

	}

	for _, tld := range timingLoadDescriptions {
		if contains(tld.PowerTypes, powerType) && tld.Code == code {
			return tld.Description
		}
	}
	return "Description for timing load '" + code + "' not found"
}

// Define a struct to represent each row of the table
type CompanyInfo struct {
	CompanyName  string
	BusinessCode string
	SectorCode   string
	ATOCCode     string
}

// Create a function to map ATOC codes to company names
func GetCompanyNameByATOC(atoc string) string {

	if atoc == "ZZ" {
		return "Non-passenger (operator name obfuscated)"
	}

	// Define a slice of CompanyInfo to store the table data
	companies := []CompanyInfo{
		{"Virtual European Paths", "EU", "?", "EU"},
		{"Alliance Rail", "ZB", "14", "AR"},
		{"Northern Trains", "ED", "23", "NT"},
		{"Transport for Wales", "HL", "71", "AW"},
		{"c2c", "HT", "79", "CC"},
		{"Caledonian Sleeper", "ES", "35", "CS"},
		{"Chiltern Railways", "HO", "74", "CH"},
		{"CrossCountry", "EH", "27", "XC"},
		{"East Midlands Railway", "EM", "28", "EM"},
		{"Eurostar", "GA", "06", "ES"},
		{"Hull Trains", "PF", "55", "HT"},
		{"Govia Thameslink Railway (Great Northern)", "ET", "88", "GN"},
		{"Govia Thameslink Railway (Thameslink)", "ET", "88", "TL"},
		{"Grand Central", "EC", "22", "GC"},
		{"Great Western Railway", "EF", "25", "GW"},
		{"Greater Anglia", "EB", "21", "LE"},
		{"Heathrow Connect", "EE", "24", "HC"},
		{"Heathrow Express", "HM", "86", "HX"},
		{"Island Lines", "HZ", "85", "IL"},
		{"Locomotive Services", "LS", "89", "LS"},
		{"West Midlands Trains", "EJ", "29", "LM"},
		{"London Overground", "EK", "30", "LO"},
		{"LUL Bakerloo Line", "XC", "91", "LT"},
		{"LUL District Line - Richmond", "XE", "93", "LT"},
		{"LUL District Line - Wimbledon", "XB", "90", "LT"},
		{"Merseyrail", "HE", "64", "ME"},
		{"Network Rail (On-Track Machines)", "LR", "15", "LR"},
		{"Nexus (Tyne & Wear Metro)", "PG", "56", "TW"},
		{"North Yorkshire Moors Railway", "PR", "51", "NY"},
		{"ScotRail", "HA", "60", "SR"},
		{"South Western Railway", "HY", "84", "SW"},
		{"South Yorkshire Supertram", "SJ", "19", "SJ"},
		{"Southeastern", "HU", "80", "SE"},
		{"Southern", "HW", "88", "SN"},
		{"Swanage Railway", "SP", "18", "SP"},
		{"Elizabeth line", "EX", "33", "XR"},
		{"TransPennine Express", "EA", "20", "TP"},
		{"Avanti West Coast", "HF", "65", "VT"},
		{"London North Eastern Railway", "HB", "61", "GR"},
		{"West Coast Railways", "PA", "50", "WR"},
		{"Grand Union Trains", "LF", "12", "LF"},
	}

	// Iterate through the slice to find the matching ATOC code
	for _, info := range companies {
		if info.ATOCCode == atoc {
			return info.CompanyName
		}
	}

	// If no match is found, return a default value
	return "Unknown operator code '" + atoc + "'"
}

func (schedule *Schedule) ApplyOverlays(overlays []Schedule, datetime int64) {

	logger.Debug("applying overlays", "count", len(overlays), "overlays", overlays)

	if len(overlays) < 1 {
		return
	}

	//N - STP schedule: similar to a permanent schedule, but planned through the Short Term Planning process and not capable of being overlaid
	if schedule.CIFStpIndicator == "N" {
 		return
	}

	//Sort the overlays by CIFStpIndicator. The first that matches will be applied.
	sort.Slice(overlays, func(i, j int) bool {
		return overlays[i].CIFStpIndicator < overlays[j].CIFStpIndicator
	})

	var overlayApplied bool

	for _, overlay := range overlays {
		if !overlayApplied {
			logger.Debug("Testing overlay...", "ciftrainui", overlay.CIFTrainUID, "startdatets", overlay.ScheduleStartDateTS, "enddatets", overlay.ScheduleEndDateTS, "date", datetime)
			//when to apply the overlay...
			if (overlay.CIFTrainUID == schedule.CIFTrainUID) &&  (overlay.ScheduleStartDateTS <= datetime && overlay.ScheduleEndDateTS > datetime) {

				logger.Debug("Applying overlay", "combinedid", overlay.CombinedID)

				schedule.Source = schedule.Source + "," + overlay.Source

				if overlay.CIFBankHolidayRunning != "" {
					schedule.CIFBankHolidayRunning = overlay.CIFBankHolidayRunning
				}

				if overlay.CIFStpIndicator != "" {
					schedule.CIFStpIndicator = overlay.CIFStpIndicator
				}

				if overlay.ApplicableTimetable != "" {
					schedule.ApplicableTimetable = overlay.ApplicableTimetable
				}

				if overlay.AtocCode != "" {
					schedule.AtocCode = overlay.AtocCode
				}

				if overlay.ScheduleDaysRuns != "" {
					schedule.ScheduleDaysRuns = overlay.ScheduleDaysRuns
				}

				if overlay.ScheduleEndDate != "" {
					schedule.ScheduleEndDate = overlay.ScheduleEndDate
				}

				if overlay.ScheduleStartDate != "" {
					schedule.ScheduleStartDate = overlay.ScheduleStartDate
				}

				if overlay.TrainStatus != "" {
					schedule.TrainStatus = overlay.TrainStatus
				}

				if overlay.TransactionType != "" {
					schedule.TransactionType = overlay.TransactionType
				}

				if overlay.TractionClass != "" {
					schedule.TractionClass = overlay.TractionClass
				}

				if overlay.UicCode != "" {
					schedule.UicCode = overlay.UicCode
				}

				if overlay.SignallingID != "" {
					schedule.SignallingID = overlay.SignallingID
				}

				if overlay.CIFTrainCategory != "" {
					schedule.CIFTrainCategory = overlay.CIFTrainCategory
				}

				if overlay.CIFHeadcode != "" {
					schedule.CIFHeadcode = overlay.CIFHeadcode
				}

				if overlay.CIFCourseIndicator != 0 {
					schedule.CIFCourseIndicator = overlay.CIFCourseIndicator
				}

				if overlay.CIFTrainServiceCode != "" {
					schedule.CIFTrainServiceCode = overlay.CIFTrainServiceCode
				}

				if overlay.CIFBusinessSector != "" {
					schedule.CIFBusinessSector = overlay.CIFBusinessSector
				}

				if overlay.CIFPowerType != "" {
					schedule.CIFPowerType = overlay.CIFPowerType
				}

				if overlay.CIFTimingLoad != "" {
					schedule.CIFTimingLoad = overlay.CIFTimingLoad
				}

				if overlay.CIFSpeed != "" {
					schedule.CIFSpeed = overlay.CIFSpeed
				}

				if overlay.CIFOperatingCharacteristics != "" {
					schedule.CIFOperatingCharacteristics = overlay.CIFOperatingCharacteristics
				}

				if overlay.CIFTrainClass != "" {
					schedule.CIFTrainClass = overlay.CIFTrainClass
				}

				if overlay.CIFSleepers != "" {
					schedule.CIFSleepers = overlay.CIFSleepers
				}

				if overlay.CIFReservations != "" {
					schedule.CIFReservations = overlay.CIFReservations
				}

				if overlay.CIFConnectionIndicator != "" {
					schedule.CIFConnectionIndicator = overlay.CIFConnectionIndicator
				}

				if overlay.CIFCateringCode != "" {
					schedule.CIFCateringCode = overlay.CIFCateringCode
				}

				if overlay.CIFServiceBranding != "" {
					schedule.CIFServiceBranding = overlay.CIFServiceBranding
				}

				schedule.ScheduleLocation = make([]ScheduleLocation, len(overlay.ScheduleLocation))
				copy(schedule.ScheduleLocation, overlay.ScheduleLocation)

				overlayApplied = true
			}
		}
	}

	if overlayApplied {
		err := schedule.AugmentSchedule()
		if err != nil {
			logger.Warn("There was an error augmenting the schedule", "error", err.Error())
		}
	} else {
		logger.Debug("No overlay applied")
	}

}

// Augments the schedule with info
func (schedule *Schedule) AugmentSchedule() error {

	schedule.CIFTrainCategoryDescription = GetTrainCategoryDescription(schedule.CIFTrainCategory)
	schedule.CIFOperatingCharacteristicsDescription = GetOperatingCharacteristicDescription(schedule.CIFOperatingCharacteristics)
	schedule.CIFPowerTypeDescription = GetPowerTypeDescription(schedule.CIFPowerType)
	schedule.TrainStatusDescription = GetTrainStatusDescription(schedule.TrainStatus)
	schedule.CIFTimingLoadDescription = GetTimingLoadDescription(schedule.CIFTimingLoad, schedule.CIFPowerType)
	schedule.AtocCodeDescription = GetCompanyNameByATOC(schedule.AtocCode)

	// A bit of formatting
	schedule.CIFTrainUID = strings.TrimSpace(schedule.CIFTrainUID)

	// Work out the start date of the schedule
	layout := "2006-01-02 15:04:05"
	// Parse the input string into a time.Time object
	ts, err := time.Parse(layout, schedule.ScheduleStartDate+" 00:00:00")
	if err != nil {
		fmt.Println("Failed to parse start date for schedule :", err)
	}
	if err == nil {
		schedule.ScheduleStartDateTS = ts.Unix()
	}

	// Work out the end date of the schedule
	ts, err = time.Parse(layout, schedule.ScheduleEndDate+" 23:59:59")
	if err != nil {
		fmt.Println("Failed to parse end date for schedule :", err)
	}
	if err == nil {
		schedule.ScheduleEndDateTS = ts.Unix()
	}

	schedule.CombinedID = schedule.CIFTrainUID + schedule.ScheduleStartDate + schedule.CIFStpIndicator

	return nil
}
