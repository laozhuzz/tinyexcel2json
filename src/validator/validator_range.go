package validator

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type RangeRule struct {
}

func init() {
	Instance().RegisterHandler("range", &RangeRule{})
}

func (r *RangeRule) CheckRuleFormat(src, cmd, dest string) error {
	if dest[0] != '[' || dest[len(dest)-1] != ']' {
		return fmt.Errorf("range format error. example: [1,100] means range 1-100")
	}
	nArr := strings.Split(dest[1:len(dest)-1], ",")
	if len(nArr) != 2 {
		return fmt.Errorf("range format error. example: [1,100] means range 1-100")
	}
	for _, s := range nArr {
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("range format error. %v is not valid number", s)
		}
		if i < 0 {
			return fmt.Errorf("range format error. %v should not be negative", s)
		}
	}
	return nil
}

func (r *RangeRule) VerifyRule(v *Validator, rule Rule) error {
	fields := strings.Split(rule.src, ".")
	table := v.tables[fields[0]]
	if table == nil {
		return errors.New("not found table data " + fields[0])
	}
	dst := strings.Split(rule.dst[1:len(rule.dst)-1], ",")
	min, _ := strconv.ParseInt(dst[0], 10, 64)
	max, _ := strconv.ParseInt(dst[1], 10, 64)
	for _, row := range table {
		fv, err := v.GetFieldValue(fields[1:], row)
		if err != nil {
			return err
		}
		if arr, ok := fv.([]interface{}); ok {
			for _, sfv := range arr {
				if err := verifyValue_Range(sfv, min, max); err != nil {
					return fmt.Errorf("table:%v id:%v ref fail. %v %v. err:%v", fields[0], row["Id"], rule.src, sfv, err)
				}
			}
		} else {
			sfv := fv
			if err := verifyValue_Range(sfv, min, max); err != nil {
				return fmt.Errorf("table:%v id:%v ref fail. %v %v. err:%v", fields[0], row["Id"], rule.src, sfv, err)
			}
		}
	}
	return nil
}

func verifyValue_Range(fv interface{}, min int64, max int64) error {
	var v int64
	switch fv := fv.(type) {
	case int:
		v = int64(fv)
	case int32:
		v = int64(fv)
	case int64:
		v = int64(fv)
	case float32:
		v = int64(fv)
	case float64:
		v = int64(fv)

	default:
		return errors.New("range only work on signed number field")

	}
	if v >= min && v <= max {
		return nil
	} else {
		return fmt.Errorf("value %v out of range [%v,%v]", fv, min, max)
	}
}
