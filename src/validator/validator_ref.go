package validator

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type RefRule struct {
}

func init() {
	Instance().RegisterHandler("ref", &RefRule{})
}

func (r *RefRule) CheckRuleFormat(src, cmd, dest string) error {
	return nil
}

func (r *RefRule) VerifyRule(v *Validator, rule Rule) error {
	fields := strings.Split(rule.src, ".")
	table := v.tables[fields[0]]
	if table == nil {
		return errors.New("not found table data " + fields[0])
	}
	dstFields := strings.Split(rule.dst, ".")
	/*dstTable, ok := v.tables[dstFields[0]]
	if !ok {
		return errors.New("dst table not exist " + rule.dst)
	}
	*/
	for _, row := range table {

		fv, err := v.GetFieldValue(fields[1:], row)
		if err != nil {
			return err
		}

		if err := verifyField(v, dstFields, fields[2:], fv); err != nil {
			return fmt.Errorf("table:%v id:%v ref fail. %v %v. err:%v", fields[0], row["Id"], rule.src, fv, err)
		}
		/*
			rf := reflect.ValueOf(fv)
			fmt.Println(rf.Kind(), fields[1:])
			if arr, ok := fv.([]interface{}); ok {
				for _, sfv := range arr {
					if !keyExists(dstTable, dstFields[1:], sfv) {
						return fmt.Errorf("table:%v id:%v ref fail. %v %v", fields[0], row["Id"], rule.src, sfv)
					}
				}
			} else {
				sfv := fv
				if !keyExists(dstTable, dstFields[1:], sfv) {
					return fmt.Errorf("table:%v id:%v ref fail. %v %v", fields[0], row["Id"], rule.src, sfv)
				}
			}
		*/

	}
	return nil
}

func verifyField(v *Validator, refFields []string, fields []string, fv interface{}) error {
	rf := reflect.ValueOf(fv)
	switch rf.Kind() {
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		arr := fv.([]interface{})
		for _, subfv := range arr {
			if err := verifyField(v, refFields, fields[1:], subfv); err != nil {
				return fmt.Errorf("%v ref %v fail. %v", fields, refFields, err)
			}
		}
		return nil
	case reflect.Ptr:
		return verifyField(v, refFields, fields, rf.Elem().Interface())
	case reflect.Struct:
		sfv, err := getSubFieldValue(fields[0:], rf)
		if err != nil {
			return fmt.Errorf("get field %v fail. %v", fields[0], err)
		}
		return verifyField(v, refFields, fields[1:], sfv)
	case reflect.Map:
		sfv, err := getSubFieldValue(fields[0:], rf)
		if err != nil {
			return fmt.Errorf("get field %v fail. %v", fields[0], err)
		}
		return verifyField(v, refFields, fields[1:], sfv)

	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		refTable, ok := v.tables[refFields[0]]
		if !ok {
			return errors.New("ref table not exist " + refFields[0])
		}
		if !keyExists(refTable, refFields[1:], fv) {
			return fmt.Errorf("table:%v id:%v ref fail. key not exists", refFields[0], fv)
		}
		return nil
	}
	return fmt.Errorf("verifyfail")
}

func keyExists(table map[interface{}]map[string]interface{}, fields []string, key interface{}) bool {
	// TODO: 暂时只支持 id 外键
	if _, ok := table[key]; ok {
		return true
	} else {
		return false
	}
}
