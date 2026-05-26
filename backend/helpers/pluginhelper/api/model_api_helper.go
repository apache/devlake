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

package api

import (
	"fmt"
	"net/http"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/srvhelper"
	"github.com/apache/incubator-devlake/helpers/utils"
	"github.com/go-playground/validator/v10"
)

var vld = validator.New()

type ModelApiHelper[M dal.Tabler] struct {
	basicRes       context.BasicRes
	dalHelper      *srvhelper.ModelSrvHelper[M]
	log            log.Logger
	modelName      string
	pkPathVarNames []string
	sterilizers    []func(m M) M
}

func NewModelApiHelper[M dal.Tabler](
	basicRes context.BasicRes,
	dalHelper *srvhelper.ModelSrvHelper[M],
	pkPathVarNames []string, // path variable names of primary key
	sterilizer func(m M) M,
) *ModelApiHelper[M] {
	m := new(M)
	modelName := fmt.Sprintf("%T", m)
	modelApiHelper := &ModelApiHelper[M]{
		basicRes:       basicRes,
		dalHelper:      dalHelper,
		log:            basicRes.GetLogger().Nested(fmt.Sprintf("%s_dal", modelName)),
		modelName:      modelName,
		pkPathVarNames: pkPathVarNames,
	}
	if sterilizer != nil {
		modelApiHelper.sterilizers = []func(m M) M{sterilizer}
	}
	return modelApiHelper
}

func (self *ModelApiHelper[M]) Post(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	model := new(M)
	err := DecodeMapStruct(input.Body, model, false)
	if err != nil {
		return nil, err
	}
	err = self.dalHelper.Create(model)
	if err != nil {
		return nil, err
	}
	model = self.Sanitize(model)
	return &plugin.ApiResourceOutput{
		Status: http.StatusCreated,
		Body:   model,
	}, nil
}

func (self *ModelApiHelper[M]) ExtractPkValues(input *plugin.ApiResourceInput) ([]interface{}, errors.Error) {
	pkv := make([]interface{}, len(self.pkPathVarNames))
	for i, pkn := range self.pkPathVarNames {
		var ok bool
		pkv[i], ok = input.Params[pkn]
		if !ok {
			return nil, errors.BadInput.New(fmt.Sprintf("missing path variable %s", pkn))
		}
	}
	return pkv, nil
}

func (self *ModelApiHelper[M]) FindByPk(input *plugin.ApiResourceInput) (*M, errors.Error) {
	pkv, err := self.ExtractPkValues(input)
	if err != nil {
		return nil, err
	}
	return self.dalHelper.FindByPk(pkv...)
}

func (self *ModelApiHelper[M]) GetDetail(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	model, err := self.FindByPk(input)
	if err != nil {
		return nil, err
	}
	model = self.Sanitize(model)
	return &plugin.ApiResourceOutput{
		Body: model,
	}, nil
}

func (self *ModelApiHelper[M]) Sanitize(model *M) *M {
	if self.sterilizers != nil {
		for _, sterilizer := range self.sterilizers {
			sanitizedModel := sterilizer(*model)
			model = &sanitizedModel
		}
	}
	return model
}

func (self *ModelApiHelper[M]) BatchSanitize(models []*M) []*M {
	for idx, m := range models {
		model := *m
		models[idx] = self.Sanitize(&model)
	}
	return models
}

type CustomMerge[M dal.Tabler] interface {
	MergeFromRequest(target *M, body map[string]interface{}) error
}

// PatchModel will get an "M" from database and try to merge update from request body
// zeroFields decides whether "M" will be zeroed if "M" doesn't implement CustomMerge.
func (self *ModelApiHelper[M]) PatchModel(input *plugin.ApiResourceInput, zeroFields bool) (*M, error) {
	model, err := self.FindByPk(input)
	if err != nil {
		return nil, err
	}
	if v, ok := (interface{}(model)).(CustomMerge[M]); ok {
		if err := v.MergeFromRequest(model, input.Body); err != nil {
			return nil, err
		}
	} else {
		err = DecodeMapStruct(input.Body, model, zeroFields)
		if err != nil {
			return nil, errors.BadInput.Wrap(err, fmt.Sprintf("faled to patch %s", self.modelName))
		}
	}
	return model, nil
}

func (self *ModelApiHelper[M]) Patch(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	model, err := self.PatchModel(input, true)
	if err != nil {
		return nil, errors.Convert(err)
	}
	if err := self.dalHelper.Update(model); err != nil {
		return nil, err
	}
	model = self.Sanitize(model)
	return &plugin.ApiResourceOutput{
		Body: model,
	}, nil
}

func (self *ModelApiHelper[M]) Delete(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	model, err := self.FindByPk(input)
	if err != nil {
		return nil, err
	}
	err = self.dalHelper.DeleteModel(model)
	if err != nil {
		return nil, err
	}
	model = self.Sanitize(model)
	return &plugin.ApiResourceOutput{
		Body: model,
	}, nil
}

func (self *ModelApiHelper[M]) GetAll(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	all, err := self.dalHelper.GetAll()
	all = self.BatchSanitize(all)
	return &plugin.ApiResourceOutput{
		Body: all,
	}, err
}

func (self *ModelApiHelper[M]) PutMultipleCb(input *plugin.ApiResourceInput, beforeSave func(*M) errors.Error) (*plugin.ApiResourceOutput, errors.Error) {
	var req struct {
		Data []*M `json:"data"`
	}
	err := DecodeMapStruct(input.Body, &req, false)
	if err != nil {
		return nil, err
	}
	for i, item := range req.Data {
		if beforeSave != nil {
			err := beforeSave(item)
			if err != nil {
				return nil, err
			}
		}
		err := self.dalHelper.CreateOrUpdate(item)
		if err != nil {
			return nil, errors.BadInput.Wrap(err, fmt.Sprintf("failed to save item %d", i))
		}
	}
	req.Data = self.BatchSanitize(req.Data)
	return &plugin.ApiResourceOutput{
		Body: req.Data,
	}, nil
}

func parsePagination[P any](input *plugin.ApiResourceInput) (*P, errors.Error) {
	if !input.Query.Has("page") {
		input.Query.Set("page", "1")
	}
	if !input.Query.Has("pageSize") {
		input.Query.Set("pageSize", "100")
	}
	pagination := new(P)
	err := utils.DecodeMapStruct(input.Query, pagination, false)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to decode pagination from query string")
	}
	err = utils.DecodeMapStruct(input.Params, pagination, false)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to decode pagination from path variables")
	}
	if e := vld.Struct(pagination); e != nil {
		return nil, errors.BadInput.Wrap(e, "invalid pagination parameters")
	}
	return pagination, nil
}
