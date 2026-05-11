package omnilogic

import "time"

type Site struct {
	MspSystemID  int    `json:"msp_system_id" xml:"msp_system_id"`
	BackyardName string `json:"backyard_name" xml:"backyard_name"`
}

type Alarm struct {
	EquipmentID string            `json:"equipment_id"`
	BowID       string            `json:"bow_id,omitempty"`
	Code        string            `json:"code,omitempty"`
	Severity    string            `json:"severity,omitempty"`
	Message     string            `json:"message,omitempty"`
	FirstSeen   string            `json:"first_seen,omitempty"`
	Raw         map[string]string `json:"raw,omitempty"`
}

type SiteAlarms struct {
	MspSystemID  int     `json:"msp_system_id"`
	BackyardName string  `json:"backyard_name"`
	Alarms       []Alarm `json:"alarms"`
}

type BodyOfWater struct {
	SystemID          string      `json:"system_id"`
	Name              string      `json:"name"`
	Type              string      `json:"type,omitempty"`
	SharedType        string      `json:"shared_type,omitempty"`
	SharedEquipID     string      `json:"shared_equipment_id,omitempty"`
	SupportsSpillover string      `json:"supports_spillover,omitempty"`
	Pumps             []Equipment `json:"pumps,omitempty"`
	Heaters           []Heater    `json:"heaters,omitempty"`
	Lights            []Equipment `json:"lights,omitempty"`
	Relays            []Equipment `json:"relays,omitempty"`
	Chlorinator       *Equipment  `json:"chlorinator,omitempty"`
	Filter            *Equipment  `json:"filter,omitempty"`
	CSAD              *Equipment  `json:"csad,omitempty"`
}

type Equipment struct {
	SystemID string            `json:"system_id"`
	Name     string            `json:"name"`
	Type     string            `json:"type,omitempty"`
	Function string            `json:"function,omitempty"`
	V2Active string            `json:"v2_active,omitempty"`
	MinSpeed string            `json:"min_speed,omitempty"`
	MaxSpeed string            `json:"max_speed,omitempty"`
	CellType string            `json:"cell_type,omitempty"`
	Attrs    map[string]string `json:"attrs,omitempty"`
}

type Heater struct {
	Name                 string `json:"name"`
	SystemID             string `json:"system_id"`
	Enabled              string `json:"enabled,omitempty"`
	CurrentSetPoint      string `json:"current_set_point,omitempty"`
	MaxWaterTemp         string `json:"max_water_temp,omitempty"`
	MinSettableWaterTemp string `json:"min_settable_water_temp,omitempty"`
	MaxSettableWaterTemp string `json:"max_settable_water_temp,omitempty"`
	SharedType           string `json:"shared_type,omitempty"`
	HeaterType           string `json:"heater_type,omitempty"`
}

type MspConfig struct {
	MspSystemID   int           `json:"msp_system_id"`
	BackyardName  string        `json:"backyard_name"`
	BodiesOfWater []BodyOfWater `json:"bodies_of_water"`
	Relays        []Equipment   `json:"relays"`
	RawXML        string        `json:"-"`
	FetchedAt     time.Time     `json:"fetched_at"`
}

type Telemetry struct {
	MspSystemID   int                       `json:"msp_system_id"`
	BackyardName  string                    `json:"backyard_name,omitempty"`
	AirTemp       *int                      `json:"air_temp,omitempty"`
	Status        string                    `json:"status,omitempty"`
	BodiesOfWater []TelemetryBOW            `json:"bodies_of_water"`
	Relays        []TelemetryEquipmentState `json:"relays,omitempty"`
	SampledAt     time.Time                 `json:"sampled_at"`
	RawXML        string                    `json:"-"`
}

type TelemetryBOW struct {
	SystemID       string                    `json:"system_id"`
	Name           string                    `json:"name"`
	WaterTemp      *int                      `json:"water_temp,omitempty"`
	PH             *float64                  `json:"ph,omitempty"`
	ORP            *int                      `json:"orp,omitempty"`
	SaltPPM        *int                      `json:"salt_ppm,omitempty"`
	ChlorOutputPct *int                      `json:"chlor_output_pct,omitempty"`
	Pumps          []TelemetryEquipmentState `json:"pumps,omitempty"`
	Heaters        []TelemetryEquipmentState `json:"heaters,omitempty"`
	Lights         []TelemetryEquipmentState `json:"lights,omitempty"`
	Relays         []TelemetryEquipmentState `json:"relays,omitempty"`
	Attrs          map[string]string         `json:"attrs,omitempty"`
}

type TelemetryEquipmentState struct {
	SystemID   string            `json:"system_id"`
	Name       string            `json:"name,omitempty"`
	IsOn       *bool             `json:"is_on,omitempty"`
	Speed      *int              `json:"speed,omitempty"`
	SpeedPct   *int              `json:"speed_pct,omitempty"`
	ShowID     *int              `json:"show_id,omitempty"`
	Brightness *int              `json:"brightness,omitempty"`
	Enabled    *bool             `json:"enabled,omitempty"`
	SetPoint   *int              `json:"set_point,omitempty"`
	Attrs      map[string]string `json:"attrs,omitempty"`
}

type Chemistry struct {
	MspSystemID int       `json:"msp_system_id"`
	BowName     string    `json:"bow_name"`
	PH          *float64  `json:"ph,omitempty"`
	ORP         *int      `json:"orp,omitempty"`
	SaltPPM     *int      `json:"salt_ppm,omitempty"`
	WaterTemp   *int      `json:"water_temp,omitempty"`
	AirTemp     *int      `json:"air_temp,omitempty"`
	Verdict     string    `json:"verdict"`
	Reasons     []string  `json:"reasons,omitempty"`
	// NotEquipped names the sensors this site doesn't have (per capabilities
	// config). When populated, those readings are omitted from PH/ORP/SaltPPM
	// even if Hayward returned a -1, and the verdict logic ignores them.
	NotEquipped []string `json:"not_equipped,omitempty"`
	// TempState captures the water-temperature reading's context for
	// installs where the sensor only reads while the pump is running.
	// "ok" = sensor is reporting; "n/a-pump-off" = sensor expected to be
	// silent because the pump is idle; "offline" = pump is running but
	// sensor still returned -1 (real alarm).
	TempState string    `json:"temp_state,omitempty"`
	SampledAt time.Time `json:"sampled_at"`
}

type CommandResult struct {
	Status     string `json:"status"`
	Operation  string `json:"operation"`
	Target     string `json:"target,omitempty"`
	Detail     string `json:"detail,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
}

type LightShow struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	V2Only bool   `json:"v2_only"`
}
