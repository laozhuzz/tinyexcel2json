package main

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
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
	for i := 0; i < len(v.rules); i++ {
		if err := v.verifyRule(v.rules[i]); err != nil {
			return err
		}
	}
	return nil
}
func (v *Validator) verifyRule(rule Rule) error {
	fmt.Println("verify rule: " + rule.src + " " + rule.cmd + " " + rule.dst)
	switch rule.cmd {
	case "ref":
		return v.verifyRule_Ref(rule)
	default:
		return errors.New("unimplement cmd " + rule.cmd)
	}
}

func (v *Validator) verifyRule_Ref(rule Rule) error {
	fields := strings.Split(rule.src, ".")
	table := v.tables[fields[0]]
	if table == nil {
		return errors.New("not found table data " + fields[0])
	}
	for _, row := range table {
		if fv, err := v.getFieldValue(fields[1:], row); err != nil {
			return err
		} else {
			dstFields := strings.Split(rule.dst, ".")
			if dstTable, ok := v.tables[dstFields[0]]; !ok {
				return errors.New("dst table not exist " + rule.dst)
			} else {
				if arr, ok := fv.([]interface{}); ok {
					for _, sfv := range arr {
						if !v.keyExists(dstTable, dstFields[1:], sfv) {
							return fmt.Errorf("table:%v id:%v ref fail. %v %v", fields[0], row["Id"], rule.src, sfv)
						}
					}
				} else {
					if !v.keyExists(dstTable, dstFields[1:], fv) {
						return fmt.Errorf("table:%v id:%v ref fail. %v %v", fields[0], row["Id"], rule.src, fv)
					}
				}
			}
		}
	}

	return nil
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

func (v *Validator) keyExists(table map[interface{}]map[string]interface{}, fields []string, key interface{}) bool {
	// TODO: 暂时只支持 id 外键
	if _, ok := table[key]; ok {
		return true
	} else {
		return false
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
