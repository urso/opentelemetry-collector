package elasticsearchexporter

import (
	"bytes"
	"sort"
	"strings"
	"time"

	"github.com/elastic/go-structform"
	"github.com/elastic/go-structform/json"
	"go.opentelemetry.io/collector/consumer/pdata"
)

type object struct {
	fields []field
}

type field struct {
	key   string
	value value
}

type value struct {
	kind      kind
	primitive uint64
	dbl       float64
	str       string
	arr       []value
	obj       object
	ts        time.Time
}

type kind uint8

const (
	kindNil kind = iota
	kindBool
	kindInt
	kindDouble
	kindString
	kindArr
	kindObj
	kindTimestamp
	kindIgnore
)

const tsLayout = "2006-01-02T15:04:05.000000000Z"

func (obj *object) AddTimestamp(key string, ts time.Time) {
	obj.Add(key, value{kind: kindTimestamp, ts: ts})
}

func (obj *object) Add(key string, v value) {
	obj.fields = append(obj.fields, field{key: key, value: v})
}

func (obj *object) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	v := json.NewVisitor(&buf)
	obj.iterJSON(v)
	return buf.Bytes(), nil
}

func (obj *object) iterJSON(w *json.Visitor) {
	objPrefix := ""
	level := 0

	w.OnObjectStart(-1, structform.AnyType)
	defer w.OnObjectFinished()

	for i := range obj.fields {
		fld := &obj.fields[i]
		if fld.value.kind == kindIgnore ||
			fld.value.kind == kindNil ||
			(fld.value.kind == kindArr && len(fld.value.arr) == 0) {
			continue
		}

		key := fld.key
		// decrease object level until last reported and current key have the same path prefix
		for L := commonPrefix(key, objPrefix); L < len(objPrefix); {
			for L > 0 && key[L-1] != '.' {
				L--
			}

			// remove levels and append write list of outstanding '}' into the writer
			if L > 0 {
				for delta := objPrefix[L:]; len(delta) > 0; {
					idx := strings.IndexRune(delta, '.')
					if idx < 0 {
						break
					}

					delta = delta[idx+1:]
					level--
					w.OnObjectFinished()
				}
			} else { // no common prefix, close all objects we reported so far.
				for ; level > 0; level-- {
					w.OnObjectFinished()
				}
				objPrefix = ""
			}
		}

		// increase object level up to current field
		for {
			start := len(objPrefix)
			idx := strings.IndexRune(key[start:], '.')
			if idx < 0 {
				break
			}

			level++
			objPrefix = key[:len(objPrefix)+idx+1]
			fieldName := key[start : start+idx]
			w.OnKey(fieldName)
			w.OnObjectStart(-1, structform.AnyType)
		}

		// report value
		fieldName := key[len(objPrefix):]
		w.OnKey(fieldName)
		fld.value.iterJSON(w)
	}

	// close all pending object levels
	for ; level > 0; level-- {
		w.OnObjectFinished()
	}
}

func (v *value) iterJSON(w *json.Visitor) {
	switch v.kind {
	case kindNil:
		w.OnNil()
	case kindBool:
		w.OnBool(v.primitive == 1)
	case kindInt:
		w.OnInt64(int64(v.primitive))
	case kindDouble:
		w.OnFloat64(v.dbl)
	case kindString:
		w.OnString(v.str)
	case kindTimestamp:
		str := v.ts.UTC().Format(tsLayout)
		w.OnString(str)
	case kindObj:
		if len(v.obj.fields) == 0 {
			w.OnNil()
		} else {
			v.obj.iterJSON(w)
		}
	case kindArr:
		w.OnArrayStart(-1, structform.AnyType)
		for i := range v.arr {
			v.arr[i].iterJSON(w)
		}
	}
}

func (obj *object) dedup() {
	// 1. sort key value pairs, such that duplicate keys are adjacent to each other
	sort.SliceStable(obj.fields, func(i, j int) bool {
		return obj.fields[i].key < obj.fields[j].key
	})

	// 2. rename fields if a primitive value is overwritten by an object.
	//    For example the pair (path.x=1, path.x.a="test") becomes:
	//    (path.x.value=1, path.x.a="test").
	//
	//    This step removes potential conflicts when dedotting and serializing fields.
	for i := 0; i < len(obj.fields)-1; i++ {
		if len(obj.fields[i].key) < len(obj.fields[i+1].key) &&
			strings.HasPrefix(obj.fields[i+1].key, obj.fields[i].key) {
			obj.fields[i].key = obj.fields[i].key + ".value"
		}
	}

	// 3. mark duplicates as 'ignore'
	//
	//    This step ensures that we do not have duplicate fields names when serializing.
	//    Elasticsearch JSON parser will fail otherwise.
	for i := 0; i < len(obj.fields)-1; i++ {
		if obj.fields[i].key == obj.fields[i+1].key {
			obj.fields[i].value.kind = kindIgnore
		}
	}

	// 4. fix objects that might be stored in arrays
	for i := range obj.fields {
		obj.fields[i].value.dedup()
	}
}

func (v *value) dedup() {
	switch v.kind {
	case kindObj:
		v.obj.dedup()
	case kindArr:
		for i := range v.arr {
			v.arr[i].dedup()
		}
	}
}

func objFromAttributes(am pdata.AttributeMap) object {
	fields := make([]field, 0, am.Len())
	fields = appendAttributeFields(fields, "", am)
	return object{fields}
}

func arrFromAttributes(aa pdata.AnyValueArray) []value {
	values := make([]value, aa.Len())
	for i := 0; i < aa.Len(); i++ {
		values[i] = valueFromAttribute(aa.At(i))
	}
	return values
}

func valueFromAttribute(attr pdata.AttributeValue) value {
	switch attr.Type() {
	case pdata.AttributeValueINT:
		return value{kind: kindInt, primitive: uint64(attr.IntVal())}
	case pdata.AttributeValueDOUBLE:
		return value{kind: kindDouble, dbl: attr.DoubleVal()}
	case pdata.AttributeValueSTRING:
		return value{kind: kindString, str: attr.StringVal()}
	case pdata.AttributeValueBOOL:
		var b uint64
		if attr.BoolVal() {
			b = 1
		}
		return value{kind: kindBool, primitive: b}
	case pdata.AttributeValueARRAY:
		sub := arrFromAttributes(attr.ArrayVal())
		return value{kind: kindArr, arr: sub}
	case pdata.AttributeValueMAP:
		sub := objFromAttributes(attr.MapVal())
		return value{kind: kindObj, obj: sub}
	default:
		return value{kind: kindNil}
	}
}

func appendAttributeFields(fields []field, path string, am pdata.AttributeMap) []field {
	am.ForEach(func(k string, val pdata.AttributeValue) {
		fields = appendAttributeValue(fields, path, k, val)
	})
	return fields
}

func appendAttributeValue(fields []field, path string, key string, attr pdata.AttributeValue) []field {
	if attr.Type() == pdata.AttributeValueNULL {
		return fields
	}

	if attr.Type() == pdata.AttributeValueMAP {
		return appendAttributeFields(fields, flattenKey(path, key), attr.MapVal())
	}

	return append(fields, field{
		key:   flattenKey(path, key),
		value: valueFromAttribute(attr),
	})
}

func flattenKey(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func commonPrefix(a, b string) int {
	end := len(a)
	if alt := len(b); alt < end {
		end = alt
	}

	for i := 0; i < end; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return end
}

func stringValue(str string) value {
	return value{kind: kindString, str: str}
}
