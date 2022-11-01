package validator

import (
	"errors"
	"fmt"
	"strings"
)

type RefRule struct {
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
