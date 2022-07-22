package workflows

import (
	"bytes"
	"database/sql/driver"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/pkg/errors"
)

func (w *Workflow) Value() (driver.Value, error) {
	buf := bytes.Buffer{}
	if err := marshaler().Marshal(&buf, w); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (w *Workflow) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	buf := bytes.NewReader(b)
	return jsonpb.Unmarshal(buf, w)
}

func marshaler() *jsonpb.Marshaler {
	return &jsonpb.Marshaler{EmitDefaults: true}
}
