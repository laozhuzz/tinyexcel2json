package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	json "github.com/json-iterator/go"
	"github.com/laozhuzz/excel2json/validator"
	"github.com/tealeg/xlsx/v3"
)

//
// 配置表 支持子message, 支持array
//

type TokenState uint8

const (
	State_None     = TokenState(0)
	State_Set      = TokenState(1)
	State_ArrBegin = TokenState(2)
	State_ArrEnd   = TokenState(4)
	State_MsgBegin = TokenState(8)
	State_MsgEnd   = TokenState(16)
	State_SetArr   = TokenState(32)
)

type NestedFieldDesc struct {
	name  string
	state TokenState
}
type FieldDesc struct {
	FieldName   string
	ValueType   string
	NestedField []NestedFieldDesc
}

type RowData struct {
	Fields []string
}
type TableData struct {
	sheet      *xlsx.Sheet
	header     map[string]*RowData
	rows       []*RowData
	rowDesc    []*FieldDesc
	parsedData map[interface{}]map[string]interface{}
	curRow     int
	curColumn  int
}
type PostSetData struct {
	node    interface{}
	key     string
	subNode interface{}
}

func (t *TableData) ReadXlsxSheet() error {

	if err := t.readXlsxHeader(); err != nil {
		return err
	}
	if err := t.readXlsxBody(); err != nil {
		return err
	}
	return nil
}

func (t *TableData) readXlsxHeader() error {
	sheet := t.sheet
	if sheet.MaxCol <= 1 {
		return t.Error("empty column by" + sheet.Name)
	}
	for rowi := 0; rowi < sheet.MaxRow; rowi++ {
		curRow := make([]string, sheet.MaxCol)
		for coli := 0; coli < sheet.MaxCol; coli++ {
			value := getCelValue(sheet, rowi, coli)
			curRow[coli] = value
		}
		if strings.HasPrefix(curRow[0], "##") {
			t.header[curRow[0]] = &RowData{
				Fields: curRow,
			}
		} else {
			break
		}
	}
	nameRow := t.header["##name"]
	if nameRow == nil {
		return t.Error("missed ##name row " + sheet.Name)
	}
	typeRow := t.header["##type"]
	if typeRow == nil {
		return t.Error("missed ##type row " + sheet.Name)
	}
	arrCharCount := 0
	subMsgCharCount := 0
	for i, v := range nameRow.Fields {
		// 第一格为##name
		if i == 0 {
			t.rowDesc = append(t.rowDesc, &FieldDesc{})
			continue
		}
		fieldDesc := &FieldDesc{}
		fieldDesc.FieldName = strings.TrimSpace(v)
		arrCharCount += strings.Count(v, "[")
		arrCharCount -= strings.Count(v, "]")
		subMsgCharCount += strings.Count(v, "{")
		subMsgCharCount -= strings.Count(v, "}")

		valueType := typeRow.Fields[i]
		if !isValueTypeValid(valueType) {
			return t.Error("invalid valueType " + valueType + " in sheet " + sheet.Name)
		}
		fieldDesc.ValueType = valueType

		if err := parseNestedFieldDesc(fieldDesc); err != nil {
			return err
		}
		t.rowDesc = append(t.rowDesc, fieldDesc)
	}
	if arrCharCount != 0 {
		return t.Error("mismatch []" + " in sheet " + sheet.Name)
	}
	if subMsgCharCount != 0 {
		return t.Error("mismatch {}" + " in sheet " + sheet.Name)
	}
	validatorRow := t.header["##validator"]
	if validatorRow != nil {
		for i, v := range validatorRow.Fields {
			if v == "" || strings.HasPrefix(v, "##") {
				continue
			}

			src := make([]string, 0, 8)
			src = append(src, t.sheet.Name)
			for j := 1; j < i; j++ {
				fieldDesc := t.rowDesc[j]
				emptyName := 0
				for k := 0; k < len(fieldDesc.NestedField); k++ {
					nestFieldDesc := fieldDesc.NestedField[k]

					switch nestFieldDesc.state {
					case State_ArrBegin:
						fallthrough
					case State_MsgBegin:
						if nestFieldDesc.name != "" {
							src = append(src, nestFieldDesc.name)
						} else {
							emptyName++
						}
					case State_ArrEnd:
						fallthrough
					case State_MsgEnd:
						if nestFieldDesc.name != "" {
							src = src[0 : len(src)-1]
						} else {
							if emptyName > 0 {
								emptyName--
							} else {
								src = src[0 : len(src)-1]
							}
						}
					}
				}
			}
			fieldDesc := t.rowDesc[i]
			for i := 0; i < len(fieldDesc.NestedField); i++ {
				/*
					if i > 0 && fieldDesc.NestedField[i].name == fieldDesc.NestedField[i-1].name &&
						fieldDesc.NestedField[i-1].state == State_ArrBegin {
						continue
					}
				*/
				if fieldDesc.NestedField[i].name != "" {
					src = append(src, fieldDesc.NestedField[i].name)
				}
			}
			cmd := strings.Split(v, "=")
			if len(cmd) != 2 {
				return t.Error("validator format err " + fieldDesc.FieldName)
			}

			if err := validator.Instance().AddRule(strings.Join(src, "."), cmd[0], cmd[1]); err != nil {
				return t.Error(err.Error() + fieldDesc.FieldName)
			}
		}
	}

	return nil
}

