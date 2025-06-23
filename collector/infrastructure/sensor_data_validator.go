package infrastructure

import (
	"fmt"
	"time"

	collectorDomain "github.com/samoilenko/cossack_labs/collector/domain"
	gen "github.com/samoilenko/cossack_labs/pkg/sensorpb/v1"
)

// SensorDataValidator implements validation rules for sensor data messages.
type SensorDataValidator struct {
}

// Apply validates a SensorData message against defined business rules.
func (r SensorDataValidator) Apply(msg *gen.SensorData) error {
	if msg.SensorName == "" {
		return collectorDomain.ErrValidation
	}
	if len([]rune(msg.SensorName)) > 10 {
		return fmt.Errorf("%w: sensor name is too long: %d chars", collectorDomain.ErrValidation, len(msg.SensorName))
	}

	if msg.Timestamp == nil {
		return fmt.Errorf("%w: timestamp is absent%s", collectorDomain.ErrValidation, msg.Timestamp)
	}

	if msg.Timestamp.AsTime().Before(time.Now().Add(-1 * time.Hour)) {
		return fmt.Errorf("%w: timestamp too old: %s", collectorDomain.ErrValidation, msg.Timestamp.AsTime().Format(time.RFC3339))
	}

	return nil
}

// NewSensorDataValidator creates a new instance of SensorDataValidator.
func NewSensorDataValidator() *SensorDataValidator {
	return &SensorDataValidator{}
}
