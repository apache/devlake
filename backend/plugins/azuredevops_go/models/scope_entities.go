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

import "github.com/apache/devlake/core/plugin"

func ScopeEntities(scopeType string, configured []string) []string {
	if len(configured) == 0 {
		configured = plugin.DOMAIN_TYPES
	}
	allowed := map[string]struct{}{}
	for _, entity := range allowedScopeEntities(scopeType) {
		allowed[entity] = struct{}{}
	}
	entities := make([]string, 0, len(configured))
	for _, entity := range configured {
		if _, ok := allowed[entity]; ok {
			entities = append(entities, entity)
		}
	}
	return entities
}

func allowedScopeEntities(scopeType string) []string {
	switch scopeType {
	case RepositoryTypeProject:
		return []string{plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CODE_QUALITY}
	case RepositoryTypeADO:
		return []string{
			plugin.DOMAIN_TYPE_CODE,
			plugin.DOMAIN_TYPE_TICKET,
			plugin.DOMAIN_TYPE_CODE_REVIEW,
			plugin.DOMAIN_TYPE_CROSS,
			plugin.DOMAIN_TYPE_CICD,
			plugin.DOMAIN_TYPE_CODE_QUALITY,
		}
	default:
		return []string{
			plugin.DOMAIN_TYPE_CODE,
			plugin.DOMAIN_TYPE_TICKET,
			plugin.DOMAIN_TYPE_CICD,
			plugin.DOMAIN_TYPE_CODE_QUALITY,
		}
	}
}

func HasProjectEntity(entities []string) bool {
	for _, entity := range entities {
		if isProjectEntity(entity) {
			return true
		}
	}
	return false
}

func DropProjectEntities(entities []string) []string {
	kept := make([]string, 0, len(entities))
	for _, entity := range entities {
		if !isProjectEntity(entity) {
			kept = append(kept, entity)
		}
	}
	return kept
}

// ClaimProjectEntities keeps ticket and QA entities for the first scope of a project.
func ClaimProjectEntities(entities []string, projectKey string, claimed map[string]struct{}) []string {
	if !HasProjectEntity(entities) || projectKey == "" {
		return DropProjectEntities(entities)
	}
	if _, ok := claimed[projectKey]; ok {
		return DropProjectEntities(entities)
	}
	claimed[projectKey] = struct{}{}
	return entities
}

func isProjectEntity(entity string) bool {
	return entity == plugin.DOMAIN_TYPE_TICKET || entity == plugin.DOMAIN_TYPE_CODE_QUALITY
}
