package proto

import (
	"../conn"
	"encoding/binary"
	"io"
)

func readMsgShared(c conn.Conn) (buffer []byte, err error) {
	c.Debug("Waiting to read message")
	var sz int64
	err = binary.Read(c, binary.LittleEndian, &sz)
	if err != nil {
		return
	}

	c.Debug("Reading message with length: %d", sz)
	buffer = make([]byte, sz)
	if _, err = io.ReadFull(c, buffer); err != nil {
		return
	}

	c.Debug("Read message %s", buffer)
	return
}

func ReadMsg(c conn.Conn) (msg Message, err error) {
	buffer, err := readMsgShared(c)
	if err != nil {
		return
	}

	return Unpack(buffer)
}

func ReadMsgInto(c conn.Conn, msg Message) (err error) {
	buffer, err := readMsgShared(c)
	if err != nil {
		return
	}
	return UnpackInto(buffer, msg)
}

func WriteMsg(c conn.Conn, msg interface{}) (err error) {
	buffer, err := Pack(msg)
	if err != nil {
		return
	}

	c.Debug("Writing message: %s", string(buffer))
	if err = binary.Write(c, binary.LittleEndian, int64(len(buffer))); err != nil {
		return
	}

	if _, err = c.Write(buffer); err != nil {
		return
	}

	return
}
