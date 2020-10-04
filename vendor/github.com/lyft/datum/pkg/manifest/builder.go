package manifest

import (
	"github.com/pkg/errors"
)

// Source represent external apt source
type Source struct {
	Name   string
	Source string
	KeyID  string
}

type protoParam struct {
	Builder    string
	SubImage   string `mapstructure:"sub_image"`
	Packages   []string
	Dockerfile string
	BuildArgs  []string
	Sources    []Source
}

func populateParams(params interface{}) (Params, error) {
	var prototype protoParam
	var p Params
	decoder, err := newStrictDecoder(&prototype)
	if err != nil {
		return p, errors.Wrap(err, "failed to decode params due to decoder issue")
	}
	err = decoder.Decode(params)
	if err != nil {
		return p, errors.Wrap(err, "failed to decode valid params")
	}
	p = Params(prototype)
	return p, nil
}

func (s *SingleBuilder) populate(bSettings interface{}) error {
	rawMap, ok := bSettings.(map[string]interface{})
	if !ok {
		return errors.Errorf("builder component cannot be parsed")
	}
	for k, rawV := range rawMap {
		if k == "name" {
			v, ok := rawV.(string)
			if !ok {
				return errors.New("builder component does not have a valid name")
			}
			s.Name = v
		}
		if k == "params" {
			params, err := populateParams(rawV)
			if err != nil {
				return errors.Wrap(err, "error decoding builder parameters")
			}
			s.Params = params
		}
	}
	if s.Name == "" {
		return errors.New("builder component does not have a name")
	}
	s.Params.Builder = s.Name
	return nil
}

func populateBuilders(bList interface{}) (BuilderList, error) {
	var builderList BuilderList
	if list, ok := bList.([]interface{}); ok {
		for _, input := range list {
			var b SingleBuilder
			if err := b.populate(input); err != nil {
				return builderList, errors.Wrap(err, "error decoding builders")
			}
			builderList = append(builderList, b)
		}
	}
	return builderList, nil
}

func populateBuilderParams(params interface{}) (BuilderParams, error) {
	var builders BuilderParams
	rawMap, ok := params.(map[string]interface{})
	if !ok {
		return builders, errors.Errorf("builder params component cannot be parsed")
	}

	if rawV, ok := rawMap["builders"]; ok {
		builderList, err := populateBuilders(rawV)
		if err != nil {
			return builders, errors.Wrap(err, "error decoding builder parameters")
		}
		builders.Builders = builderList
	} else {
		return builders, errors.New("invalid builder structure")
	}
	return builders, nil
}

// populateBuilder populates a builder struct
func (m *MultiBuilder) populate(builderComponent interface{}) error {
	rawMap, ok := builderComponent.(map[string]interface{})
	if !ok {
		return errors.Errorf("builder component cannot be parsed")
	}

	for k, rawV := range rawMap {
		if k == "name" {
			v, ok := rawV.(string)
			if !ok {
				return errors.New("builder component does not have a valid name")
			}
			m.Name = v
		}
		if k == "params" {
			params, err := populateBuilderParams(rawV)
			if err != nil {
				return errors.Wrap(err, "error decoding builder parameters")
			}
			m.Params = params
		}
	}
	return nil
}

func parseBuilder(builderComponent interface{}) (Builder, error) {
	rawMap, ok := builderComponent.(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("builder component cannot be parsed")
	}

	var b Builder
	var err error
	if rawV, ok := rawMap["name"]; ok {
		if rawV == "multi" {
			var m MultiBuilder
			err = m.populate(builderComponent)
			b = &m
		} else {
			var s SingleBuilder
			err = s.populate(builderComponent)
			b = &s
		}
	} else {
		return nil, errors.Errorf("builder component does not have a name")
	}
	return b, err
}
