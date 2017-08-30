package model

// Pin identifies a connection pin of a hardware device.
type Pin struct {
	// Unique identifier of the device that contains this pin.
	DeviceID string `json:"device"`
	// Pin number (1...)
	Pin int `json:"pin"`
}
