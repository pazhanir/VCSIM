// Per-host hardware health sensors.
//
// Real ESXi hosts expose IPMI/CIM numeric sensors via
// HostRuntimeInfo.HealthSystemRuntime.SystemHealthInfo.NumericSensorInfo.
// Stock vcsim only ships a handful of "Processors" / "Software Components"
// sensors, so the Site24x7 ESX Hardware dashboard shows 0/0 for every
// category (Fan, Temperature, Power, Voltage, Battery, ...).
//
// The Site24x7 poller buckets sensors by HostNumericSensorInfo.SensorType
// (lowercased) against these exact strings (ESXHardwareMetricFormatter.java):
//
//	power, fan, temperature, voltage, memory, battery, watchdog,
//	storage, systemboard, bios, cable, processor, other
//
// BuildSensors generates a realistic, healthy sensor set using those exact
// type strings, scaled per host (e.g. one temperature/voltage sensor per CPU
// socket), so the dashboard populates with non-zero monitored counts.
package inventory

import (
	"fmt"

	"github.com/vmware/govmomi/vim25/types"
)

// greenSensor builds a single healthy (green) numeric sensor.
func greenSensor(name, sensorType string, reading int64, baseUnits string) types.HostNumericSensorInfo {
	return types.HostNumericSensorInfo{
		Name:       name,
		SensorType: sensorType,
		HealthState: &types.ElementDescription{
			Description: types.Description{
				Label:   "Green",
				Summary: "Sensor is operating under normal conditions",
			},
			Key: "green",
		},
		CurrentReading: reading,
		UnitModifier:   0,
		BaseUnits:      baseUnits,
		RateUnits:      "none",
	}
}

// BuildSensors returns a realistic set of healthy hardware sensors for a host,
// scaled to the given profile. SensorType strings match the Site24x7 poller's
// category buckets exactly so each dashboard category reports a non-zero count.
func BuildSensors(p *HardwareProfile) []types.HostNumericSensorInfo {
	sockets := int(p.NumCPUPkgs)
	if sockets < 1 {
		sockets = 1
	}

	var s []types.HostNumericSensorInfo

	// system rollup (kept from stock behaviour; bucketed as "other")
	s = append(s, greenSensor("VMware Rollup Health State", "system", 0, ""))

	// processor — one per socket
	for i := 0; i < sockets; i++ {
		s = append(s, greenSensor(fmt.Sprintf("CPU%d Status", i), "processor", 0, ""))
	}

	// temperature — ambient + per-socket + system board
	s = append(s, greenSensor("System Ambient Temp", "temperature", 22, "degrees C"))
	for i := 0; i < sockets; i++ {
		s = append(s, greenSensor(fmt.Sprintf("CPU%d Temp", i), "temperature", 55, "degrees C"))
	}
	s = append(s, greenSensor("System Board Temp", "temperature", 30, "degrees C"))

	// fan — typical chassis fan bank
	for i := 1; i <= 6; i++ {
		s = append(s, greenSensor(fmt.Sprintf("Fan%d", i), "fan", 6000, "RPM"))
	}

	// power — dual redundant PSUs (realistic draw) + total draw.
	// Use meaningful wattage readings; a "1 W" PSU reading can be filtered out
	// as degenerate by downstream consumers.
	s = append(s, greenSensor("Power Supply 1", "power", 210, "Watts"))
	s = append(s, greenSensor("Power Supply 2", "power", 205, "Watts"))
	s = append(s, greenSensor("System Power Consumption", "power", 415, "Watts"))
	s = append(s, greenSensor("Power Supply 1 Status", "power", 0, ""))
	s = append(s, greenSensor("Power Supply 2 Status", "power", 0, ""))

	// voltage — common rails + per-socket Vcore
	s = append(s, greenSensor("Planar 3.3V", "voltage", 3, "Volts"))
	s = append(s, greenSensor("Planar 5V", "voltage", 5, "Volts"))
	s = append(s, greenSensor("Planar 12V", "voltage", 12, "Volts"))
	for i := 0; i < sockets; i++ {
		s = append(s, greenSensor(fmt.Sprintf("CPU%d Vcore", i), "voltage", 1, "Volts"))
	}

	// memory — DIMM presence/health (scale loosely with capacity)
	dimms := int(p.MemoryBytes / (32 * 1024 * 1024 * 1024)) // ~one sensor per 32GB
	if dimms < 4 {
		dimms = 4
	}
	if dimms > 24 {
		dimms = 24
	}
	for i := 0; i < dimms; i++ {
		s = append(s, greenSensor(fmt.Sprintf("DIMM %d Status", i), "memory", 0, ""))
	}

	// battery — CMOS / RAID cache battery
	s = append(s, greenSensor("CMOS Battery", "battery", 1, "Volts"))
	s = append(s, greenSensor("ROMB Battery", "battery", 1, ""))

	// storage — controller + drive bay health
	s = append(s, greenSensor("Storage Controller 0", "storage", 0, ""))
	s = append(s, greenSensor("Drive Bay Status", "storage", 0, ""))

	// systemboard — chassis/board health
	s = append(s, greenSensor("System Board Status", "systemboard", 0, ""))
	s = append(s, greenSensor("Chassis Intrusion", "systemboard", 0, ""))

	// bios — POST/firmware health
	s = append(s, greenSensor("BIOS POST Status", "bios", 0, ""))

	// cable — interconnect/SAS cable presence
	s = append(s, greenSensor("SAS Cable A", "cable", 0, ""))

	// watchdog — IPMI watchdog timer
	s = append(s, greenSensor("IPMI Watchdog", "watchdog", 0, ""))

	return s
}

// ApplySensors installs the generated sensor set on the host's runtime health
// system, creating the HealthSystemRuntime/SystemHealthInfo structs if needed.
func ApplySensors(runtime *types.HostRuntimeInfo, p *HardwareProfile) {
	if runtime == nil || p == nil {
		return
	}
	if runtime.HealthSystemRuntime == nil {
		runtime.HealthSystemRuntime = &types.HealthSystemRuntime{}
	}
	if runtime.HealthSystemRuntime.SystemHealthInfo == nil {
		runtime.HealthSystemRuntime.SystemHealthInfo = &types.HostSystemHealthInfo{}
	}
	runtime.HealthSystemRuntime.SystemHealthInfo.NumericSensorInfo = BuildSensors(p)
}
