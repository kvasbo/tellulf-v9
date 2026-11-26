// Package han decodes what an AMS power meter sends on its HAN port: an HDLC
// frame around a DLMS/COSEM data notification. A Tibber Pulse set up for
// local MQTT publishes one frame per message, as the meter sent it.
//
// Only Aidon's layout is read, where every value comes with its OBIS code
// and its scaler: a list of structures [OBIS, value, [scaler, unit]].
// Kaifa sends values without OBIS codes, and Kamstrup without scalers.
package han

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// OBIS codes of the values Tellulf uses.
const (
	ActivePowerImport  = "1.0.1.7.0.255" // W
	ActivePowerExport  = "1.0.2.7.0.255" // W
	ActiveEnergyImport = "1.0.1.8.0.255" // Wh, every hour
)

// Reading is a frame's numeric values, scaled, by OBIS code ("1.0.1.7.0.255").
type Reading map[string]float64

// ErrNotFrame: the message isn't an HDLC frame. The Pulse also sends its own
// status as JSON on the same topic.
var ErrNotFrame = errors.New("han: not an HDLC frame")

const flag = 0x7e

// Decode checks a frame (with its flags) and returns its numeric values.
func Decode(b []byte) (Reading, error) {
	if len(b) == 0 || b[0] != flag {
		return nil, ErrNotFrame
	}
	if len(b) < 2+9 || b[len(b)-1] != flag {
		return nil, errors.New("han: frame cut short")
	}
	f := b[1 : len(b)-1]
	format := binary.BigEndian.Uint16(f)
	if n := int(format & 0x7ff); n != len(f) {
		return nil, fmt.Errorf("han: frame says %d bytes, has %d", n, len(f))
	}
	if format&0x800 != 0 {
		return nil, errors.New("han: segmented frames aren't read")
	}
	if crc16(f[:len(f)-2]) != binary.LittleEndian.Uint16(f[len(f)-2:]) {
		return nil, errors.New("han: check sum mismatch")
	}

	// Frame format (2), destination and source addresses (each ends with a
	// byte whose low bit is 1), control (1) and the header check sum (2).
	i := 2
	for range 2 {
		for i < len(f) && f[i]&1 == 0 {
			i++
		}
		i++
	}
	i += 1 + 2
	info := f[min(i, len(f)-2) : len(f)-2]

	// LLC (E6 E7 00), data-notification (0F), invoke id (4), date-time.
	if len(info) < 3+1+4+1 || info[0] != 0xe6 || info[1] != 0xe7 || info[3] != 0x0f {
		return nil, errors.New("han: not a data notification")
	}
	d := &decoder{b: info[8:]}
	switch d.byte() {
	case 0x00: // no date-time
	case 0x09:
		d.skip(int(d.byte()))
	default:
		return nil, errors.New("han: bad date-time")
	}
	v := d.value()
	if d.err != nil {
		return nil, d.err
	}
	r := Reading{}
	collect(v, r)
	return r, nil
}

// collect finds [OBIS, number, [scaler, unit]] and [OBIS, number] structures
// in v, at any depth.
func collect(v any, r Reading) {
	items, ok := v.([]any)
	if !ok {
		return
	}
	if obis, ok := obisOf(items); ok {
		if n, ok := items[1].(float64); ok {
			if len(items) == 3 {
				if su, ok := items[2].([]any); ok && len(su) == 2 {
					if s, ok := su[0].(float64); ok {
						n *= math.Pow(10, s)
					}
				}
			}
			r[obis] = n
		}
		return
	}
	for _, item := range items {
		collect(item, r)
	}
}

func obisOf(items []any) (string, bool) {
	if len(items) < 2 || len(items) > 3 {
		return "", false
	}
	b, ok := items[0].([]byte)
	if !ok || len(b) != 6 {
		return "", false
	}
	return fmt.Sprintf("%d.%d.%d.%d.%d.%d", b[0], b[1], b[2], b[3], b[4], b[5]), true
}

// decoder reads DLMS values: arrays and structures as []any, octet and
// visible strings as []byte, numbers as float64.
type decoder struct {
	b   []byte
	err error
}

func (d *decoder) take(n int) []byte {
	if d.err != nil {
		return nil
	}
	if n > len(d.b) {
		d.err = errors.New("han: data cut short")
		return nil
	}
	out := d.b[:n]
	d.b = d.b[n:]
	return out
}

func (d *decoder) byte() byte {
	if b := d.take(1); b != nil {
		return b[0]
	}
	return 0
}

func (d *decoder) skip(n int) { d.take(n) }

// uint reads an n-byte big-endian number, or 0 (with d.err set) when the
// data ends first.
func (d *decoder) uint(n int) uint64 {
	var v uint64
	for _, c := range d.take(n) {
		v = v<<8 | uint64(c)
	}
	return v
}

func (d *decoder) value() any {
	tag := d.byte()
	if d.err != nil {
		return nil
	}
	switch tag {
	case 0x00: // null
		return nil
	case 0x01, 0x02: // array, structure
		n := int(d.byte())
		items := make([]any, 0, n)
		for range n {
			items = append(items, d.value())
			if d.err != nil {
				return nil
			}
		}
		return items
	case 0x09, 0x0a: // octet string, visible string
		return d.take(int(d.byte()))
	case 0x0f: // integer
		return float64(int8(d.uint(1)))
	case 0x11, 0x16: // unsigned, enum
		return float64(d.uint(1))
	case 0x10: // long
		return float64(int16(d.uint(2)))
	case 0x12: // long-unsigned
		return float64(d.uint(2))
	case 0x05: // double-long
		return float64(int32(d.uint(4)))
	case 0x06: // double-long-unsigned
		return float64(d.uint(4))
	case 0x14: // long64
		return float64(int64(d.uint(8)))
	case 0x15: // long64-unsigned
		return float64(d.uint(8))
	default:
		d.err = fmt.Errorf("han: unknown data type %#x", tag)
		return nil
	}
}

// crc16 is the HDLC frame check sequence (CRC-16/X-25).
func crc16(b []byte) uint16 {
	crc := uint16(0xffff)
	for _, c := range b {
		crc ^= uint16(c)
		for range 8 {
			if crc&1 != 0 {
				crc = crc>>1 ^ 0x8408
			} else {
				crc >>= 1
			}
		}
	}
	return ^crc
}
