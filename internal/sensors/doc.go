// Package sensors holds the sensor/ingest adapters that turn external inputs
// (simulated feeds, weather, drone/satellite imagery, human operators) into
// events on the bus (SPEC §10, §16.1).
//
// Owner: sensors Builder.
// Depends on: contracts/{events,interfaces}.
// Must NOT: bypass validation — every ingested event still passes the §14.2
// gatekeeper in internal/state.
package sensors
