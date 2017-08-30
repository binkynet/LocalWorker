package devices

// Service contains the API that is exposed by the device service.
type Service interface {
	// DeviceByID returns the device with given ID.
	// Return false if not found
	DeviceByID(id string) (Device, bool)
}
