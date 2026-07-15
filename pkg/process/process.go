package process

type Process struct {
	PID int

	Executable string
	Command    string
	User       string
}
