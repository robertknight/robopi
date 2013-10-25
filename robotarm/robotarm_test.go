package robotarm

import (
	"testing"
	"time"
)

func TestArm(t *testing.T) {
	arm, err := Open()
	if err != nil {
		msg := `
Connecting to robot arm failed: %v

On Linux, you may need to set up appropriate
permissions to connect to the arm without root privileges.

See the libusb FAQ
`
		t.Errorf(msg, err)
		return
	}
	moves := []Move{
		{GripOpen, 1 * time.Second},
		{GripClose, 1 * time.Second},
		{Reset, 1 * time.Second},
	}
	err = arm.Move(moves)
	if err != nil {
		t.Errorf("Unable to operate the robot arm: %v", err)
	}
	arm.Close()
}
