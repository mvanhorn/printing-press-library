// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

// cardataContainerTemplates maps a template name to a curated container
// definition (name, purpose, VSS descriptors). Used by
// `customers create-container --template <name>` to one-shot a sensible
// descriptor bundle without hand-listing VSS paths.
var cardataContainerTemplates = map[string]cardataContainerTemplate{
	"hv-battery": {
		Name:    "BMW CarData HV Battery",
		Purpose: "High voltage battery telemetry",
		Descriptors: []string{
			"vehicle.drivetrain.batteryManagement.header",
			"vehicle.powertrain.electric.battery.stateOfCharge.displayed",
			"vehicle.drivetrain.batteryManagement.maxEnergy",
			"vehicle.powertrain.electric.battery.charging.power",
			"vehicle.drivetrain.electricEngine.charging.power",
			"vehicle.drivetrain.electricEngine.charging.status",
			"vehicle.drivetrain.electricEngine.charging.level",
			"vehicle.drivetrain.electricEngine.charging.method",
			"vehicle.drivetrain.electricEngine.charging.timeToFullyCharged",
			"vehicle.drivetrain.electricEngine.charging.timeRemaining",
			"vehicle.drivetrain.electricEngine.charging.acVoltage",
			"vehicle.drivetrain.electricEngine.charging.acAmpere",
			"vehicle.drivetrain.electricEngine.charging.phaseNumber",
			"vehicle.drivetrain.electricEngine.remainingElectricRange",
			"vehicle.powertrain.electric.battery.stateOfCharge.target",
			"vehicle.powertrain.electric.battery.stateOfHealth.displayed",
			"vehicle.drivetrain.batteryManagement.batterySizeMax",
			"vehicle.body.chargingPort.plugEventId",
			"vehicle.body.chargingPort.lockedStatus",
			"vehicle.powertrain.tractionBattery.charging.port.anyPosition.isPlugged",
		},
	},
}

type cardataContainerTemplate struct {
	Name        string
	Purpose     string
	Descriptors []string
}
