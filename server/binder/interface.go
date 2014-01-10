package binder

import (
	"net"
	"encoding/json"
)

// the most awful hack in the whole library
// binders are passed their options as an empty interface
// and must cast it to the proper type. However, because the
// options are defined as the empty interface in the protocol,
// they are unserialized as a map.
// So in order to get them in the form we want, we write them out
// as JSON again and then read them back in with the now-known
// proper type for deserializion
func unpackOptions(rawOpts, unpacked interface{}) error {
	bytes, err := json.Marshal(rawOpts)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(bytes, unpacked); err != nil {
		return err
	}

	return nil
}

type Binder interface {
	Bind(interface{}) (net.Listener, string, error)
}
