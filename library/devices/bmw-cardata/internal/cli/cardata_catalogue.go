// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/store"
)

// catalogueEntry is one row of the seeded VSS telematic data catalogue.
type catalogueEntry struct {
	Descriptor  string
	Unit        string
	Domain      string
	Description string
}

// cardataDescriptorCatalogue is a curated subset of BMW CarData's VSS
// telematic data catalogue (the TDC). It lets `descriptors search` work
// offline and guides container authoring. Drawn from BMW's integration
// guide and the kvanbiesen/bmw-cardata-ha reference descriptors.
var cardataDescriptorCatalogue = []catalogueEntry{
	// HV battery / state of charge
	{"vehicle.drivetrain.batteryManagement.header", "%", "battery", "Current high-voltage battery state of charge (header value)."},
	{"vehicle.powertrain.electric.battery.stateOfCharge.displayed", "%", "battery", "State of charge as displayed to the driver."},
	{"vehicle.powertrain.electric.battery.stateOfCharge.target", "%", "battery", "Target state of charge for the next charge."},
	{"vehicle.powertrain.electric.battery.stateOfHealth.displayed", "%", "battery", "Battery state of health (degradation indicator)."},
	{"vehicle.drivetrain.batteryManagement.maxEnergy", "kWh", "battery", "Maximum usable energy currently stored in the HV battery."},
	{"vehicle.drivetrain.batteryManagement.batterySizeMax", "kWh", "battery", "Nameplate maximum battery capacity."},
	// Charging
	{"vehicle.drivetrain.electricEngine.charging.status", "", "charging", "Current charging status."},
	{"vehicle.drivetrain.electricEngine.charging.level", "", "charging", "Charging level (AC/DC mapping)."},
	{"vehicle.drivetrain.electricEngine.charging.power", "kW", "charging", "Current charging power."},
	{"vehicle.drivetrain.electricEngine.charging.acVoltage", "V", "charging", "AC charging voltage."},
	{"vehicle.drivetrain.electricEngine.charging.acAmpere", "A", "charging", "AC charging current."},
	{"vehicle.drivetrain.electricEngine.charging.phaseNumber", "", "charging", "Number of AC charging phases."},
	{"vehicle.drivetrain.electricEngine.charging.method", "", "charging", "Charging method (AC/DC)."},
	{"vehicle.drivetrain.electricEngine.charging.timeToFullyCharged", "minutes", "charging", "Time remaining until fully charged."},
	{"vehicle.drivetrain.electricEngine.charging.timeRemaining", "minutes", "charging", "Remaining charging time."},
	{"vehicle.drivetrain.electricEngine.charging.hvStatus", "", "charging", "HV charging readiness status."},
	{"vehicle.drivetrain.electricEngine.charging.lastChargingReason", "", "charging", "Reason the last charging session was initiated."},
	{"vehicle.drivetrain.electricEngine.charging.lastChargingResult", "", "charging", "Outcome of the last charging session."},
	{"vehicle.drivetrain.electricEngine.charging.reasonChargingEnd", "", "charging", "Reason the last charging session ended."},
	{"vehicle.powertrain.electric.battery.charging.power", "kW", "charging", "Battery charging power."},
	{"vehicle.powertrain.electric.battery.charging.acLimit.selected", "A", "charging", "Selected AC charging current limit."},
	{"vehicle.drivetrain.electricEngine.remainingElectricRange", "km", "range", "Remaining purely electric range."},
	// Range / mileage
	{"vehicle.vehicle.travelledDistance", "km", "range", "Total distance travelled."},
	{"vehicle.drivetrain.fuelSystem.remainingFuel", "l", "range", "Remaining fuel."},
	{"vehicle.drivetrain.fuelSystem.level", "%", "range", "Fuel tank level."},
	// Location
	{"vehicle.cabin.infotainment.navigation.currentLocation.latitude", "degrees", "location", "Current vehicle latitude."},
	{"vehicle.cabin.infotainment.navigation.currentLocation.longitude", "degrees", "location", "Current vehicle longitude."},
	{"vehicle.cabin.infotainment.navigation.currentLocation.heading", "degrees", "location", "Current vehicle heading."},
	{"vehicle.cabin.infotainment.navigation.currentLocation.altitude", "m", "location", "Current vehicle altitude."},
	// Body / windows / locks
	{"vehicle.cabin.window.row1.driver.status", "", "body", "Driver window status."},
	{"vehicle.cabin.window.row1.passenger.status", "", "body", "Front passenger window status."},
	{"vehicle.cabin.window.row2.driver.status", "", "body", "Rear driver window status."},
	{"vehicle.cabin.window.row2.passenger.status", "", "body", "Rear passenger window status."},
	{"vehicle.body.trunk.window.isOpen", "", "body", "Whether the trunk window is open."},
	{"vehicle.body.chargingPort.lockedStatus", "", "body", "Charging port lock status."},
	{"vehicle.body.chargingPort.plugEventId", "", "body", "Charging port plug event id."},
	{"vehicle.powertrain.tractionBattery.charging.port.anyPosition.flap.isOpen", "", "body", "Charging flap open status."},
	{"vehicle.powertrain.tractionBattery.charging.port.anyPosition.isPlugged", "", "body", "Charging cable plugged status."},
	// Thermal / preconditioning
	{"vehicle.powertrain.electric.battery.preconditioning.automaticMode.statusFeedback", "", "thermal", "Automatic battery preconditioning feedback."},
	{"vehicle.powertrain.electric.battery.preconditioning.manualMode.statusFeedback", "", "thermal", "Manual battery preconditioning feedback."},
	// Trip / efficiency
	{"vehicle.trip.segment.end.drivetrain.batteryManagement.hvSoc", "%", "trip", "HV state of charge at the end of the last trip segment."},
	{"vehicle.trip.segment.accumulated.drivetrain.electricEngine.recuperationTotal", "kWh", "trip", "Accumulated recuperated energy."},
	{"vehicle.vehicle.avgAuxPower", "kW", "trip", "Average auxiliary power consumption."},
	{"vehicle.vehicleIdentification.basicVehicleData", "", "vehicle", "Basic vehicle identification data."},
	// Derived/learned (community)
	{"vehicle.predicted_soc", "%", "predicted", "Predicted state of charge (learned model)."},
	{"vehicle.magic_soc", "%", "predicted", "Magic state of charge (adjusted usable SoC)."},
}

// seedCardataCatalogue populates the descriptor catalogue table if empty.
// Idempotent and best-effort.
func seedCardataCatalogue(db *store.Store) error {
	var n int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM cardata_descriptor_catalogue`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	tx, err := db.DB().BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(context.Background(),
		`INSERT OR IGNORE INTO cardata_descriptor_catalogue(descriptor, unit, domain, description) VALUES(?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, e := range cardataDescriptorCatalogue {
		if _, err := stmt.ExecContext(context.Background(), e.Descriptor, e.Unit, e.Domain, e.Description); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "seeded %d descriptors into local catalogue\n", len(cardataDescriptorCatalogue))
	return nil
}
