package daemon

import "time"

const defaultCompactorDogInterval = 24 * time.Hour

// CompactorDogConfig holds configuration for the compactor_dog patrol.
type CompactorDogConfig struct {
	Enabled     bool   `json:"enabled"`
	IntervalStr string `json:"interval,omitempty"`
}

// compactorDogInterval returns the configured interval, or the default (24h).
func compactorDogInterval(config *DaemonPatrolConfig) time.Duration {
	if config != nil && config.Patrols != nil && config.Patrols.CompactorDog != nil {
		if config.Patrols.CompactorDog.IntervalStr != "" {
			if d, err := time.ParseDuration(config.Patrols.CompactorDog.IntervalStr); err == nil && d > 0 {
				return d
			}
		}
	}
	return defaultCompactorDogInterval
}

// runCompactorDog pours a compactor molecule for agent execution.
// The formula (mol-dog-compactor) describes the flatten steps declaratively.
// An agent interprets and executes them — no imperative Go logic here.
func (d *Daemon) runCompactorDog() {
	if !IsPatrolEnabled(d.patrolConfig, "compactor_dog") {
		return
	}
	d.logger.Printf("compactor_dog: pouring molecule")
	mol := d.pourDogMolecule("mol-dog-compactor", nil)
	defer mol.close()
}
