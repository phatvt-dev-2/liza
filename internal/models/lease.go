package models

import "time"

const (
	// LeaseExpiryGracePeriod is the grace window after lease expiry before alerting/warning.
	LeaseExpiryGracePeriod = 120 * time.Second
)
