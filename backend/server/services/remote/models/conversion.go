/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package models

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/impls/dalgorm"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/utils"
	"gorm.io/datatypes"
)

func LoadTableModel(tableName string, schema utils.JsonObject, parentModel any) (models.DynamicTabler, errors.Error) {
	var baseType reflect.Type = nil
	if parentModel != nil {
		baseType = reflect.TypeOf(parentModel)
	}
	structType, err := GenerateStructType(schema, baseType)
	if err != nil {
		return nil, err
	}
	return models.NewDynamicTabler(tableName, structType), nil
}

func GenerateStructType(schema utils.JsonObject, baseType reflect.Type) (reflect.Type, errors.Error) {
	var structFields []reflect.StructField
	props, err := utils.GetProperty[utils.JsonObject](schema, "properties")
	if err != nil {
		return nil, err
	}
	required, err := utils.GetProperty[[]string](schema, "required")
	if err != nil {
		required = []string{}
	}
	if baseType != nil {
		anonymousField := reflect.StructField{
			Name:      baseType.Name(),
			Type:      baseType,
			Tag:       reflect.StructTag("mapstructure:\",squash\""),
			Anonymous: true,
		}
		structFields = append(structFields, anonymousField)
	}
	for k, v := range props {
		if baseType != nil && isBaseTypeField(k, baseType) {
			continue
		}
		spec := v.(utils.JsonObject)
		field, err := generateStructField(k, spec, isRequired(k, required))
		if err != nil {
			return nil, err
		}
		structFields = append(structFields, *field)
	}
	return reflect.StructOf(structFields), nil
}

func MapTo(x any, y any) errors.Error {
	b, err := json.Marshal(x)
	if err != nil {
		return errors.Convert(err)
	}
	if err = json.Unmarshal(b, y); err != nil {
		return errors.Convert(err)
	}
	return nil
}

func ToDatabaseMap(tableName string, ifc any, createdAt *time.Time, updatedAt *time.Time) (map[string]any, errors.Error) {
	m := map[string]any{}
	err := MapTo(ifc, &m)
	if err != nil {
		return nil, err
	}
	if createdAt != nil {
		m["createdAt"] = createdAt
	}
	if updatedAt != nil {
		m["updatedAt"] = updatedAt
	}
	m = dalgorm.ToDatabaseMap(tableName, m)
	return m, nil
}

func isRequired(fieldName string, required []string) bool {
	for _, r := range required {
		if fieldName == r {
			return true
		}
	}
	return false
}

func isBaseTypeField(fieldName string, baseType reflect.Type) bool {
	fieldName = canonicalFieldName(fieldName)
	for i := 0; i < baseType.NumField(); i++ {
		baseField := baseType.Field(i)
		if baseField.Anonymous {
			if isBaseTypeField(fieldName, baseField.Type) {
				return true
			}
		}
		if fieldName == canonicalFieldName(baseField.Name) {
			return true
		}
	}
	return false
}

func canonicalFieldName(fieldName string) string {
	return strings.ToLower(strings.Replace(fieldName, "_", "", -1))
}

var (
	int64Type   = reflect.TypeOf(int64(0))
	float64Type = reflect.TypeOf(float64(0))
	boolType    = reflect.TypeOf(false)
	stringType  = reflect.TypeOf("")
	timeType    = reflect.TypeOf(time.Time{})
	jsonMapType = reflect.TypeOf(datatypes.JSONMap{})
	jsonType    = reflect.TypeOf(datatypes.JSON{})
)

func generateStructField(name string, schema utils.JsonObject, required bool) (*reflect.StructField, errors.Error) {
	goType, err := getGoType(schema, required)
	if err != nil {
		return nil, errors.Default.Wrap(err, fmt.Sprintf("couldn't resolve type for field: \"%s\"", name))
	}
	tag, err := getTag(name, schema, goType, required)
	if err != nil {
		return nil, err
	}
	sf := &reflect.StructField{
		Name: strings.Title(name), //nolint:staticcheck
		Type: goType,
		Tag:  tag,
	}
	return sf, nil
}

func getGoType(schema utils.JsonObject, required bool) (reflect.Type, errors.Error) {
	rawType, ok := schema["type"].(string)
	if !ok {
		return nil, errors.BadInput.New("\"type\" property must be a string")
	}
	switch rawType {
	//TODO: support more types
	case "integer":
		return int64Type, nil
	case "number":
		return float64Type, nil
	case "boolean":
		return boolType, nil
	case "string":
		format, err := utils.GetProperty[string](schema, "format")
		if err == nil && format == "date-time" {
			if required {
				return timeType, nil
			} else {
				return reflect.PointerTo(timeType), nil
			}
		} else {
			return stringType, nil
		}
	case "array":
		return jsonType, nil
	case "object":
		return jsonMapType, nil
	default:
		return nil, errors.BadInput.New(fmt.Sprintf("Unsupported type %s", rawType))
	}
}

func getTag(name string, schema utils.JsonObject, goType reflect.Type, required bool) (reflect.StructTag, errors.Error) {
	tags := []string{}
	tags = append(tags, fmt.Sprintf("json:\"%s\"", name))
	gormTag := getGormTag(schema, goType)
	if gormTag != "" {
		tags = append(tags, gormTag)
	}
	if required {
		tags = append(tags, "validate:\"required\"")
	}
	return reflect.StructTag(strings.Join(tags, " ")), nil
}

func getGormTag(schema utils.JsonObject, goType reflect.Type) string {
	gormTags := []string{}
	primaryKey, err := utils.GetProperty[bool](schema, "primaryKey")
	if err == nil && primaryKey {
		gormTags = append(gormTags, "primaryKey")
	}
	autoIncrement, err := utils.GetProperty[bool](schema, "autoIncrement")
	if err == nil {
		gormTags = append(gormTags, fmt.Sprintf("autoIncrement:%v", autoIncrement))
	}
	if goType == stringType {
		maxLength, err := utils.GetProperty[float64](schema, "maxLength")
		maxLengthInt := int(maxLength)
		if err == nil {
			if maxLengthInt > 255 {
				gormTags = append(gormTags, "type:text")
			} else {
				gormTags = append(gormTags, fmt.Sprintf("type:varchar(%d)", maxLengthInt))
			}
		} else if primaryKey {
			// primary keys must have a key length
			gormTags = append(gormTags, "type:varchar(255)")
		} else {
			gormTags = append(gormTags, "type:text")
		}
	}
	format, err := utils.GetProperty[string](schema, "format")
	if err == nil && format == "password" {
		gormTags = append(gormTags, "serializer:encdec")
	}
	if len(gormTags) == 0 {
		return ""
	}
	return fmt.Sprintf("gorm:\"%s\"", strings.Join(gormTags, ";"))
}
