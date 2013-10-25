package robotarm

import (
	"errors"
	"fmt"
	usb "github.com/robertknight/libusb"
	"time"
)

const MaplinRobotArmVendorId = 0x1267
const MaplinRobotArmProductId = 0

type MoveType []byte

var Reset = MoveType{0, 0, 0}
var GripOpen = MoveType{0x02, 0, 0}
var GripClose = MoveType{0x01, 0, 0}
var BaseLeft = MoveType{0, 0x01, 0}
var BaseRight = MoveType{0, 0x02, 0}
var WristUp = MoveType{0x04, 0, 0}
var WristDown = MoveType{0x08, 0, 0}
var ElbowUp = MoveType{0x10, 0, 0}
var ElbowDown = MoveType{0x20, 0, 0}
var ShoulderUp = MoveType{0x40, 0, 0}
var ShoulderDown = MoveType{0x80, 0, 0}

type Move struct {
	Move     MoveType
	Duration time.Duration
}

type Arm struct {
	device *usb.Device
}

func Open() (Arm, error) {
	usb.Init()
	var arm Arm
	var err error
	arm.device = usb.Open(MaplinRobotArmVendorId, MaplinRobotArmProductId)
	if arm.device == nil {
		err = errors.New("Unable to connect to robot arm USB device")
	}
	return arm, err
}

func (arm *Arm) Close() {
	arm.device.Close()
}

func (arm *Arm) StartMove(move MoveType) error {
	result := arm.device.ControlMsg(0x40 /* req type */, 6 /* request */, 0x100, /* value */
		0 /* index */, move)
	if result < 0 {
		return fmt.Errorf("move failed with error %d", result)
	} else {
		return nil
	}
}

func (arm *Arm) Stop() {
	arm.StartMove(Reset)
}

func (arm *Arm) Move(moves []Move) error {
	for _, move := range moves {
		err := arm.StartMove(move.Move)
		if err != nil {
			return err
		}
		time.Sleep(move.Duration)
	}
	arm.StartMove(Reset)
	return nil
}
