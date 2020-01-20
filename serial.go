package main

import (
	"errors"
	serialPackage "go.bug.st/serial"
	"log"
	"strconv"
	"time"
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
	port        serialPackage.Port
	cmds        chan cmd
	readChannel chan []byte
}

func Open() (*Serial, error) {
	mode := &serialPackage.Mode{
		BaudRate: CmdSerialBaud,
	}
	port, err := serialPackage.Open("/dev/ttyAMA0", mode)
	if err != nil {
		return nil, err
	}

	s := &Serial{port: port, cmds: make(chan cmd), readChannel: make(chan []byte)}
	go s.runCmdHandler()
	go s.runReadHandler()
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

func (serial *Serial) runCmdHandler() {
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

	err := serial.write([]byte{CmdSync0, CmdSync1, crc, cmd, data1, data2, data3, data4, CmdTermByte})
	if err != nil {
		return nil, err
	}

	return serial.readResponse(cmd)
}

func (serial *Serial) write(bytes []byte) error {
	//for i, readByte := range bytes {
	//	log.Println("send byte[" + strconv.Itoa(i) + "]: " + strconv.Itoa(int(readByte)))
	//}
	_, err := serial.port.Write(bytes)
	return err
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

func (serial *Serial) readResponse(cmd byte) ([]byte, error) {
	select {
	case response := <-serial.readChannel:
		return response[5:], nil
	case <-time.After(1 * time.Second):
		return nil, errors.New("timeout when reading serial")
	}
}

func (serial *Serial) runReadHandler() {
	var readBytes = make([]byte, 0, 20)
	var buf = make([]byte, 20)
	scanPosition := 0
	responseStart := -1
	for {
		read, err := serial.port.Read(buf)
		if err != nil {
			log.Println("error reading:", err)
			continue
		}
		readBytes = append(readBytes, buf[0:read]...)
		for ; scanPosition < len(readBytes); scanPosition++ {
			if readBytes[scanPosition] == CmdSync0 {
				responseStart = scanPosition
			}
			if readBytes[scanPosition] == CmdTermByte && responseStart != -1 {
				bytes, err := validate(readBytes[responseStart : scanPosition+1])
				if err != nil {
					log.Println("error reading response", err)
				} else {
					serial.readChannel <- bytes
				}
				readBytes = make([]byte, 0, 20)
				responseStart = -1
				scanPosition = 0
				break
			}
		}
	}
}

func validate(readBytes []byte) ([]byte, error) {
	//for i, readByte := range readBytes {
	//	log.Println("receive byte[" + strconv.Itoa(i) + "]: " + strconv.Itoa(int(readByte)))
	//}
	if readBytes[0] != CmdSync0 {
		return nil, errors.New("wrong sync byte 0 :( " + strconv.Itoa(int(readBytes[0])))
	}
	if readBytes[1] != CmdSync1 {
		return nil, errors.New("wrong sync byte 1 :( " + strconv.Itoa(int(readBytes[1])))
	}
	var crc = calcCrc(readBytes)
	if readBytes[2] != crc {
		return nil, errors.New("wrong crc :( " + strconv.Itoa(int(crc)))
	}
	if readBytes[3] != StatusAckOk {
		return nil, errors.New("wrong status :( " + strconv.Itoa(int(readBytes[3])))
	}
	return readBytes, nil
}

func calcCrc(bytes []byte) byte {
	var crc byte = 0
	for i := 0; i < len(bytes)-1; i++ {
		if i != 2 {
			crc ^= bytes[i]
		}
	}
	return crc
}

var cmdMapping = map[string]serialCmd{
	"reset":     &okCall{[]byte{CmdRegReset, 100, 100, 100}},
	"power_on":  &okCall{[]byte{CmdRegPower, 1}},
	"power_off": &okCall{[]byte{CmdRegPower, 0}},

	"walk_stop":       &okCall{[]byte{CmdRegWalk, 128, 128, 128}},
	"walk_forward":    &okCall{[]byte{CmdRegWalk, 128, 0, 128}},
	"walk_back":       &okCall{[]byte{CmdRegWalk, 128, 255, 128}},
	"walk_left":       &okCall{[]byte{CmdRegWalk, 0, 128, 128}},
	"walk_right":      &okCall{[]byte{CmdRegWalk, 255, 128, 128}},
	"walk_turn_left":  &okCall{[]byte{CmdRegWalk, 128, 128, 0}},
	"walk_turn_right": &okCall{[]byte{CmdRegWalk, 128, 128, 255}},

	"sound":       &parameterizedOkCall{CmdRegSound, []string{"duration", "frequency"}, []convertParam{paramToByte, mapping(0, 2550, 0, 255)}},
	"body_height": &parameterizedOkCall{CmdRegBodyHeight, []string{"height"}, []convertParam{percentMapping(0, 130)}},
	"speed":       &parameterizedOkCall{CmdRegSpeed, []string{"speed"}, []convertParam{percentMapping(100, 0)}},
	"walk":        &parameterizedOkCall{CmdRegWalk, []string{"side", "forward", "turn"}, []convertParam{mapping(-100, 100, 1, 255), mapping(-100, 100, 255, 1), mapping(-100, 100, 1, 255)}},

	"akku_charge": &onlyReturnCall{[]byte{CmdRegAkku}, convertAkkuCharge},
}

type serialCmd interface {
	call(*Serial, map[string]string) string
}

type okCall struct {
	data []byte
}

type parameterizedOkCall struct {
	cmd       byte
	params    []string
	converter []convertParam
}

type onlyReturnCall struct {
	data      []byte
	converter convertReturnValues
}
type convertReturnValues func([]byte) string

type convertParam func(string) (byte, error)

func (cmd *okCall) call(serial *Serial, _ map[string]string) string {
	_, err := serial.sendArray(cmd.data)
	if err != nil {
		return err.Error()
	}
	return "ok"
}

func (cmd *parameterizedOkCall) call(serial *Serial, paramsMap map[string]string) string {
	params, err := gatherParams(paramsMap, cmd.params)
	if err != nil {
		return err.Error()
	}
	data := make([]byte, len(cmd.params))
	for i := 0; i < len(data); i++ {
		paramAsByte, err := cmd.converter[i](params[i])
		if err != nil {
			return err.Error()
		}
		data[i] = paramAsByte
	}
	_, err = serial.sendArray(append(append(make([]byte, 0, 20), cmd.cmd), data...))
	if err != nil {
		return err.Error()
	}
	return "ok"
}

func (cmd *onlyReturnCall) call(serial *Serial, _ map[string]string) string {
	returnData, err := serial.sendArray(cmd.data)
	if err != nil {
		return err.Error()
	}
	return cmd.converter(returnData)
}

func gatherParams(paramsMap map[string]string, paramNames []string) ([]string, error) {
	var params = make([]string, 0, 4)
	for _, paramName := range paramNames {
		param, ok := paramsMap[paramName]
		if !ok {
			return nil, errors.New("missing param: " + paramName)
		}
		params = append(params, param)
	}
	return params, nil
}

func convertAkkuCharge(bytes []byte) string {
	//TODO experiment with convert to percentage?
	var charge = int(bytes[0])
	charge = charge << 8
	charge |= int(bytes[1])
	return strconv.Itoa(charge)
}

func percentMapping(start int, end int) convertParam {
	return mapping(0, 100, start, end)
}

func mapping(inStart int, inEnd int, outStart int, outEnd int) convertParam {
	return func(param string) (byte, error) {
		paramAsInt, err := strconv.Atoi(param)
		if err != nil {
			return 0, err
		}
		return doMapping(paramAsInt, inStart, inEnd, outStart, outEnd)
	}
}

func doMapping(inValue int, inStart int, inEnd int, outStart int, outEnd int) (byte, error) {
	if inStart >= inEnd {
		return 0, errors.New("inStart has to be smaller then inEnd")
	}
	if inValue < inStart || inValue > inEnd {
		return 0, errors.New("param has to be in inStart and inEnd range")
	}
	inSteps := inEnd - inStart
	outRange := outEnd - outStart
	value := outStart + ((outRange)*(inValue-inStart))/inSteps
	return byte(value), nil
}

func paramToByte(param string) (byte, error) {
	paramAsInt, err := strconv.Atoi(param)
	if err != nil {
		return 0, err
	}
	if paramAsInt < 0 || paramAsInt > 255 {
		return 0, errors.New("value " + param + " out of byte range")
	}
	return byte(paramAsInt), nil
}
