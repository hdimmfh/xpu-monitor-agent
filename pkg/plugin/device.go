package plugin

type DeviceType string

const (
	DeviceTypeHost DeviceType = "host"
	DeviceTypeGPU  DeviceType = "gpu"
	DeviceTypeXPU  DeviceType = "xpu"
	DeviceTypeNPU  DeviceType = "npu"
	DeviceTypeDPU  DeviceType = "dpu"
	DeviceTypeFPGA DeviceType = "fpga"
	DeviceTypeASIC DeviceType = "asic"
)

type Device struct {
	ID     string     `json:"id"`
	Vendor string     `json:"vendor"`
	Model  string     `json:"model"`
	Type   DeviceType `json:"type"`
}
