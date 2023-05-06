package main

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"go.bug.st/serial"
)

// https://wings.twelite.info/how-to-use/parent-mode/receive-message/app_aria
// https://wings.twelite.info/how-to-use/parent-mode/receive-message/app_pal/app_pal-detail
type Aria struct {
	RelaySerialID  string
	LQI            uint8
	ContinueNumber uint16
	SenderSerialID string
	SenderLID      string
	SensorType     string
	PALID          string
	SensorCount    uint8
	PacketProperty PacketProperty
	Event          MagnetismEventType
	SupplyVoltage  uint16
	DC1Voltage     uint16
	Magnetism      MagnetismType
	Temperature    float32
	Humidity       float32
}

type PacketProperty struct {
	packetID             string
	hasEvent             bool
	WakeFactorDataSource WakeFactorDataSourceType
	WakeFactor           WakeFactorType
}

// 起床要因データソース
type WakeFactorDataSourceType string

const (
	// 0x00(0): 磁気センサー:
	Magnetism WakeFactorDataSourceType = "Magnestim"
	// 0x01(1): 温度
	Temperature WakeFactorDataSourceType = "Temperature"
	// 0x02(2): 湿度
	Humidity WakeFactorDataSourceType = "Humidity"
	// 0x03(3): 照度
	Illuminance WakeFactorDataSourceType = "Illuminance"
	// 0x04(4): 加速度
	Acceleration WakeFactorDataSourceType = "Acceleration"
	// 0x31(49): DIO
	DIO WakeFactorDataSourceType = "DIO"
	// 0x32:(50): タイマー
	Timer WakeFactorDataSourceType = "Timer"
)

func byteToWakeFactorDataSourceType(b byte) (t WakeFactorDataSourceType) {
	switch b {
	case 0x00:
		t = Magnetism
	case 0x01:
		t = Temperature
	case 0x02:
		t = Humidity
	case 0x03:
		t = Illuminance
	case 0x04:
		t = Acceleration
	case 0x31:
		t = DIO
	case 0x32:
		t = Timer
	}
	return t
}

// 起床要因
type WakeFactorType string

const (
	// 0x00(0): 磁気センサー
	SendFactorEventOccurred WakeFactorType = "SendFactorEventOccurred"
	// 0x01(1): 値が変化した
	ValueChanged WakeFactorType = "ValueChanged"
	// 0x02(2): 値が閾値を超えた
	ExceededThreshold WakeFactorType = "ExceededThreshold"
	// 0x03(3): 閾値を下回った
	BelowThreshold WakeFactorType = "BelowThreshold"
	// 0x04(4): 敷地の範囲に入った
	InThresholdRange WakeFactorType = "InThresholdRange"
)

func byteToWakeFactorType(b byte) (t WakeFactorType) {
	switch b {
	case 0x00:
		t = SendFactorEventOccurred
	case 0x01:
		t = ValueChanged
	case 0x02:
		t = ExceededThreshold
	case 0x03:
		t = BelowThreshold
	case 0x04:
		t = InThresholdRange
	}
	return t
}

type MagnetismEventType string

const (
	// 0x00(0):近くに磁石がない
	E_NoMagnet MagnetismEventType = "NoMagnet"
	// 0x01(1):磁石のN極が近くにある
	E_NPoleMagnet MagnetismEventType = "NPoleMagnet"
	// 0x02(2):磁石のS極が近くにある
	E_SPoleMagnet MagnetismEventType = "SPoleMagnet"
)

func byteToMagnetismEventType(b byte) (t MagnetismEventType) {
	switch b {
	case 0x00:
		t = E_NoMagnet
	case 0x01:
		t = E_NPoleMagnet
	case 0x02:
		t = E_SPoleMagnet
	}
	return t
}

type MagnetismType string

const (
	// 0x00(0): 近くに磁石がない
	NoMagnet MagnetismType = "NoMagnet"
	// 0x01(1): N極が近い
	NPoleMagnet MagnetismType = "NPoleMagnet"
	// 0x02(2): S極が近い
	SPoleMagnet MagnetismType = "SPoleMagnet"
	// 0x80(128): 定期送信ビット(このビットが1の時は定期送信、0の時は磁気センサーの状態が変化したことを示す)
	NoChangeNoMagne     MagnetismType = "NoChangeNoMagne"
	NoChangeNPoleMagnet MagnetismType = "NoChangeNPoleMagnet"
	NoChangeSPoleMagnet MagnetismType = "NoChangeSPoleMagnet"
)

func byteToMagnetismType(b byte) (t MagnetismType) {
	switch b {
	case 0x00:
		t = NoMagnet
	case 0x01:
		t = NPoleMagnet
	case 0x02:
		t = SPoleMagnet
	case 0x80:
		t = NoChangeNoMagne
	case 0x81:
		t = NoChangeNPoleMagnet
	case 0x82:
		t = NoChangeSPoleMagnet
	}
	return t
}

func NewAria(rawData []byte) *Aria {
	return &Aria{
		RelaySerialID:  hex.EncodeToString(rawData[0 : 0+4]),
		LQI:            uint8(rawData[4]),
		ContinueNumber: binary.BigEndian.Uint16(rawData[5 : 5+2]),
		SenderSerialID: hex.EncodeToString(rawData[7 : 7+4]),
		SenderLID:      fmt.Sprintf("%x", int(rawData[11])),
		SensorType:     fmt.Sprintf("%x", int(rawData[12])),
		PALID:          fmt.Sprintf("%x", int(rawData[13])),
		SensorCount:    uint8(rawData[14]),
		PacketProperty: NewPacketProperty(*(*[3]byte)(rawData[19 : 19+3])),
		Event:          byteToMagnetismEventType(rawData[26]),
		SupplyVoltage:  binary.BigEndian.Uint16(rawData[34 : 34+2]),
		DC1Voltage:     binary.BigEndian.Uint16(rawData[40 : 40+2]),
		Magnetism:      byteToMagnetismType(rawData[46]),
		Temperature:    float32(binary.BigEndian.Uint16(rawData[51:51+2])) / 100,
		Humidity:       float32(binary.BigEndian.Uint16(rawData[57:57+2])) / 100,
	}
}

func NewPacketProperty(bs [3]byte) PacketProperty {
	var hasEvent bool
	if bs[0]&0x80 == 1 {
		hasEvent = false
	} else {
		hasEvent = true
	}
	packetID := bs[0] & 0x7F

	return PacketProperty{
		packetID:             strconv.Itoa(int(packetID)),
		hasEvent:             hasEvent,
		WakeFactorDataSource: byteToWakeFactorDataSourceType(bs[1]),
		WakeFactor:           byteToWakeFactorType(bs[2]),
	}

}

func main() {
	portNmae := flag.String("p", "", "MONOSTICKが接続されているポート名")
	flag.Parse()

	if *portNmae == "" {
		fmt.Fprintf(os.Stderr, "MONOSTICKが接続されているポート名を入力してください\n")
		os.Exit(1)
	}

	port, err := serial.Open(*portNmae, &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
	})
	if err != nil {
		panic(err)
	}
	defer port.Close()

	r := bufio.NewReader(port)
	for {
		line, _, err := r.ReadLine()
		if err != nil {
			log.Panic(err)
		}
		d, _ := hex.DecodeString(string(line[1:]))
		if len(line) == 123 && line[0] == ':' {
			aria := NewAria(d)
			fmt.Fprintf(os.Stdout, "%+v\n", aria)
		}
	}
}
