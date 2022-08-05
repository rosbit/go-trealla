package trealla

import (
	"reflect"
	"fmt"
	"bytes"
	"encoding/json"
)

type Term = string

type PlArg interface {
	ToTerm() Term
}

var (
	_ PlArg = PlBool(false)
	_ PlArg = PlInt(0)
	_ PlArg = PlFloat(0.0)
	_ PlArg = PlString("")
	_ PlArg = PlVar("")
	_ PlArg = PlStrTerm("[]")
	_ PlArg = &PlList{}
	_ PlArg = PlAtom("")
)

func makePlTerm(v interface{}) (Term, error) {
	if v == nil {
		return "[]", nil
	}

	switch vv := v.(type) {
	case int, int8, int16, int32, int64,
	     uint,uint8,uint16,uint32,uint64:
		return makeInt(v), nil
	case string:
		return PlString(vv).ToTerm(), nil
	case bool:
		return PlBool(vv).ToTerm(), nil
	case float64:
		return PlFloat(vv).ToTerm(), nil
	case float32:
		return PlFloat(float64(vv)).ToTerm(), nil
	case PlAtom:
		return vv.ToTerm(), nil
	case PlVar:
		return vv.ToTerm(), nil
	case PlStrTerm:
		return vv.ToTerm(), nil
	case Record:
		return makeRecordTerm(vv)
	default:
	}

	vv := reflect.ValueOf(v)
	switch vv.Kind() {
	case reflect.Slice:
		t := vv.Type()
		if t.Elem().Kind() == reflect.Uint8 {
			return PlString(string(v.([]byte))).ToTerm(), nil
		}
		fallthrough
	case reflect.Array:
		if plL, err := newPlList(v); err != nil {
			return "[]", err
		} else {
			return plL.ToTerm(), nil
		}
	case reflect.Ptr:
		switch vv.Elem().Kind() {
		case reflect.Array, reflect.Struct:
			if plL, err := newPlList(v); err != nil {
				return "[]", err
			} else {
				return plL.ToTerm(), nil
			}
		}
		return makePlTerm(vv.Elem().Interface())
	/*
	case reflect.Map:
	case reflect.Struct:
	case reflect.Func:
	*/
	default:
		return "", fmt.Errorf("unsupported type %v", vv.Kind())
	}
}

func makeInt(i interface{}) Term {
	switch i.(type) {
	case int,int8,int16,int32,int64:
		return PlInt(reflect.ValueOf(i).Int()).ToTerm()
	case uint8,uint16,uint32:
		return PlInt(int64(reflect.ValueOf(i).Uint())).ToTerm()
	case uint,uint64:
		return PlFloat(float64(reflect.ValueOf(i).Uint())).ToTerm()
	default:
		return "0"
	}
}

// var
type PlVar string
func (v PlVar) ToTerm() Term {
	return string(v)
}

// bool
type PlBool bool
func (b PlBool) ToTerm() Term {
	if b {
		return "true"
	}
	return "false"
}

// int
type PlInt int64
func (i PlInt) ToTerm() Term {
	return fmt.Sprintf("%d", int64(i))
}

// float
type PlFloat float64
func (f PlFloat) ToTerm() Term {
	return fmt.Sprintf("%f", float64(f))
}

// string
type PlString string
func (s PlString) ToTerm() Term {
	b, _ := json.Marshal(string(s))
	return string(b)
}

// string -> term
type PlStrTerm string
func (s PlStrTerm) ToTerm() Term {
	str := string(s)
	if len(str) == 0 {
		return `""`
	}
	return str
}

type PlAtom string
func (a PlAtom) ToTerm() Term {
	s := string(a)
	if len(s) == 0 {
		return "[]"
	}

	c := s[0]
	allIdChar := (c >= 'a' && c <= 'z') || (c == '_')
	if !allIdChar {
		goto EXIT
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || (c >= '0' && c <= '9')) {
			allIdChar = false
			break
		}
	}
	if allIdChar {
		return s
	}
EXIT:
	return fmt.Sprintf(`'%s'`, a)
}

// list
type PlList struct {
	pa []interface{}
}
func newPlList(a interface{}) (plL *PlList, err error) {
	if a == nil {
		plL = &PlList{[]interface{}{}}
		return
	}
	ref := reflect.ValueOf(a)
	l := ref.Len()
	r := make([]interface{}, l)
	for i:=0; i<l; i++ {
		r[i] = ref.Index(i).Interface()
	}
	plL = &PlList{r}
	return
}
func (a *PlList) ToTerm() Term {
    b := &bytes.Buffer{}
    fmt.Fprintf(b, "[")
    for i, f := range a.pa {
        if i > 0 {
            fmt.Fprintf(b, ",")
        }
		ft, _ := makePlTerm(f)
        fmt.Fprintf(b, "%s", ft)
    }
    fmt.Fprintf(b, "]")
    return b.String()
}

func makeRecordTerm(r Record) (Term, error) {
	b := &bytes.Buffer{}
	fmt.Fprintf(b, "%s(", r.TableName())
	for i, f := range r.FieldValues() {
		if i > 0 {
			fmt.Fprintf(b, ",")
		}
		t, err := makePlTerm(f)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(b, "%s", t)
	}
	fmt.Fprintf(b, ")")
	return b.String(), nil
}
