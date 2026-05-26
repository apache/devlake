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
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/plugins/refdiff/models"
)

type RefdiffTaskData struct {
	Options *models.RefdiffOptions
	Since   *time.Time
}

type RefPairLists []models.RefPairList

type Refs []code.Ref
type RefsAlphabetically Refs
type RefsReverseAlphabetically Refs
type RefsSemver Refs
type RefsReverseSemver Refs

func (rs Refs) Len() int {
	return len(rs)
}

func (rs RefsAlphabetically) Len() int {
	return len(rs)
}

func (rs RefsAlphabetically) Less(i, j int) bool {
	return rs[i].Id < rs[j].Id
}

func (rs RefsAlphabetically) Swap(i, j int) {
	rs[i], rs[j] = rs[j], rs[i]
}

func (rs RefsReverseAlphabetically) Len() int {
	return len(rs)
}

func (rs RefsReverseAlphabetically) Less(i, j int) bool {
	return rs[i].Id > rs[j].Id
}

func (rs RefsReverseAlphabetically) Swap(i, j int) {
	rs[i], rs[j] = rs[j], rs[i]
}

func (rs RefsSemver) Len() int {
	return len(rs)
}

func (rs RefsSemver) Less(i, j int) bool {
	parti := strings.Split(rs[i].Name, ".")
	partj := strings.Split(rs[j].Name, ".")

	for k := 0; k < len(partj); k++ {
		if k >= len(parti) {
			return true
		}

		if len(parti[k]) < len(partj[k]) {
			return true
		}
		if len(parti[k]) > len(partj[k]) {
			return false
		}

		if parti[k] < partj[k] {
			return true
		}
		if parti[k] > partj[k] {
			return false
		}
	}
	return false
}

func (rs RefsSemver) Swap(i, j int) {
	rs[i], rs[j] = rs[j], rs[i]
}

func (rs RefsReverseSemver) Len() int {
	return len(rs)
}

func (rs RefsReverseSemver) Less(i, j int) bool {
	return RefsSemver(rs).Less(j, i)
}

func (rs RefsReverseSemver) Swap(i, j int) {
	rs[i], rs[j] = rs[j], rs[i]
}

// CalculateTagPattern Calculate the TagPattern order by tagsOrder and return the Refs
func CalculateTagPattern(db dal.Dal, tagsPattern string, tagsLimit int, tagsOrder string) (Refs, errors.Error) {
	rs := Refs{}

	// calculate Pattern part
	if tagsPattern == "" || tagsLimit <= 1 {
		return rs, nil
	}
	rows, err := db.Cursor(
		dal.From("refs"),
		dal.Where(""),
		dal.Orderby("created_date desc"),
	)

	if err != nil {
		return rs, err
	}
	defer rows.Next()
	r, err := errors.Convert01(regexp.Compile(tagsPattern))
	if err != nil {
		return rs, errors.Default.Wrap(err, fmt.Sprintf("unable to parse: %s", tagsPattern))
	}
	for rows.Next() {
		var ref code.Ref
		err = db.Fetch(rows, &ref)
		if err != nil {
			return rs, err
		}

		if ok := r.Match([]byte(ref.Name)); ok {
			rs = append(rs, ref)
		}
	}
	switch tagsOrder {
	case "alphabetically":
		sort.Sort(RefsAlphabetically(rs))
	case "reverse alphabetically":
		sort.Sort(RefsReverseAlphabetically(rs))
	case "semver":
		sort.Sort(RefsSemver(rs))
	case "reverse semver":
		sort.Sort(RefsReverseSemver(rs))
	default:
	}

	if tagsLimit < rs.Len() {
		rs = rs[:tagsLimit]
	}

	return rs, nil
}

// CalculateCommitPairs Calculate the commits pairs both from Options.Pairs and TagPattern
func CalculateCommitPairs(db dal.Dal, repoId string, pairs []models.RefPair, rs Refs) (models.RefCommitPairs, errors.Error) {
	commitPairs := make(models.RefCommitPairs, 0, len(rs)+len(pairs))
	for i := 1; i < len(rs); i++ {
		commitPairs = append(commitPairs, [4]string{rs[i-1].CommitSha, rs[i].CommitSha, rs[i-1].Name, rs[i].Name})
	}

	// calculate pairs part
	// convert ref pairs into commit pairs
	ref2sha := func(refName string) (string, error) {
		ref := &code.Ref{}
		if refName == "" {
			return "", errors.Default.New("ref name is empty")
		}
		ref.Id = fmt.Sprintf("%s:%s", repoId, refName)
		err := db.First(ref)
		if err != nil && !db.IsErrorNotFound(err) {
			return "", errors.NotFound.Wrap(err, fmt.Sprintf("failed to load Ref info for repoId:%s, refName:%s", repoId, refName))
		}
		return ref.CommitSha, nil
	}

	for i, refPair := range pairs {
		// get new ref's commit sha
		newCommit, err := ref2sha(refPair.NewRef)
		if err != nil {
			return models.RefCommitPairs{}, errors.Default.Wrap(err, fmt.Sprintf("failed to load commit sha for NewRef on pair #%d", i))
		}
		// get old ref's commit sha
		oldCommit, err := ref2sha(refPair.OldRef)
		if err != nil {
			return models.RefCommitPairs{}, errors.Default.Wrap(err, fmt.Sprintf("failed to load commit sha for OleRef on pair #%d", i))
		}

		have := false
		for _, cp := range commitPairs {
			if cp[0] == newCommit && cp[1] == oldCommit {
				have = true
				break
			}
		}
		if !have {
			commitPairs = append(commitPairs, models.RefCommitPair{newCommit, oldCommit, refPair.NewRef, refPair.OldRef})
		}
	}

	return commitPairs, nil
}
