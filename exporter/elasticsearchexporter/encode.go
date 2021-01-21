package elasticsearchexporter

import (
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
)

func encodeLogEvent(record pdata.LogRecord, mapping map[string]string) ([]byte, error) {
	obj := objectFromLog(record, mapping)
	obj.dedup()
	return obj.MarshalJSON()
}

func objectFromLog(record pdata.LogRecord, mapping map[string]string) object {
	obj := objFromAttributes(record.Attributes())

	// common logging fields
	obj.AddTimestamp("@timestamp", time.Unix(0, int64(record.Timestamp())))
	addOptAttribute(&obj, "message", record.Body())
	addOptString(&obj, "log.level", record.SeverityText())

	// trace details
	addOptID(&obj, "trace.id", record.TraceID())

	// span details
	addOptID(&obj, "span.id", record.SpanID())

	// TODO: what is name?
	addOptString(&obj, "name", record.Name())

	// TODO: add ECS observer fields

	if mapping != nil {
		mapFieldKeys(&obj, mapping)
	}
	return obj
}

func mapFieldKeys(obj *object, mapping map[string]string) {
	for i := range obj.fields {
		if mappedKey, ok := mapping[obj.fields[i].key]; ok {
			obj.fields[i].key = mappedKey
		}
	}
}

func addOptAttribute(obj *object, key string, attr pdata.AttributeValue) {
	if attr.Type() != pdata.AttributeValueNULL {
		obj.Add(key, valueFromAttribute(attr))
	}
}

func addOptString(obj *object, key string, val string) {
	if val != "" {
		obj.Add(key, stringValue(val))
	}
}

func addOptID(obj *object, key string, id interface {
	IsValid() bool
	HexString() string
}) {
	if id.IsValid() {
		obj.Add(key, stringValue(id.HexString()))
	}
}
