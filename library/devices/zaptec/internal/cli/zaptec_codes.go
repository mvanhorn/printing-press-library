// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.

package cli

// Baked decode tables from the Zaptec /api/constants endpoint. These rarely
// change; baking them in lets state/control commands speak plain English
// (decoded operation modes, named observations, named commands) offline,
// instead of forcing the user to deal with magic numbers.

// Command IDs accepted by POST /api/chargers/{id}/sendCommand/{commandId}.
const (
	cmdRestartCharger  = 102
	cmdUpgradeFirmware = 200
	cmdStartCharging   = 501
	cmdStopCharging    = 502 // pausable stop
	cmdStopFinal       = 506 // StopChargingFinal — ends the session
	cmdResumeCharging  = 507
	cmdUnlockConnector = 708
	cmdDeauthorize     = 10001 // DeauthorizeAndStop
)

// chargerOperationModeName decodes the ChargerOperationMode enum (observation
// 710 and the chargers.operating_mode column).
func chargerOperationModeName(mode int) string {
	switch mode {
	case 0:
		return "Unknown"
	case 1:
		return "Disconnected"
	case 2:
		return "Connected (requesting)"
	case 3:
		return "Charging"
	case 5:
		return "Connected (finished)"
	default:
		return "Unknown"
	}
}

// observationMeta maps an observation StateId to a human name and unit. Only the
// observations a user is likely to care about are decoded; unknown IDs fall
// through to a generic label.
type observationMeta struct {
	Name string
	Unit string
}

var observationTable = map[int]observationMeta{
	1:    {"OfflineMode", ""},
	110:  {"Product name", ""},
	120:  {"Authentication required", ""},
	151:  {"Permanent cable lock", ""},
	153:  {"HMI brightness", ""},
	201:  {"Temperature (internal)", "°C"},
	205:  {"Temperature (power board)", "°C"},
	270:  {"Humidity", "%"},
	501:  {"Voltage phase 1", "V"},
	502:  {"Voltage phase 2", "V"},
	503:  {"Voltage phase 3", "V"},
	507:  {"Current phase 1", "A"},
	508:  {"Current phase 2", "A"},
	509:  {"Current phase 3", "A"},
	510:  {"Charger max current", "A"},
	513:  {"Total charge power", "W"},
	520:  {"Max phases", ""},
	546:  {"Installation max current limit", "A"},
	553:  {"Session energy", "kWh"},
	702:  {"Charge mode", ""},
	710:  {"Operation mode", ""},
	716:  {"Detected car", ""},
	723:  {"Completed session", ""},
	1121: {"Energy level", "%"},
}

func observationName(id int) string {
	if m, ok := observationTable[id]; ok {
		return m.Name
	}
	return ""
}

func observationUnit(id int) string {
	if m, ok := observationTable[id]; ok {
		return m.Unit
	}
	return ""
}

// observationsOfInterest is the ordered subset shown in the default (non-verbose)
// state view — the readings a charger owner checks at a glance.
var observationsOfInterest = []int{710, 716, 513, 553, 507, 508, 509, 501, 502, 503, 510, 546, 201}
