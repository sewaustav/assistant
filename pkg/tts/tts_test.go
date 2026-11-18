package tts

import (
	"encoding/binary"
	"testing"
)

func makeWAV(rate, channels, bits int, pcm []byte) []byte {
	buf := make([]byte, 0, 44+len(pcm))
	buf = append(buf, []byte("RIFF")...)
	buf = append(buf, 0, 0, 0, 0)
	buf = append(buf, []byte("WAVEfmt ")...)
	buf = append(buf, 16, 0, 0, 0)
	var tmp [4]byte
	binary.LittleEndian.PutUint16(tmp[:2], 1)
	buf = append(buf, tmp[:2]...)
	binary.LittleEndian.PutUint16(tmp[:2], uint16(channels))
	buf = append(buf, tmp[:2]...)
	binary.LittleEndian.PutUint32(tmp[:4], uint32(rate))
	buf = append(buf, tmp[:4]...)
	byteRate := rate * channels * bits / 8
	binary.LittleEndian.PutUint32(tmp[:4], uint32(byteRate))
	buf = append(buf, tmp[:4]...)
	binary.LittleEndian.PutUint16(tmp[:2], uint16(channels*bits/8))
	buf = append(buf, tmp[:2]...)
	binary.LittleEndian.PutUint16(tmp[:2], uint16(bits))
	buf = append(buf, tmp[:2]...)
	buf = append(buf, []byte("data")...)
	binary.LittleEndian.PutUint32(tmp[:4], uint32(len(pcm)))
	buf = append(buf, tmp[:4]...)
	buf = append(buf, pcm...)
	return buf
}

func TestParseWAV(t *testing.T) {
	pcm := make([]byte, 9600)
	wav := makeWAV(48000, 1, 16, pcm)
	out, rate, channels, bits, err := parseWAV(wav)
	if err != nil {
		t.Fatal(err)
	}
	if rate != 48000 || channels != 1 || bits != 16 {
		t.Fatalf("rate=%d channels=%d bits=%d", rate, channels, bits)
	}
	if len(out) != len(pcm) {
		t.Fatalf("len(pcm)=%d", len(out))
	}
}

func TestParseWAVRejectsGarbage(t *testing.T) {
	if _, _, _, _, err := parseWAV([]byte("not a wav")); err == nil {
		t.Fatal("ожидалась ошибка")
	}
}
