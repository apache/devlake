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

package parse

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	ParserRFE   = "rfe_assess"
	ParserStrat = "strat_pipeline"

	SchemaVersion  = "0.1"
	MappingVersion = "draft-0.1"
	AdapterVersion = "0.1"

	KindAssessment = "assessment"
	KindProcessing = "processing_result"

	AttemptUnknown      = "unknown"
	MatchUnresolved     = "unresolved"
	MatchUnique         = "confirmed_unique"
	MatchAmbiguous      = "ambiguous"
	JiraSystem          = "jira"
	JiraInstance        = "redhat.atlassian.net"
	GenericRunID        = "artifacts"
	IdStatusKeyOnly     = "key_only"
	IdStatusUnconfirmed = "unconfirmed"
)

// ID is the stable identity of a collected record.
func ID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

// ObservationID identifies one converted record. The blob sha is the file revision.
func ObservationID(sourceID, blobID, path, selector, kind string) string {
	return ID(sourceID, blobID, path, selector, kind)
}
