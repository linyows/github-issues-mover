package main

import (
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

type R struct {
	Wrong string `yaml:"wrong"`
	Right string `yaml:"right"`
}

type Map struct {
	User []R `yaml:"user"`
	Body []R `yaml:"body"`
}

func LoadReplacementMap() (*Map, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	path := dir + "/replace.yml"
	_, err = os.Stat(path)
	if err != nil {
		return nil, err
	}

	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	m := Map{}
	err = yaml.Unmarshal(buf, &m)
	if err != nil {
		return nil, err
	}

	return &m, nil
}
