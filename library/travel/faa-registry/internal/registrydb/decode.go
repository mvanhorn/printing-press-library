// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

package registrydb

import "strings"

// Code tables from the FAA's ardata.pdf (Aircraft Registration Master File
// documentation). Values are stored raw; these decoders translate for output.

var registrantTypes = map[string]string{
	"1": "Individual", "2": "Partnership", "3": "Corporation", "4": "Co-Owned",
	"5": "Government", "7": "LLC", "8": "Non-Citizen Corporation", "9": "Non-Citizen Co-Owned",
}

var aircraftTypes = map[string]string{
	"1": "Glider", "2": "Balloon", "3": "Blimp/Dirigible",
	"4": "Fixed Wing Single-Engine", "5": "Fixed Wing Multi-Engine",
	"6": "Rotorcraft", "7": "Weight-Shift-Control", "8": "Powered Parachute",
	"9": "Gyroplane", "H": "Hybrid Lift", "O": "Other",
}

var engineTypes = map[string]string{
	"0": "None", "1": "Reciprocating", "2": "Turbo-prop", "3": "Turbo-shaft",
	"4": "Turbo-jet", "5": "Turbo-fan", "6": "Ramjet", "7": "2-Cycle",
	"8": "4-Cycle", "9": "Unknown", "10": "Electric", "11": "Rotary",
}

var regions = map[string]string{
	"1": "Eastern", "2": "Southwestern", "3": "Central", "4": "Western-Pacific",
	"5": "Alaskan", "7": "Southern", "8": "European", "C": "Great Lakes",
	"E": "New England", "S": "Northwest Mountain",
}

var statusCodes = map[string]string{
	"A":  "Triennial form mailed, not returned",
	"D":  "Expired Dealer",
	"E":  "Certificate revoked by enforcement action",
	"M":  "Valid — assigned under dealer certificate",
	"N":  "Non-citizen corporation, flight hour reports outstanding",
	"R":  "Registration pending",
	"S":  "Second triennial form mailed, not returned",
	"T":  "Valid registration from a trainee",
	"V":  "Valid Registration",
	"W":  "Certificate deemed ineffective or invalid",
	"X":  "Enforcement letter",
	"Z":  "Permanent reserved",
	"1":  "Triennial form undeliverable",
	"2":  "N-Number assigned, not yet registered",
	"3":  "N-Number assigned (non-type-certificated), not yet registered",
	"4":  "N-Number assigned (import), not yet registered",
	"5":  "Reserved N-Number",
	"6":  "Administratively canceled",
	"7":  "Sale reported",
	"8":  "Second triennial mailing attempt, no response",
	"9":  "Certificate of registration revoked",
	"10": "N-Number assigned, pending cancellation",
	"11": "N-Number assigned (non-type-certificated), pending cancellation",
	"12": "N-Number assigned (import), pending cancellation",
	"13": "Registration expired",
	"14": "First notice for re-registration/renewal",
	"15": "Second notice for re-registration/renewal",
	"16": "Registration expired, pending cancellation",
	"17": "Sale reported, pending cancellation",
	"18": "Sale reported, canceled",
	"19": "Registration pending, pending cancellation",
	"20": "Registration pending, canceled",
	"21": "Revoked, pending cancellation",
	"22": "Revoked, canceled",
	"23": "Expired dealer, pending cancellation",
	"24": "Third notice for re-registration/renewal",
	"25": "First notice for renewal",
	"26": "Second notice for renewal",
	"27": "Registration expired",
	"28": "Third notice for renewal",
	"29": "Registration expired, pending cancellation",
}

var airworthinessClasses = map[string]string{
	"1": "Standard", "2": "Limited", "3": "Restricted", "4": "Experimental",
	"5": "Provisional", "6": "Multiple", "7": "Primary", "8": "Special Flight Permit",
	"9": "Light Sport",
}

func decode(m map[string]string, code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	if v, ok := m[code]; ok {
		return v
	}
	return code
}

// DecodeRegistrantType translates a TYPE REGISTRANT code.
func DecodeRegistrantType(code string) string { return decode(registrantTypes, code) }

// DecodeAircraftType translates a TYPE AIRCRAFT code.
func DecodeAircraftType(code string) string { return decode(aircraftTypes, code) }

// DecodeEngineType translates a TYPE ENGINE code.
func DecodeEngineType(code string) string { return decode(engineTypes, code) }

// DecodeRegion translates a REGION code.
func DecodeRegion(code string) string { return decode(regions, code) }

// DecodeStatus translates a STATUS CODE.
func DecodeStatus(code string) string { return decode(statusCodes, code) }

// DecodeAirworthinessClass translates the first CERTIFICATION character.
func DecodeAirworthinessClass(cert string) string {
	cert = strings.TrimSpace(cert)
	if cert == "" {
		return ""
	}
	return decode(airworthinessClasses, cert[:1])
}

// EngineClass groups an engine-type code into jet / turboprop / piston /
// helicopter-turbine / electric / other buckets for fleet reporting.
func EngineClass(typeEngine string) string {
	switch strings.TrimSpace(typeEngine) {
	case "4", "5", "6":
		return "jet"
	case "2":
		return "turboprop"
	case "3":
		return "turbine (shaft)"
	case "1", "7", "8", "11":
		return "piston"
	case "10":
		return "electric"
	case "0":
		return "none"
	default:
		return "other/unknown"
	}
}
