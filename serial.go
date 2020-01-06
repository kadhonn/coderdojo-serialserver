package main

import (
	"errors"
	serialPackage "go.bug.st/serial"
	"log"
	"strconv"
)

const CmdSerialBaud = 38400
const CmdSync0 byte = 33
const CmdSync1 byte = 42
const CmdTermByte byte = 255

const CmdRegPower byte = 5
const CmdRegSpeed byte = 10
const CmdRegGaintMode byte = 15
const CmdRegBalanceMode byte = 20
const CmdRegBodyHeight byte = 25
const CmdRegTranslate byte = 35
const CmdRegWalk byte = 40
const CmdRegRotate byte = 45
const CmdRegDoubleHeight byte = 50
const CmdRegDoubleLength byte = 55
const CmdRegSingleLegPos byte = 60
const CmdRegSound byte = 65
const CmdRegOut byte = 70
const CmdRegStatusLed byte = 75

const CmdRegSaLeg byte = 100
const CmdRegAkku byte = 105
const CmdRegPs2Active byte = 110
const CmdRegIsWalking byte = 115
const CmdRegIsPowerOn byte = 120
const CmdRegReadPs2Values byte = 125
const CmdRegIn1 byte = 130

const CmdRegReset byte = 255

const StatusAckOk byte = 64
const StatusErrTerm byte = 1
const StatusErrState byte = 2
const StatusErrCrc byte = 3
const StatusErrCmd byte = 255

const Walkmode byte = 0
const TranslateMode byte = 1
const RotateMode byte = 2
const SingleLegMode byte = 3

const BalancemodeOn byte = 1
const BalancemodeOff byte = 0

type cmd struct {
	name   string
	params map[string]string
	answer chan string
}
type Serial struct {
	port serialPackage.Port
	cmds chan cmd
}

func Open() (*Serial, error) {
	mode := &serialPackage.Mode{
		BaudRate: 38400,
	}
	port, err := serialPackage.Open("/dev/ttyAMA0", mode)
	if err != nil {
		return nil, err
	}

	s := &Serial{port: port, cmds: make(chan cmd)}
	go s.run()
	return s, nil
}

func InvokeCommand(serial *Serial, cmdName string, params map[string]string) string {
	cmd := cmd{
		cmdName,
		params,
		make(chan string),
	}
	serial.cmds <- cmd
	return <-cmd.answer
}

func (serial *Serial) run() {
	for {
		select {
		case cmd := <-serial.cmds:
			serial.runCommand(cmd)
		}
	}
}

func (serial *Serial) sendArray(data []byte) ([]byte, error) {
	for len(data) != 5 {
		data = append(data, 0)
	}
	return serial.sendBytes(data[0], data[1], data[2], data[3], data[4])
}

func (serial *Serial) sendBytes(cmd byte, data1 byte, data2 byte, data3 byte, data4 byte) ([]byte, error) {
	var crc = CmdSync0 ^ CmdSync1 ^ cmd ^ data1 ^ data2 ^ data3 ^ data4

	_, err := serial.port.Write([]byte{CmdSync0, CmdSync1, crc, cmd, data1, data2, data3, data4, CmdTermByte})
	if err != nil {
		return nil, err
	}

	var readBytes = make([]byte, 20)
	readBytes[0] = 0
	var buf = make([]byte, 12)
	//TODO blocks, can lead to server dying until restarted or bug restarted
	//TODO read is reeeally friggin hacky and not race condition safe, rewrite!
	for readBytes[len(readBytes)-1] != CmdTermByte {
		read, err := serial.port.Read(buf)
		if err != nil {
			return nil, err
		}
		readBytes = append(readBytes, buf[:read]...)
	}
	for readBytes[0] != CmdSync0 {
		readBytes = readBytes[1:]
	}

	for i, byte := range readBytes {
		log.Println("byte[" + strconv.Itoa(i) + "]: " + strconv.Itoa(int(byte)))
	}

	//TODO crc check
	if readBytes[3] != StatusAckOk {
		return nil, errors.New("wrong status :( " + strconv.Itoa(int(readBytes[3])))
	}

	if readBytes[4] != cmd {
		return nil, errors.New("wrong return :( " + strconv.Itoa(int(readBytes[4])) + " instead of " + strconv.Itoa(int(cmd)))
	}

	return readBytes[5:11], nil
}

func (serial *Serial) runCommand(cmd cmd) {
	serialCmd, ok := cmdMapping[cmd.name]
	if !ok {
		cmd.answer <- "unknown command"
	} else {
		answer := serialCmd.call(serial, cmd.params)
		cmd.answer <- answer
	}

	close(cmd.answer)
}

var cmdMapping = map[string]serialCmd{
	"reset":     &simpleOkCall{[]byte{CmdRegReset, 100, 100, 100}},
	"power_on":  &simpleOkCall{[]byte{CmdRegPower, 1}},
	"power_off": &simpleOkCall{[]byte{CmdRegPower, 0}},

	"walk_stop":       &simpleOkCall{[]byte{CmdRegWalk, 128, 128, 128}},
	"walk_forward":    &simpleOkCall{[]byte{CmdRegWalk, 128, 0, 128}},
	"walk_back":       &simpleOkCall{[]byte{CmdRegWalk, 128, 255, 128}},
	"walk_left":       &simpleOkCall{[]byte{CmdRegWalk, 0, 128, 128}},
	"walk_right":      &simpleOkCall{[]byte{CmdRegWalk, 255, 128, 128}},
	"walk_turn_left":  &simpleOkCall{[]byte{CmdRegWalk, 128, 128, 0}},
	"walk_turn_right": &simpleOkCall{[]byte{CmdRegWalk, 128, 128, 255}},

	"sound": &simpleOkCall{[]byte{CmdRegSound, 50, 150}}, //TODO figure out parameters

	"body_height": &simpleParameterizedOkCall{CmdRegBodyHeight, []string{"height"}},

	"akku_charge": &simpleReturnCall{[]byte{CmdRegAkku}, convertAkkuCharge},
}

type serialCmd interface {
	call(*Serial, map[string]string) string
}

type simpleOkCall struct {
	data []byte
}

type simpleParameterizedOkCall struct {
	cmd    byte
	params []string
}

type simpleReturnCall struct {
	data      []byte
	converter convertReturnValues
}

type convertReturnValues func([]byte) string

func (cmd *simpleOkCall) call(serial *Serial, _ map[string]string) string {
	_, err := serial.sendArray(cmd.data)
	if err != nil {
		return err.Error()
	}
	return "ok"
}

func (cmd *simpleParameterizedOkCall) call(serial *Serial, paramsMap map[string]string) string {
	var data = []byte{cmd.cmd}
	for _, paramName := range cmd.params {
		param, ok := paramsMap[paramName]
		if !ok {
			return "missing param: " + paramName
		}
		paramAsInt, err := strconv.Atoi(param)
		if err != nil {
			return err.Error()
		}
		if paramAsInt > 255 {
			return "value " + param + " out of byte range"
		}
		data = append(data, byte(paramAsInt))
	}
	_, err := serial.sendArray(data)
	if err != nil {
		return err.Error()
	}
	return "ok"
}

func (cmd *simpleReturnCall) call(serial *Serial, _ map[string]string) string {
	returnData, err := serial.sendArray(cmd.data)
	if err != nil {
		return err.Error()
	}
	return cmd.converter(returnData)
}

func convertAkkuCharge(bytes []byte) string {
	//TODO experiment with convert to percentage?
	var charge = int(bytes[0])
	charge = charge << 8
	charge |= int(bytes[1])
	return strconv.Itoa(charge)
}
