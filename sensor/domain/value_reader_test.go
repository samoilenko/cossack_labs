package domain

import (
	"context"
	"testing"
	"time"
)

type mockLogger struct{}

func (m *mockLogger) Info(_ string, _ ...interface{})  {}
func (m *mockLogger) Error(_ string, _ ...interface{}) {}

type mockSensor struct {
	value int32
}

func (m *mockSensor) GetValue() (int32, error) {
	return m.value, nil
}

func TestValueReader_Read(t *testing.T) {
	t.Run("reads values from sensor at a given rate", func(t *testing.T) {
		sensor := &mockSensor{value: 42}
		reader := NewValueReader(Rate(10), 2, &mockLogger{})

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond) // +- 5 reads per 500ms
		defer cancel()

		valueCh := reader.Read(ctx, sensor)

		var receivedValues []*SensorValue
		for val := range valueCh {
			receivedValues = append(receivedValues, val)
		}

		if len(receivedValues) < 4 {
			t.Errorf("expected more or equal 4 values, but got %d", len(receivedValues))
		}

		for _, sensorData := range receivedValues {
			if sensorData.Value != 42 {
				t.Errorf("expected value 42, but got %d", sensorData.Value)
			}
		}
	})

	t.Run("stops reading when context is cancelled", func(t *testing.T) {
		sensor := &mockSensor{value: 42}
		reader := NewValueReader(Rate(1000), 2, &mockLogger{})

		ctx, cancel := context.WithCancel(context.Background())
		valueCh := reader.Read(ctx, sensor)
		time.Sleep(20 * time.Millisecond)

		cancel()

		timeout := time.After(100 * time.Millisecond)
		for {
			select {
			case _, ok := <-valueCh:
				if !ok {
					return
				}
			case <-timeout:
				t.Fatal("timed out waiting for channel to close")
			}
		}
	})
}
