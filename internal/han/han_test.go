package han

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// Frames as the Tibber Pulse sent them from the home's Aidon meter on
// 2026-11-26.
const (
	// Every 2.5 seconds: active power import, 11686 W.
	list1 = "7ea02a410883130413e6e7000f40000000000101020309060100010700ff0600002da602020f00161bd0967e"
	// Every 10 seconds: power, reactive power, currents L1 and L3, voltages.
	list2 = "7ea10b41088313fa7ce6e7000f4000000000010c020209060101000281ff0a0b4149444f4e5f5630303031020209060000600100ff0a1037333539393932383935333936363834020209060000600107ff0a0436353235020309060100010700ff0600002d4202020f00161b020309060100020700ff060000000002020f00161b020309060100030700ff060000000002020f00161d020309060100040700ff060000033402020f00161d0203090601001f0700ff10014002020fff1621020309060100470700ff10011002020fff1621020309060100200700ff12090602020fff1623020309060100340700ff1208ed02020fff1623020309060100480700ff1208f102020fff162344997e"
)

func frame(t *testing.T, s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDecodeList1(t *testing.T) {
	r, err := Decode(frame(t, list1))
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 1 || r[ActivePowerImport] != 11686 {
		t.Errorf("got %v", r)
	}
}

func TestDecodeList2(t *testing.T) {
	r, err := Decode(frame(t, list2))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{
		ActivePowerImport: 11586,
		ActivePowerExport: 0,
		"1.0.3.7.0.255":   0,
		"1.0.4.7.0.255":   820,
		"1.0.31.7.0.255":  32.0, // current L1, scaler −1
		"1.0.71.7.0.255":  27.2,
		"1.0.32.7.0.255":  231.0, // voltage L1
		"1.0.52.7.0.255":  228.5,
		"1.0.72.7.0.255":  228.9,
	}
	for k, v := range want {
		if got, ok := r[k]; !ok || got-v > 1e-9 || v-got > 1e-9 {
			t.Errorf("%s: got %v (%v), want %v", k, got, ok, v)
		}
	}
	if len(r) != len(want) {
		t.Errorf("only numbers count, got %v", r)
	}
}

// The Pulse's own status is JSON on the same topic.
func TestDecodeSkipsStatus(t *testing.T) {
	_, err := Decode([]byte(`{"status":{"rssi":-65,"meter":"Aidon_V2"}}`))
	if !errors.Is(err, ErrNotFrame) {
		t.Errorf("got %v, want ErrNotFrame", err)
	}
}

// A frame whose check sum is right but whose data ends inside a number must
// fail, not panic.
func TestDecodeDataCutShort(t *testing.T) {
	b := frame(t, list1)
	cut := strings.Index(hex.EncodeToString(b), "0600002da6")/2 + 3 // after 06 00 00
	f := append([]byte(nil), b[1:cut]...)
	f = append(f, 0, 0)             // room for the FCS
	f[0], f[1] = 0xa0, byte(len(f)) // type 3, the new length
	hcs := crc16(f[:8])             // header: format, 41, 08 83, 13
	f[8], f[9] = byte(hcs), byte(hcs>>8)
	fcs := crc16(f[:len(f)-2])
	f[len(f)-2], f[len(f)-1] = byte(fcs), byte(fcs>>8)
	if _, err := Decode(append(append([]byte{flag}, f...), flag)); err == nil {
		t.Error("want an error for data cut short")
	}
}

func TestDecodeRejectsDamage(t *testing.T) {
	b := frame(t, list1)
	b[30] ^= 0x01 // a bit in the value
	if _, err := Decode(b); err == nil {
		t.Error("a flipped bit should fail the check sum")
	}
	if _, err := Decode(frame(t, list1)[:20]); err == nil {
		t.Error("a cut frame should fail")
	}
}

// Every hour, the list also has the meter clock (skipped: not a number) and
// the meter readings, here with scaler +1.
const list3 = "7ea17741088313391ee6e7000f40000000000111020209060101000281ff0a0b4149444f4e5f5630303031020209060000600100ff0a1037333539393932383935333936363834020209060000600107ff0a0436353235020309060100010700ff06000027fa02020f00161b020309060100020700ff060000000002020f00161b020309060100030700ff060000000002020f00161d020309060100040700ff060000036102020f00161d0203090601001f0700ff10012a02020fff1621020309060100470700ff1000da02020fff1621020309060100200700ff12090402020fff1623020309060100340700ff12090102020fff1623020309060100480700ff1208fc02020fff1623020209060000010000ff090c07ea0b1a040f0000ff8000ff020309060100010800ff0603141c7302020f01161e020309060100020800ff060000000002020f01161e020309060100030800ff060000401d02020f011620020309060100040800ff06004fac6002020f01162094217e"

func TestDecodeList3(t *testing.T) {
	r, err := Decode(frame(t, list3))
	if err != nil {
		t.Fatal(err)
	}
	if r[ActivePowerImport] != 10234 || r[ActiveEnergyImport] != 0x03141c73*10 || r["1.0.4.8.0.255"] != 0x4fac60*10 {
		t.Errorf("got %v", r)
	}
	if _, ok := r["0.0.1.0.0.255"]; ok || len(r) != 13 {
		t.Errorf("%d values: %v", len(r), r)
	}
}
