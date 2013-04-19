// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package datastore

import (
	"fmt"
	"reflect"
	"time"

	"appengine"

	pb "appengine_internal/datastore"
)

var (
	typeOfByteSlice = reflect.TypeOf([]byte(nil))
	typeOfTime      = reflect.TypeOf(time.Time{})
)

// typeMismatchReason returns a string explaining why the property p could not
// be stored in an entity field of type v.Type().
func typeMismatchReason(p Property, v reflect.Value) string {
	entityType := "empty"
	switch p.Value.(type) {
	case int64:
		entityType = "int"
	case bool:
		entityType = "bool"
	case string:
		entityType = "string"
	case float64:
		entityType = "float"
	case *Key:
		entityType = "*datastore.Key"
	case time.Time:
		entityType = "time.Time"
	case appengine.BlobKey:
		entityType = "appengine.BlobKey"
	case []byte:
		entityType = "[]byte"
	}
	return fmt.Sprintf("type mismatch: %s versus %v", entityType, v.Type())
}

type propertyLoader struct {
	// m holds the number of times a substruct field like "Foo.Bar.Baz" has
	// been seen so far. The map is constructed lazily.
	m map[string]int
}

func (l *propertyLoader) load(codec *structCodec, structValue reflect.Value, p Property, requireSlice bool) string {
	var v reflect.Value
	// Traverse a struct's struct-typed fields.
	for name := p.Name; ; {
		decoder, ok := codec.byName[name]
		if !ok {
			return "no such struct field"
		}
		v = structValue.Field(decoder.index)
		if !v.IsValid() {
			return "no such struct field"
		}
		if !v.CanSet() {
			return "cannot set struct field"
		}

		if decoder.substructCodec == nil {
			break
		}

		if v.Kind() == reflect.Slice {
			if l.m == nil {
				l.m = make(map[string]int)
			}
			index := l.m[p.Name]
			l.m[p.Name] = index + 1
			for v.Len() <= index {
				v.Set(reflect.Append(v, reflect.New(v.Type().Elem()).Elem()))
			}
			structValue = v.Index(index)
			requireSlice = false
		} else {
			structValue = v
		}
		// Strip the "I." from "I.X".
		name = name[len(codec.byIndex[decoder.index].name):]
		codec = decoder.substructCodec
	}

	var slice reflect.Value
	if v.Kind() == reflect.Slice && v.Type() != typeOfByteSlice {
		slice = v
		v = reflect.New(v.Type().Elem()).Elem()
	} else if requireSlice {
		return "multiple-valued property requires a slice field type"
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		x, ok := p.Value.(int64)
		if !ok {
			return typeMismatchReason(p, v)
		}
		if v.OverflowInt(x) {
			return fmt.Sprintf("value %v overflows struct field of type %v", x, v.Type())
		}
		v.SetInt(x)
	case reflect.Bool:
		x, ok := p.Value.(bool)
		if !ok {
			return typeMismatchReason(p, v)
		}
		v.SetBool(x)
	case reflect.String:
		if x, ok := p.Value.(appengine.BlobKey); ok {
			v.SetString(string(x))
			break
		}
		x, ok := p.Value.(string)
		if !ok {
			return typeMismatchReason(p, v)
		}
		v.SetString(x)
	case reflect.Float32, reflect.Float64:
		x, ok := p.Value.(float64)
		if !ok {
			return typeMismatchReason(p, v)
		}
		if v.OverflowFloat(x) {
			return fmt.Sprintf("value %v overflows struct field of type %v", x, v.Type())
		}
		v.SetFloat(x)
	case reflect.Ptr:
		x, ok := p.Value.(*Key)
		if p.Value != nil && !ok {
			return typeMismatchReason(p, v)
		}
		if _, ok := v.Interface().(*Key); !ok {
			return typeMismatchReason(p, v)
		}
		v.Set(reflect.ValueOf(x))
	case reflect.Struct:
		x, ok := p.Value.(time.Time)
		if !ok {
			return typeMismatchReason(p, v)
		}
		if _, ok := v.Interface().(time.Time); !ok {
			return typeMismatchReason(p, v)
		}
		v.Set(reflect.ValueOf(x))
	case reflect.Slice:
		x, ok := p.Value.([]byte)
		if !ok {
			return typeMismatchReason(p, v)
		}
		if _, ok := v.Interface().([]byte); !ok {
			return typeMismatchReason(p, v)
		}
		v.Set(reflect.ValueOf(x))
	default:
		return typeMismatchReason(p, v)
	}
	if slice.IsValid() {
		slice.Set(reflect.Append(slice, v))
	}
	return ""
}

// loadEntity loads an EntityProto into PropertyLoadSaver or struct pointer.
func loadEntity(dst interface{}, src *pb.EntityProto) (err error) {
	c := make(chan Property, 32)
	errc := make(chan error, 1)
	defer func() {
		if err == nil {
			err = <-errc
		}
	}()
	go protoToProperties(c, errc, src)
	if e, ok := dst.(PropertyLoadSaver); ok {
		return e.Load(c)
	}
	return LoadStruct(dst, c)
}

func (s structPLS) Load(c <-chan Property) error {
	var fieldName, reason string
	var l propertyLoader
	for p := range c {
		if errStr := l.load(s.codec, s.v, p, p.Multiple); errStr != "" {
			// We don't return early, as we try to load as many properties as possible.
			// It is valid to load an entity into a struct that cannot fully represent it.
			// That case returns an error, but the caller is free to ignore it.
			fieldName, reason = p.Name, errStr
		}
	}
	if reason != "" {
		return &ErrFieldMismatch{
			StructType: s.v.Type(),
			FieldName:  fieldName,
			Reason:     reason,
		}
	}
	return nil
}

func protoToProperties(dst chan<- Property, errc chan<- error, src *pb.EntityProto) {
	defer close(dst)
	props, rawProps := src.Property, src.RawProperty
	for {
		var (
			x       *pb.Property
			noIndex bool
		)
		if len(props) > 0 {
			x, props = props[0], props[1:]
		} else if len(rawProps) > 0 {
			x, rawProps = rawProps[0], rawProps[1:]
			noIndex = true
		} else {
			break
		}

		var value interface{}
		switch {
		case x.Value.Int64Value != nil:
			if x.Meaning != nil && *x.Meaning == pb.Property_GD_WHEN {
				value = fromUnixMicro(*x.Value.Int64Value)
			} else {
				value = *x.Value.Int64Value
			}
		case x.Value.BooleanValue != nil:
			value = *x.Value.BooleanValue
		case x.Value.StringValue != nil:
			if x.Meaning != nil && *x.Meaning == pb.Property_BLOB {
				value = []byte(*x.Value.StringValue)
			} else if x.Meaning != nil && *x.Meaning == pb.Property_BLOBKEY {
				value = appengine.BlobKey(*x.Value.StringValue)
			} else {
				value = *x.Value.StringValue
			}
		case x.Value.DoubleValue != nil:
			value = *x.Value.DoubleValue
		case x.Value.Referencevalue != nil:
			key, err := referenceValueToKey(x.Value.Referencevalue)
			if err != nil {
				errc <- err
				return
			}
			value = key
		}
		dst <- Property{
			Name:     x.GetName(),
			Value:    value,
			NoIndex:  noIndex,
			Multiple: x.GetMultiple(),
		}
	}
	errc <- nil
}
