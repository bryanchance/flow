package accounts

import (
	"bytes"
	"database/sql/driver"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/pkg/errors"
)

// Account
func (a *Account) Value() (driver.Value, error) {
	buf := bytes.Buffer{}
	if err := marshaler().Marshal(&buf, a); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *Account) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	buf := bytes.NewReader(b)
	return jsonpb.Unmarshal(buf, a)
}

// Namespace
func (n *Namespace) Value() (driver.Value, error) {
	buf := bytes.Buffer{}
	if err := marshaler().Marshal(&buf, n); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (n *Namespace) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	buf := bytes.NewReader(b)
	return jsonpb.Unmarshal(buf, n)
}

// ServiceToken
func (t *ServiceToken) Value() (driver.Value, error) {
	buf := bytes.Buffer{}
	if err := marshaler().Marshal(&buf, t); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (t *ServiceToken) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	buf := bytes.NewReader(b)
	return jsonpb.Unmarshal(buf, t)
}

// APIToken
func (t *APIToken) Value() (driver.Value, error) {
	buf := bytes.Buffer{}
	if err := marshaler().Marshal(&buf, t); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (t *APIToken) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	buf := bytes.NewReader(b)
	return jsonpb.Unmarshal(buf, t)
}

func marshaler() *jsonpb.Marshaler {
	return &jsonpb.Marshaler{EmitDefaults: true}
}
