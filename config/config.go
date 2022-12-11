package config

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"gopkg.in/yaml.v3"
)

type Layer struct {
	Name    string
	Title   string
	MinZoom int `yaml:"minZoom"`
	MaxZoom int `yaml:"maxZoom"`
	Filter  yaml.Node
}

type Config struct {
	Layers        []Layer
	FilterByLayer map[string]*bson.D
}

func ParseYAML(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.FilterByLayer = make(map[string]*bson.D)

	for _, l := range cfg.Layers {
		f, err := toBsonD(&l.Filter)
		if err != nil {
			return nil, err
		}
		cfg.FilterByLayer[l.Name] = &f
	}

	return &cfg, nil
}

func toBsonD(value *yaml.Node) (bson.D, error) {
	var d bson.D

	switch value.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(value.Content)/2; i++ {
			k, err := toString(value.Content[2*i])
			if err != nil {
				return nil, err
			}

			v, err := toValue(value.Content[2*i+1])
			if err != nil {
				return nil, err
			}

			d = append(d, bson.E{
				Key:   k,
				Value: v,
			})
		}
	default:
		return nil, fmt.Errorf("unsupported node kind for bson.D %v", *value)
	}

	return d, nil
}

func toBsonA(value *yaml.Node) (bson.A, error) {
	if value.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("unsupported node kind for string %v", *value)
	}
	var a bson.A

	for _, c := range value.Content {
		v, err := toValue(c)
		if err != nil {
			return nil, err
		}
		a = append(a, v)
	}

	return a, nil
}

func toString(value *yaml.Node) (string, error) {
	if value.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("unsupported node kind for string %v", *value)
	}
	return value.Value, nil
}

func toValue(value *yaml.Node) (any, error) {
	switch value.Kind {
	case yaml.ScalarNode:
		return toString(value)
	case yaml.MappingNode:
		return toBsonD(value)
	case yaml.SequenceNode:
		return toBsonA(value)
	}
	return nil, fmt.Errorf("unexpected node kind: %d", value.Kind)
}
