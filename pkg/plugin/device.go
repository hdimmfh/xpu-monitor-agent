package plugin

type DeviceType string

const (
	DeviceTypeGPU  DeviceType = "gpu"
	DeviceTypeXPU  DeviceType = "xpu"
	DeviceTypeNPU  DeviceType = "npu"
	DeviceTypeDPU  DeviceType = "dpu"
	DeviceTypeFPGA DeviceType = "fpga"
	DeviceTypeASIC DeviceType = "asic"
)

type Device struct {
	ID     string
	Vendor string
	Model  string
	Type   DeviceType
}
