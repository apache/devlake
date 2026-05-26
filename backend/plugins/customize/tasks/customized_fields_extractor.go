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

package tasks

import (
	"context"
	"fmt"
	"strings"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/customize/models"
	"github.com/tidwall/gjson"
)

var _ plugin.SubTaskEntryPoint = ExtractCustomizedFields

var ExtractCustomizedFieldsMeta = plugin.SubTaskMeta{Name: "extractCustomizedFields",
	EntryPoint:       ExtractCustomizedFields,
	EnabledByDefault: true,
	Description:      "extract customized fields",
}

// ExtractCustomizedFields extracts fields from raw data tables and assigns to domain layer tables
func ExtractCustomizedFields(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*TaskData)
	if data == nil || data.Options == nil {
		return nil
	}
	d := taskCtx.GetDal()
	var err error
	for _, rule := range data.Options.TransformationRules {
		err = extractCustomizedFields(taskCtx.GetContext(), d, rule.Table, rule.RawDataTable, rule.RawDataParams, rule.Mapping)
		if err != nil {
			return errors.Default.Wrap(err, "error extracting customized fields")
		}
	}
	return nil
}

func extractCustomizedFields(ctx context.Context, d dal.Dal, table, rawTable, rawDataParams string, extractor map[string]string) error {
	pkFields, err := dal.GetPrimarykeyColumns(d, &models.Table{Name: table})
	if err != nil {
		return err
	}
	rawDataField := fmt.Sprintf("%s.data", rawTable)
	// `fields` only include `_raw_data_id` and primary keys coming from the domain layer table, and `data` coming from the raw layer
	fields := []string{fmt.Sprintf("%s.%s", table, "_raw_data_id")}
	fields = append(fields, rawDataField)
	for _, field := range pkFields {
		fields = append(fields, fmt.Sprintf("%s.%s", table, field.Name()))
	}
	clauses := []dal.Clause{
		dal.Select(strings.Join(fields, ", ")),
		dal.From(table),
		dal.Join(fmt.Sprintf(" LEFT JOIN %s ON %s._raw_data_id = %s.id", rawTable, table, rawTable)),
		dal.Where("_raw_data_table = ? AND _raw_data_params LIKE ?", rawTable, rawDataParams),
	}
	rows, err := d.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		row := make(map[string]interface{})
		updates := make(map[string]interface{})
		err = d.Fetch(rows, &row)
		if err != nil {
			return err
		}
		switch blob := row["data"].(type) {
		case []byte:
			for field, path := range extractor {
				result := gjson.GetBytes(blob, path)
				fillInUpdates(result, field, updates)
			}
		case string:
			for field, path := range extractor {
				result := gjson.Get(blob, path)
				// special case for issues custom_fields
				rawDataId, ok := row["_raw_data_id"].(int64)
				if !ok {
					return errors.Default.New("_raw_data_id is not int64")
				}
				if table == "issues" && result.IsArray() {
					issueId := row["id"].(string)
					fieldId := field
					// Delete existing records for the given issue and field
					err = d.Delete(
						&ticket.IssueCustomArrayField{},
						dal.Where("issue_id = ? AND field_id = ?", issueId, fieldId),
					)
					if err != nil {
						return err
					}

					result.ForEach(func(_, v gjson.Result) bool {
						err1 := d.Create(&ticket.IssueCustomArrayField{
							IssueId:    issueId,
							FieldId:    fieldId,
							FieldValue: v.String(),
							NoPKModel: common.NoPKModel{
								RawDataOrigin: common.RawDataOrigin{
									RawDataParams: rawDataParams,
									RawDataTable:  rawTable,
									RawDataId:     uint64(rawDataId),
								},
							},
						})
						if err1 != nil {
							err = err1
							return false
						}
						return true
					})
				} else {
					fillInUpdates(result, field, updates)
				}
			}
		default:
			return nil
		}

		if len(updates) > 0 {
			// remove columns that are not primary key
			delete(row, "_raw_data_id")
			delete(row, "data")
			query, params, err := mkUpdate(table, updates, row)
			if err != nil {
				return err
			}
			err = d.Exec(query, params...)
			if err != nil {
				return errors.Default.Wrap(err, "Exec SQL error")
			}
		}
	}
	return nil
}

// fillInUpdates fills in the updates map with the result of the gjson query
func fillInUpdates(result gjson.Result, field string, updates map[string]interface{}) {
	if result.Type == gjson.Null {
		updates[field] = nil
	} else {
		updates[field] = result.String()
	}
}

// mkUpdate generates SQL statement and parameters for updating a record
func mkUpdate(table string, updates map[string]interface{}, pk map[string]interface{}) (string, []interface{}, error) {
	if err := dal.ValidateTableName(table); err != nil {
		return "", nil, err
	}
	var params []interface{}
	stat := fmt.Sprintf("UPDATE %s SET ", table)
	var uu []string
	for field, value := range updates {
		if err := dal.ValidateColumnName(field); err != nil {
			return "", nil, err
		}
		uu = append(uu, fmt.Sprintf("%s = ?", field))
		params = append(params, value)
	}
	var ww []string
	for field, value := range pk {
		if err := dal.ValidateColumnName(field); err != nil {
			return "", nil, err
		}
		ww = append(ww, fmt.Sprintf("%s = ?", field))
		params = append(params, value)
	}
	return stat + strings.Join(uu, ", ") + " WHERE " + strings.Join(ww, " AND "), params, nil
}
