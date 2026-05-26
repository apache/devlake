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

package context

import (
	"github.com/apache/incubator-devlake/core/config"
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/log"
)

// DefaultBasicRes offers a common implementation for the  BasisRes interface
type DefaultBasicRes struct {
	cfg    config.ConfigReader
	logger log.Logger
	db     dal.Dal
}

// GetConfigReader returns the ConfigReader instance
func (c *DefaultBasicRes) GetConfigReader() config.ConfigReader {
	return c.cfg
}

// GetConfig returns the value of the specified name
func (c *DefaultBasicRes) GetConfig(name string) string {
	return c.cfg.GetString(name)
}

// GetLogger returns the Logger instance
func (c *DefaultBasicRes) GetLogger() log.Logger {
	return c.logger
}

// NestedLogger returns a new DefaultBasicRes with a new nested logger
func (c *DefaultBasicRes) NestedLogger(name string) context.BasicRes {
	return &DefaultBasicRes{
		cfg:    c.cfg,
		logger: c.logger.Nested(name),
		db:     c.db,
	}
}

// ReplaceLogger returns a new DefaultBasicRes with the specified logger
func (c *DefaultBasicRes) ReplaceLogger(logger log.Logger) context.BasicRes {
	return &DefaultBasicRes{
		cfg:    c.cfg,
		logger: logger,
		db:     c.db,
	}
}

// GetDal returns the Dal instance
func (c *DefaultBasicRes) GetDal() dal.Dal {
	return c.db
}

// NewDefaultBasicRes creates a new DefaultBasicRes instance
func NewDefaultBasicRes(
	cfg config.ConfigReader,
	logger log.Logger,
	db dal.Dal,
) *DefaultBasicRes {
	return &DefaultBasicRes{
		cfg:    cfg,
		logger: logger,
		db:     db,
	}
}
