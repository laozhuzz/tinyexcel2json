package main

import (
	"errors"
)

var (
	VALIDATOR *Validator
)

type Rule struct {
	cmd string
	src string
	dst string
}

type Validator struct {
	rules  []Rule
	tables map[string]map[interface{}]map[string]interface{}
}

func init() {
	VALIDATOR = &Validator{}
}

func (v *Validator) AddRule(src, cmd, dest string) error {
	if !isValidCmd(cmd) {
		return errors.New(cmd + " cmd not supported.")
	}
	v.rules = append(v.rules, Rule{
		cmd: cmd,
		src: src,
		dst: dest,
	})
	return nil
}
func (v *Validator) AddTableData(name string, parsedData map[interface{}]map[string]interface{}) {
	if v.tables == nil {
		v.tables = make(map[string]map[interface{}]map[string]interface{})
	}
	v.tables[name] = parsedData
}

func (v *Validator) Validate() error {
	if len(v.rules) == 0 {
		return nil
	}
	return nil
}

func isValidCmd(cmd string) bool {
	switch cmd {
	case "ref":
		return true
	case "range":
		return true
	default:
		return false
	}
}