func (t *TableData) readXlsxBody() error {
	sheet := t.sheet
	t.rows = make([]*RowData, 0, t.sheet.MaxRow)

	for rowi := len(t.header); rowi < sheet.MaxRow; rowi++ {
		curRow := make([]string, sheet.MaxCol)
		for coli := 0; coli < sheet.MaxCol; coli++ {
			value := getCelValue(sheet, rowi, coli)
			// 移除前后空白
			curRow[coli] = strings.TrimSpace(value)
		}
		if strings.HasPrefix(curRow[0], "##") {
			return t.Error("desc row " + curRow[0] + " should be the top of a sheet " + sheet.Name)
		}
		t.rows = append(t.rows, &RowData{Fields: curRow})
	}
	if err := t.parseTableData(); err != nil {
		return err
	}
	return nil
}

func (t *TableData) parseTableData() error {
	if t.parsedData == nil {
		t.parsedData = make(map[interface{}]map[string]interface{})
	}
	for k := range t.rows {
		if r, err := t.parseRowData(k); err != nil {
			return err
		} else {
			t.parsedData[r["Id"].(int32)] = r
		}
	}

	return nil
}

func (t *TableData) parseRowData(rowi int) (map[string]interface{}, error) {
	row := t.rows[rowi]
	parsed := map[string]interface{}{}
	objStack := []*PostSetData{}
	var curObj interface{}

	t.curRow = rowi + 1 + len(t.header)
	curObj = parsed
	for k1, v1 := range row.Fields {
		// 忽略第一列 ##name
		if k1 == 0 {
			continue
		}
		t.curColumn = k1
		desc := t.rowDesc[k1]

		for _, v2 := range desc.NestedField {
			switch v2.state {
			case State_Set:
				// 支持空值,
				if v1 == "" {
					// 限制只能是array中消息为空, 或者array字段为空; 比如奖励多个物品, 有的奖励5个, 有个4个, 这个时候就有一个为空
					if len(objStack) > 0 {
						pdata := objStack[len(objStack)-1]
						if _, ok := pdata.node.(*[]interface{}); ok {
							continue
						}
						if _, ok := pdata.subNode.(*[]interface{}); ok {
							continue
						}
					}
					// 字符串支持空值
					if desc.ValueType == "string" {
						continue
					}
					return nil, t.Error("value not set. column " + desc.FieldName + " row " + strconv.Itoa(rowi+1+len(t.header)))
				}
				pv, err := parseFieldValue(v1, desc.ValueType)
				if err != nil {
					return nil, err
				}
				if err := setCurValue(curObj, v2.name, pv); err != nil {
					return nil, err
				}
			case State_SetArr:
				// 支持空arr
				if v1 == "" {
					continue
				}
				if !strings.HasPrefix(v1, "[") || !strings.HasSuffix(v1, "]") {
					return nil, t.Error("arrValue invalid. column " + desc.FieldName + " row " + strconv.Itoa(rowi+1+len(t.header)))
				}
				strArr := strings.Split(v1[1:len(v1)-1], ",")
				for _, sv := range strArr {
					sv = strings.TrimSpace(sv)
					if sv == "" {
						continue
					}
					pv, err := parseFieldValue(sv, desc.ValueType)
					if err != nil {
						return nil, err
					}
					if err := setCurValue(curObj, v2.name, pv); err != nil {
						return nil, err
					}
				}

			case State_ArrBegin:
				arr := []interface{}{}
				subObj := &arr
				objStack = append(objStack, &PostSetData{node: curObj, key: v2.name, subNode: subObj})
				curObj = subObj
			case State_MsgBegin:
				subObj := map[string]interface{}{}
				objStack = append(objStack, &PostSetData{node: curObj, key: v2.name, subNode: subObj})
				curObj = subObj
			case State_ArrEnd:
				fallthrough
			case State_MsgEnd:
				pdata := objStack[len(objStack)-1]
				setCurValue(pdata.node, pdata.key, curObj)
				objStack = objStack[:len(objStack)-1]
				curObj = pdata.node

			}
		}
	}
	// need Id column
	if _, ok := parsed["Id"]; !ok {
		return nil, t.Error("missed Id column")
	}

	return parsed, nil
}

