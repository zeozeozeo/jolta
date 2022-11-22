package jolta

import (
	"encoding/binary"
	"io"
)

func uint32ToBytes(val uint32) []byte {
	buf := make([]byte, 32/8)
	binary.LittleEndian.PutUint32(buf, val)
	return buf
}

func uint64ToBytes(val uint64) []byte {
	buf := make([]byte, 64/8)
	binary.LittleEndian.PutUint64(buf, val)
	return buf
}

func readUint8(r io.Reader) (uint8, error) {
	buf := make([]byte, 1)
	_, err := r.Read(buf)
	return buf[0], err
}

func readUint32(r io.Reader) (uint32, error) {
	buf := make([]byte, 32/8)
	_, err := r.Read(buf)
	return binary.LittleEndian.Uint32(buf), err
}

func readUint64(r io.Reader) (uint64, error) {
	buf := make([]byte, 64/8)
	_, err := r.Read(buf)
	return binary.LittleEndian.Uint64(buf), err
}

func readBytes(r io.Reader, length uint) ([]byte, error) {
	buf := make([]byte, length)
	_, err := r.Read(buf)
	return buf, err
}

func readPacket(r io.Reader) (Packet, error) {
	packetUint, err := readUint8(r)
	return Packet(packetUint), err
}

func writePacket(w io.Writer, packet Packet) error {
	return writeUint8(w, uint8(packet))
}

func writeUint64(w io.Writer, val uint64) error {
	_, err := w.Write(uint64ToBytes(val))
	return err
}

func writeUint32(w io.Writer, val uint32) error {
	_, err := w.Write(uint32ToBytes(val))
	return err
}

func writeUint8(w io.Writer, val uint8) error {
	_, err := w.Write([]byte{val})
	return err
}

func writeString(w io.Writer, val string) error {
	_, err := w.Write([]byte(val))
	return err
}

func writeBytes(w io.Writer, data []byte) error {
	_, err := w.Write(data)
	return err
}

func removeIndex[T any](s []T, i int) []T {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func writeEventName(w io.Writer, name string) error {
	if err := writeUint32(w, uint32(len(name))); err != nil {
		return err
	}
	return writeString(w, name)
}

func writeData(w io.Writer, data []byte) error {
	if err := writeUint64(w, uint64(len(data))); err != nil {
		return err
	}

	return writeBytes(w, data)
}

func readEventName(r io.Reader) (string, error) {
	eventNameLen, err := readUint32(r)
	if err != nil {
		return "", err
	}

	data, err := readBytes(r, uint(eventNameLen))
	return string(data), err
}

func readData(r io.Reader) ([]byte, error) {
	dataLen, err := readUint64(r)
	if err != nil {
		return nil, err
	}

	return readBytes(r, uint(dataLen))
}
