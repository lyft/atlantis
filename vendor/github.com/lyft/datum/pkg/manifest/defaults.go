package manifest

import "github.com/mitchellh/mapstructure"

var defaultDecoder = mapstructure.DecoderConfig{
	ZeroFields:       false,
	WeaklyTypedInput: true,
	ErrorUnused:      false,
}

func newDecoder(target interface{}) (*mapstructure.Decoder, error) {
	c := defaultDecoder
	c.Result = target
	return mapstructure.NewDecoder(&c)
}

func newStrictDecoder(target interface{}) (*mapstructure.Decoder, error) {
	c := defaultDecoder
	c.ErrorUnused = true
	c.Result = target
	return mapstructure.NewDecoder(&c)
}