func (t *TableData) Error(format string, a ...interface{}) error {
	fieldName := ""
	if t.curColumn < len(t.rowDesc) {
		fieldName = t.rowDesc[t.curColumn].FieldName
	}
	prefix := fmt.Sprintf("data row %v column %v %v:", t.curRow, t.curColumn, fieldName)
	errstr := fmt.Sprintf(format, a...)
	return errors.New(prefix + errstr)
}

func setCurValue(node interface{}, k string, v interface{}) error {
	if node == nil {
		return errors.New("curObj both nil")
	}
	// 空子消息忽略, 支持空
	if mv, ok := v.(map[string]interface{}); ok {
		if len(mv) == 0 {
			return nil
		}
	}

	if m, ok := node.(map[string]interface{}); ok {
		if k == "" {
			return errors.New("field name empty")
		}
		m[k] = v
		return nil
	} else if a, ok := node.(*[]interface{}); ok {
		*a = append(*a, v)
		return nil
	}
	return errors.New("unknonw curObj type")

}

func (t *TableData) ExportJson(w io.Writer) error {
	data, err := json.MarshalIndent(t.parsedData, "", " ")
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
	/*
		c := json.Config{
			SortMapKeys: true,
			//EscapeHTML:  true,
			IndentionStep: 1,
			//ValidateJsonRawMessage: true,
		}

		e := c.Froze().NewEncoder(w)
		if err := e.Encode(t.parsedData); err != nil {
			return err
		}
		return nil
	*/
}

func parseNestedFieldDesc(desc *FieldDesc) error {
	start := 0
	for cur := 0; start < len(desc.FieldName); cur++ {
		if cur < len(desc.FieldName) {
			switch desc.FieldName[cur] {
			case '[':
				name := strings.TrimSpace(desc.FieldName[start:cur])
				start = cur + 1
				desc.NestedField = append(desc.NestedField, NestedFieldDesc{name: name, state: State_ArrBegin})
				// 以[字符结束时, 需要增加一个set
				if start == len(desc.FieldName) {
					desc.NestedField = append(desc.NestedField, NestedFieldDesc{name: name, state: State_Set})
				}
				// 在同一行 读取arr
				if len(desc.FieldName) > start && desc.FieldName[start] == ']' {
					desc.NestedField = append(desc.NestedField, NestedFieldDesc{name: name, state: State_SetArr})
				}
			case '{':
				name := strings.TrimSpace(desc.FieldName[start:cur])
				start = cur + 1
				desc.NestedField = append(desc.NestedField, NestedFieldDesc{name: name, state: State_MsgBegin})
			case ']':
				name := strings.TrimSpace(desc.FieldName[start:cur])
				start = cur + 1
				// 以]字符开头时, 需要增加一个set
				if len(name) > 0 || start == 1 {
					desc.NestedField = append(desc.NestedField, NestedFieldDesc{name: name, state: State_Set})
				}
				desc.NestedField = append(desc.NestedField, NestedFieldDesc{name: "", state: State_ArrEnd})
			case '}':
				name := strings.TrimSpace(desc.FieldName[start:cur])
				start = cur + 1
				if len(name) > 0 {
					desc.NestedField = append(desc.NestedField, NestedFieldDesc{name: name, state: State_Set})
				}
				desc.NestedField = append(desc.NestedField, NestedFieldDesc{name: "", state: State_MsgEnd})
			default:

			}
		} else {
			name := strings.TrimSpace(desc.FieldName[start:])
			start = len(desc.FieldName)
			desc.NestedField = append(desc.NestedField, NestedFieldDesc{name: name, state: State_Set})
		}

	}

	return nil
}

