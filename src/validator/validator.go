package validator

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

var (
	once     sync.Once
	instance *Validator
)

type IRuleHandler interface {
	CheckRuleFormat(src, cmd, dest string) error
	VerifyRule(v *Validator, rule Rule) error
}

type Rule struct {
	cmd string
	src string
	dst string
}

type Validator struct {
	rules       []Rule
	ruleHandler map[string]IRuleHandler
	tables      map[string]map[interface{}]map[string]interface{}
}

func Instance() *Validator {
	once.Do(func() {
		instance = &Validator{
			rules:       []Rule{},
			ruleHandler: map[string]IRuleHandler{},
			tables:      map[string]map[interface{}]map[string]interface{}{},
		}

	})
	return instance
}
func (v *Validator) RegisterHandler(cmd string, h IRuleHandler) {
	v.ruleHandler[cmd] = h
}

func (v *Validator) AddRule(src, cmd, dest string) error {
	h := v.ruleHandler[cmd]
	if h == nil {
		return errors.New(cmd + " cmd not supported.")
	}
	if err := h.CheckRuleFormat(src, cmd, dest); err != nil {
		return err
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
	for i := 0; i < len(v.rules); i++ {
		if err := v.verifyRule(v.rules[i]); err != nil {
			return err
		}
	}
	return nil
}

func (v *Validator) verifyRule(rule Rule) error {
	fmt.Println("verify rule: " + rule.src + " " + rule.cmd + " " + rule.dst)
	h := v.ruleHandler[rule.cmd]
	if h == nil {
		return errors.New("unimplement cmd " + rule.cmd)
	}
	return h.VerifyRule(v, rule)
}

func (v *Validator) getFieldValue(fields []string, row map[string]interface{}) (interface{}, error) {
	if fv, ok := row[fields[0]]; !ok {
		return nil, errors.New("field " + fields[0] + " not exist")
	} else {
		if len(fields) > 1 {
			cur := reflect.ValueOf(fv)
			return getSubFieldValue(fields[1:], cur)
		} else {
			return fv, nil
		}
	}
}

func getSubFieldValue(fields []string, fv reflect.Value) (interface{}, error) {
	cur := fv
	if cur.Kind() == reflect.Ptr {
		cur = cur.Elem()
	}
	if cur.Kind() == reflect.Interface {
		cur = cur.Elem()
	}
	//fmt.Printf("%v:%v %v", fields, cur.Kind().String(), fv)

	switch cur.Kind() {
	case reflect.Slice:
		fallthrough
	case reflect.Array:
		res := make([]interface{}, 0, cur.Len())
		for i := 0; i < cur.Len(); i++ {
			v, err := getSubFieldValue(fields[0:], cur.Index(i))
			if err != nil {
				return nil, err
			}
			res = append(res, v)
		}
		return (interface{})(res), nil
	case reflect.Map:
		sfv := cur.MapIndex(reflect.ValueOf(fields[0]))
		if !sfv.IsValid() {
			return nil, fmt.Errorf("field %v not found", fields[0])
		}
		return sfv.Interface(), nil
	case reflect.Struct:
		sfv := reflect.Indirect(cur).FieldByName(fields[0])
		if !sfv.IsValid() {
			return nil, fmt.Errorf("field %v not found", fields[0])
		}
		return sfv.Interface(), nil

	default:
	}
	return nil, nil
}
