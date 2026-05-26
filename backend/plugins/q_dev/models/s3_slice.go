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
	"fmt"
	"strconv"
	"strings"

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	"gorm.io/gorm"
)

// QDevS3Slice describes a time-sliced S3 prefix to collect from.
type QDevS3Slice struct {
	common.Scope `mapstructure:",squash"`
	Id           string `json:"id" mapstructure:"id" gorm:"primaryKey;type:varchar(512)"`
	Prefix       string `json:"prefix" mapstructure:"prefix" gorm:"type:varchar(512);not null"`
	BasePath     string `json:"basePath" mapstructure:"basePath" gorm:"type:varchar(512)"`
	AccountId    string `json:"accountId,omitempty" mapstructure:"accountId" gorm:"type:varchar(255)"`
	Year         int    `json:"year" mapstructure:"year" gorm:"not null"`
	Month        *int   `json:"month,omitempty" mapstructure:"month"`

	Name     string `json:"name" mapstructure:"name" gorm:"-"`
	FullName string `json:"fullName" mapstructure:"fullName" gorm:"-"`
}

func (QDevS3Slice) TableName() string {
	return "_tool_q_dev_s3_slices"
}

// BeforeSave ensures derived fields stay in sync before persisting.
func (s *QDevS3Slice) BeforeSave(_ *gorm.DB) error {
	return s.normalize(true)
}

// AfterFind fills derived fields for API responses.
func (s *QDevS3Slice) AfterFind(_ *gorm.DB) error {
	return s.normalize(false)
}

// normalize trims inputs, derives prefix/id/name fields, and optionally validates.
func (s *QDevS3Slice) normalize(strict bool) error {
	if s == nil {
		return nil
	}

	s.BasePath = cleanPath(s.BasePath)
	s.AccountId = strings.TrimSpace(s.AccountId)
	s.Prefix = cleanPath(selectNonEmpty(s.Prefix, s.Id))

	if s.Year <= 0 {
		if err := s.deriveYearAndMonthFromPrefix(); err != nil && strict {
			return err
		}
	}

	if s.Year <= 0 {
		if strict {
			return fmt.Errorf("year is required for QDev S3 slice")
		}
	}

	if s.Month != nil {
		if *s.Month < 1 || *s.Month > 12 {
			return fmt.Errorf("month must be between 1 and 12")
		}
	}

	if s.AccountId != "" {
		// New-style scope: construct a logical identifier from component parts
		s.Prefix = buildPrefixWithAccount(s.BasePath, s.AccountId, s.Year, s.Month)
	} else {
		// Legacy scope: derive prefix from basePath + year + month
		if s.Prefix == "" {
			s.Prefix = buildPrefix(s.BasePath, s.Year, s.Month)
		}

		prefix := buildPrefix(s.BasePath, s.Year, s.Month)
		if prefix != "" {
			s.Prefix = prefix
		}
	}

	if s.Id == "" {
		if s.AccountId != "" {
			// Use URL-safe ID: account_year or account_year_month
			if s.Month != nil {
				s.Id = fmt.Sprintf("%s_%04d_%02d", s.AccountId, s.Year, *s.Month)
			} else {
				s.Id = fmt.Sprintf("%s_%04d", s.AccountId, s.Year)
			}
		} else {
			s.Id = s.Prefix
		}
	}

	if s.AccountId != "" {
		if s.Month != nil {
			s.Name = fmt.Sprintf("%s %04d-%02d", s.AccountId, s.Year, *s.Month)
		} else if s.Year > 0 {
			s.Name = fmt.Sprintf("%s %04d", s.AccountId, s.Year)
		}
	} else {
		if s.Month != nil {
			s.Name = fmt.Sprintf("%04d-%02d", s.Year, *s.Month)
		} else if s.Year > 0 {
			s.Name = fmt.Sprintf("%04d", s.Year)
		}
	}

	if s.FullName == "" {
		s.FullName = s.Prefix
	}

	return nil
}

func (s *QDevS3Slice) deriveYearAndMonthFromPrefix() error {
	if s == nil {
		return nil
	}
	segments := splitPath(s.Prefix)
	if len(segments) == 0 {
		return fmt.Errorf("prefix is empty")
	}
	last := segments[len(segments)-1]
	if len(last) == 2 {
		if month, err := strconv.Atoi(last); err == nil {
			s.Month = ptr(month)
			if len(segments) >= 2 {
				yearSegment := segments[len(segments)-2]
				year, yearErr := strconv.Atoi(yearSegment)
				if yearErr != nil {
					return yearErr
				}
				s.Year = year
				base := segments[:len(segments)-2]
				s.BasePath = strings.Join(base, "/")
				return nil
			}
		}
	}
	if year, err := strconv.Atoi(last); err == nil {
		s.Year = year
		base := segments[:len(segments)-1]
		s.BasePath = strings.Join(base, "/")
		s.Month = nil
		return nil
	}
	return fmt.Errorf("unable to derive year/month from prefix %q", s.Prefix)
}

func (s QDevS3Slice) ScopeId() string {
	return s.Id
}

func (s QDevS3Slice) ScopeName() string {
	if s.Name != "" {
		return s.Name
	}
	if s.AccountId != "" {
		if s.Month != nil {
			return fmt.Sprintf("%s %04d-%02d", s.AccountId, s.Year, *s.Month)
		}
		if s.Year > 0 {
			return fmt.Sprintf("%s %04d", s.AccountId, s.Year)
		}
	}
	if s.Month != nil {
		return fmt.Sprintf("%04d-%02d", s.Year, *s.Month)
	}
	if s.Year > 0 {
		return fmt.Sprintf("%04d", s.Year)
	}
	return s.Prefix
}

func (s QDevS3Slice) ScopeFullName() string {
	if s.FullName != "" {
		return s.FullName
	}
	return s.Prefix
}

func (s QDevS3Slice) ScopeParams() interface{} {
	return &QDevS3SliceParams{
		ConnectionId: s.ConnectionId,
		Prefix:       s.Prefix,
	}
}

// Sanitize returns a copy ready for JSON serialization.
func (s QDevS3Slice) Sanitize() QDevS3Slice {
	_ = s.normalize(false)
	return s
}

type QDevS3SliceParams struct {
	ConnectionId uint64 `json:"connectionId"`
	Prefix       string `json:"prefix"`
}

var _ plugin.ToolLayerScope = (*QDevS3Slice)(nil)

func buildPrefixWithAccount(basePath string, accountId string, year int, month *int) string {
	parts := splitPath(basePath)
	if accountId != "" {
		parts = append(parts, accountId)
	}
	if year > 0 {
		parts = append(parts, fmt.Sprintf("%04d", year))
	}
	if month != nil {
		parts = append(parts, fmt.Sprintf("%02d", *month))
	}
	return strings.Join(parts, "/")
}

func buildPrefix(basePath string, year int, month *int) string {
	parts := splitPath(basePath)
	if year > 0 {
		parts = append(parts, fmt.Sprintf("%04d", year))
	}
	if month != nil {
		parts = append(parts, fmt.Sprintf("%02d", *month))
	}
	return strings.Join(parts, "/")
}

func splitPath(value string) []string {
	if value == "" {
		return nil
	}
	chunks := strings.Split(value, "/")
	result := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		trimmed := strings.TrimSpace(chunk)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func cleanPath(value string) string {
	return strings.Join(splitPath(value), "/")
}

func selectNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func ptr[T any](value T) *T {
	return &value
}