func getCelValue(sheet *xlsx.Sheet, row int, col int) string {
	if cell, err := sheet.Cell(row, col); err != nil {
		panic(err)
	} else {
		if value, err := cell.FormattedValue(); err != nil {
			panic(err)
		} else {
			return value
		}
	}
}

func isValueTypeValid(valueType string) bool {
	switch valueType {
	case "string":
		fallthrough
	case "int": // treat as int32
		fallthrough
	case "int32":
		fallthrough
	case "int64":
		fallthrough
		//case "float":
		//fallthrough
		//case "double":
		//fallthrough
	case "bool":
		return true
	default:
		return false
	}
}
func parseFieldValue(value string, valueType string) (interface{}, error) {
	switch valueType {
	case "string":
		return value, nil
	case "int": // treat as int32
		fallthrough
	case "int32":
		v, err := strconv.ParseInt(value, 10, 64)
		return int32(v), err
	case "int64":
		return strconv.ParseInt(value, 10, 64)
	case "float":
		fallthrough
	case "double":
		return strconv.ParseFloat(value, 64)
	case "bool":
		return strconv.ParseBool(value)
	default:
		return nil, errors.New("unsupport type " + valueType)
	}
}

func ConvertDir(inputDir string, output string) error {

	err := filepath.WalkDir(inputDir, func(path string, f fs.DirEntry, err error) error {
		file := filepath.Base(path)
		if strings.HasSuffix(strings.ToLower(file), ".xlsx") && !strings.HasPrefix(file, "~") {
			return ConvertFile(path, output)
		}
		return nil
	})
	return err
}

func ConvertFile(filename string, output string) error {
	wb, err := xlsx.OpenFile(filename)
	if err != nil {
		panic(err)
	}

	for _, sheet := range wb.Sheets {
		if !strings.HasSuffix(sheet.Name, "Config") && !strings.HasSuffix(sheet.Name, "Cfg") {
			continue
		}
		fmt.Printf("convert file %v sheet %v\n", filename, sheet.Name)
		tableData := &TableData{
			sheet:  sheet,
			header: map[string]*RowData{},
		}
		if err := tableData.ReadXlsxSheet(); err != nil {
			panic("error:" + filename + ":" + err.Error())
		}
		// set output
		outputfile := filepath.Join(output, sheet.Name+".json")
		f, err := os.OpenFile(outputfile, os.O_CREATE|os.O_TRUNC|os.O_RDWR, fs.ModePerm)
		if err != nil {
			panic(err)
		}
		if err := tableData.ExportJson(f); err != nil {
			panic("error:" + filename + ":" + err.Error())
		}
		validator.Instance().AddTableData(sheet.Name, tableData.parsedData)
	}
	return nil
}

func main() {
	flagInput := flag.String("i", "./excel", "input excel folder")
	flagOutput := flag.String("o", "./outjson", "output json folder")
	flag.Parse()

	ifs, err := os.Stat(*flagInput)
	if err != nil {
		fmt.Printf("read %v error. %v", *flagInput, err)
		os.Exit(-1)
	}

	fullOutput := filepath.Join(".", *flagOutput)
	if ofs, err := os.Stat(*flagOutput); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(fullOutput, os.ModePerm); err != nil {
				fmt.Printf("open %v error. %v", *flagOutput, err)
				os.Exit(-1)
			}
		} else {
			fmt.Printf("open %v error. %v", *flagOutput, err)
			os.Exit(-1)
		}
	} else {
		if !ofs.IsDir() {
			fmt.Printf("output %v should be folder", *flagOutput)
			os.Exit(-1)
		}
	}

	if ifs.IsDir() {
		if err := ConvertDir(*flagInput, fullOutput); err != nil {
			panic(err)
		}
	} else {
		if err := ConvertFile(*flagInput, fullOutput); err != nil {
			panic(err)
		}
	}

	if err := validator.Instance().Validate(); err != nil {
		panic(err)
	} else {
		fmt.Println("validator verify succ.")
	}

	fmt.Println("convert finish.")
}
