package proto

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// UnpackInterfaceField allows the caller to unpack anything that is specified
// in the protocol as an interface{}
// This includes the all Extra fields and the Options for a Bind.
// This is one of the most awful hacks in the whole library.
// The trouble is that anything that is passed as an empty interface in
// the protocol will be deserialized into a map[string]interface{}.
// So in order to get them in the form we want, we write them out
// as JSON again and then read them back in with the now-known
// proper type for deserializion
func UnpackInterfaceField(interfaceField, deserializeInto interface{}) error {
	bytes, err := json.Marshal(interfaceField)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(bytes, deserializeInto); err != nil {
		return err
	}

	return nil
}

func unpack(buffer []byte, msgIn Message) (msg Message, err error) {
	var env Envelope
	if err = json.Unmarshal(buffer, &env); err != nil {
		return
	}

	if msgIn == nil {
		t, ok := TypeMap[env.Type]

		if !ok {
			err = errors.New(fmt.Sprintf("Unsupposted message type %s", env.Type))
			return
		}

		// guess type
		msg = reflect.New(t).Interface().(Message)
	} else {
		msg = msgIn
	}

	err = json.Unmarshal(env.Payload, &msg)
	return
}

func UnpackInto(buffer []byte, msg Message) (err error) {
	_, err = unpack(buffer, msg)
	return
}

func Unpack(buffer []byte) (msg Message, err error) {
	return unpack(buffer, nil)
}

func Pack(payload interface{}) ([]byte, error) {
	return json.Marshal(struct {
		Type    string
		Payload interface{}
	}{
		Type:    reflect.TypeOf(payload).Elem().Name(),
		Payload: payload,
	})
}
