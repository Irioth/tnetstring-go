package tnetstring

import (
	"os"
	"reflect"
	"strconv"
	"strings"
)

func Unmarshal(data string, v interface{}) os.Error {
	val := reflect.ValueOf(v)
	val = reflect.Indirect(val)
	if !val.CanSet() {
		return os.NewError("tnetstring: Unmarshal requires a settable value")
	}
	_, err := unmarshal(data, val)
	return err
}

func indirect(v reflect.Value, create bool) reflect.Value {
	for {
		switch v.Kind() {
		case reflect.Ptr:
			if create && v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		case reflect.Interface:
			if create && v.IsNil() {
				return v
			}
			v = v.Elem()
		default:
			return v
		}
	}
	panic("unreachable")
}

func unmarshal(data string, v reflect.Value) (int, os.Error) {
	typ, content, n := readElement(data)
	if n == 0 {
		return 0, os.NewError("tnetstring: invalid data")
	}
	v = indirect(v, true)
	kind := v.Kind()
	// ~ and interface types are special cases
	if typ != '~' && kind != reflect.Interface && typeLookup[kind] != typ {
		return 0, os.NewError("tnetstring: invalid value to unmarshal into")
	}
	switch typ {
	case '!':
		v.Set(reflect.ValueOf(content == "true"))
	case '#':
		switch kind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			i, err := strconv.Atoi64(content)
			if err != nil {
				return 0, err
			}
			v.SetInt(i)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
			reflect.Uint64, reflect.Uintptr:
			ui, err := strconv.Atoui64(content)
			if err != nil {
				return 0, err
			}
			v.SetUint(ui)
		case reflect.Interface:
			i, err := strconv.Atoi64(content)
			if err != nil {
				return 0, err
			}
			v.Set(reflect.ValueOf(i))
		}
	case ',':
		v.Set(reflect.ValueOf(content))
	case ']':
		unmarshalArray(content, v)
	case '}':
		var err os.Error
		if kind == reflect.Map {
			err = unmarshalMap(content, v)
		} else {
			err = unmarshalStruct(content, v)
		}
		if err != nil {
			return 0, err
		}
	case '~':
		switch kind {
		case reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
			v.Set(reflect.Zero(v.Type()))
		default:
			return 0, os.NewError("tnetstring: invalid value to unmarshal into")
		}
	default:
		return 0, os.NewError("tnetstring: unknown type")
	}
	return n, nil
}

func unmarshalArray(data string, v reflect.Value) os.Error {
	kind := v.Kind()
	n := 0
	i := 0
	elType := v.Type().Elem()
	elVal := reflect.Zero(elType)
	for len(data)-n > 0 {
		if i >= v.Len() {
			if kind == reflect.Array {
				break
			} else {
				v.Set(reflect.Append(v, elVal))
			}
		}
		el := v.Index(i)
		i++
		nn, err := unmarshal(data[n:], el)
		if err != nil {
			return err
		}
		n += nn
	}
	return nil
}

func unmarshalMap(data string, v reflect.Value) os.Error {
	if v.Type().Key().Kind() != reflect.String {
		return os.NewError("tnetstring: only maps with string keys can be unmarshaled")
	}
	if v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}
	n := 0
	vtype := v.Type().Elem()
	var s string
	key := reflect.ValueOf(&s).Elem()
	val := reflect.New(vtype).Elem()
	for len(data)-n > 0 {
		typ, content, nn := readElement(data[n:])
		if typ != ',' {
			return os.NewError("tnetstring: non-string key in dictionary")
		}
		s = content
		n += nn
		nn, err := unmarshal(data[n:], val)
		if err != nil {
			return err
		}
		n += nn
		v.SetMapIndex(key, val)
	}
	return nil
}

func unmarshalStruct(data string, v reflect.Value) os.Error {
	n := 0
	structType := v.Type()
	var name string
	for len(data)-n > 0 {
		typ, content, nn := readElement(data[n:])
		if typ != ',' {
			return os.NewError("tnetstring: non-string key in dictionary")
		}
		name = content
		n += nn
		field := v.FieldByName(name)
		if field.Internal == nil {
			for i := 0; i < structType.NumField(); i++ {
				f := structType.Field(i)
				if f.Tag == name {
					field = v.Field(i)
					break
				}
			}
			if field.Internal == nil {
				// skip the field
				_, _, nn := readElement(data[n:])
				n += nn
				continue
			}
		}
		nn, err := unmarshal(data[n:], field)
		if err != nil {
			return err
		}
		n += nn
	}
	return nil
}

func readElement(data string) (typ byte, content string, n int) {
	col := strings.Index(data, ":")
	if col < 1 {
		return
	}
	n, err := strconv.Atoi(data[:col])
	if err != nil || n > len(data[col+1:]) {
		return
	}
	// +1 for colon
	n += col + 1
	content = data[col+1 : n]
	typ = data[n]
	n++
	return
}
