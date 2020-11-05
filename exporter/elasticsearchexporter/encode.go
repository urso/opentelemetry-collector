package elasticsearchexporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
)

// TODO: create translate package

type logEvent struct {
	Timestamp time.Time        `json:"@timestamp"`
	Message   string           `json:"message"`
	Trace     *traceModel      `json:"trace,omitempty"`
	Span      *spanModel       `json:"span,omitempty"`
	Log       *logModel        `json:"log,omitempty"`
	Fields    attributeMapJSON `json:"fields,omitempty"`
}

type traceModel struct {
	ID string `json:"id,omitempty"`
}

type spanModel struct {
	ID string `json:"id,omitempty"`
}

type logModel struct {
	Level string `json:"level,omitempty"`
}

type attributeMapJSON pdata.AttributeMap

func encodeLogEvent(record pdata.LogRecord) ([]byte, error) {
	var trace *traceModel
	var span *spanModel
	var logDetails *logModel

	if record.TraceID().IsValid() {
		trace = &traceModel{ID: record.TraceID().HexString()}
	}
	if record.SpanID().IsValid() {
		span = &spanModel{ID: record.SpanID().HexString()}
	}
	if record.SeverityText() != "" {
		logDetails = &logModel{
			Level: record.SeverityText(),
		}
	}

	var message string
	if !record.Body().IsNil() && record.Body().Type() == pdata.AttributeValueSTRING {
		message = record.Body().StringVal()
	} else if val, ok := record.Attributes().Get("message"); ok && val.Type() == pdata.AttributeValueSTRING {
		message = val.StringVal()
	}

	return json.Marshal(logEvent{
		Timestamp: time.Unix(0, int64(record.Timestamp())),
		Message:   message,
		Trace:     trace,
		Span:      span,
		Log:       logDetails,

		// TODO: move attributes into intermediate model and de-dot fields.
		Fields: attributeMapJSON(record.Attributes()),
	})
}

func (amj attributeMapJSON) MarshalJSON() ([]byte, error) {
	am := pdata.AttributeMap(amj)
	var buf bytes.Buffer
	_, err := encodeAttributeMap(&buf, 0, am)
	return buf.Bytes(), err
}

func encodeAttributeMap(buf *bytes.Buffer, level int, am pdata.AttributeMap) (n int, err error) {
	initPos := buf.Len()
	buf.WriteRune('{')

	am.ForEach(func(k string, v pdata.AttributeValue) {
		if err != nil {
			return
		}

		// we already extracted message as body, let's ignore it here.
		if level == 0 && k == "message" {
			return
		}

		if v.IsNil() {
			return
		}

		kvPos := buf.Len()
		var added bool
		if n > 0 {
			buf.WriteRune(',')
		}
		fmt.Fprintf(buf, "%q:", k)
		added, err = encodeAttributeVal(buf, level, v, true)
		if !added {
			buf.Truncate(kvPos)
		} else {
			n++
		}
	})
	if err != nil {
		return n, err
	}

	if n == 0 {
		buf.Truncate(initPos)
		return 0, nil
	}

	buf.WriteRune('}')
	return n, nil
}

func encodeAttributeArray(buf *bytes.Buffer, level int, arr pdata.AnyValueArray) (n int, err error) {
	initPos := buf.Len()
	buf.WriteRune('[')

	for i := 0; i < arr.Len(); i++ {
		v := arr.At(i)
		if n == 0 {
			buf.WriteRune(',')
		}

		_, err := encodeAttributeVal(buf, level+1, v, false)
		if err != nil {
			buf.Truncate(initPos)
			return 0, err
		}
	}

	if n == 0 {
		buf.Truncate(initPos)
		return 0, nil
	}

	buf.WriteRune(']')
	return n, nil
}

func encodeAttributeVal(buf *bytes.Buffer, level int, v pdata.AttributeValue, omitEmpty bool) (added bool, err error) {
	switch v.Type() {
	case pdata.AttributeValueNULL:
		buf.WriteString("null")
	case pdata.AttributeValueINT:
		fmt.Fprintf(buf, "%d", v.IntVal())
		return true, nil
	case pdata.AttributeValueDOUBLE:
		fmt.Fprintf(buf, "%f", v.DoubleVal())
		return true, nil
	case pdata.AttributeValueSTRING:
		if sv := v.StringVal(); sv != "" {
			fmt.Fprintf(buf, "%q", sv)
			return true, nil
		}
	case pdata.AttributeValueBOOL:
		if v.BoolVal() {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return true, nil
	case pdata.AttributeValueMAP:
		n, err := encodeAttributeMap(buf, level+1, v.MapVal())
		if err == nil && !omitEmpty && n == 0 {
			buf.WriteString("null")
			return true, nil
		}
		return n > 0, err
	case pdata.AttributeValueARRAY:
		n, err := encodeAttributeArray(buf, level+1, v.ArrayVal())
		if err == nil && !omitEmpty && n == 0 {
			buf.WriteString("null")
			return true, nil
		}
		return n > 0, err
	}

	return false, nil
}
