package xdr

import (
	"bytes"
	"encoding"
)

func Equals(a, b encoding.BinaryMarshaler) (bool, error) {
	serialized, err := a.MarshalBinary()
	if err != nil {
		return false, err
	}
	otherSerialized, err := b.MarshalBinary()
	if err != nil {
		return false, err
	}
	return bytes.Equal(serialized, otherSerialized), nil
}
