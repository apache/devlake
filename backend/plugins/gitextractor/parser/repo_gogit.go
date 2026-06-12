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

package parser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/gitextractor/models"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

type GogitRepoCollector struct {
	id      string
	logger  log.Logger
	store   models.Store
	repo    *gogit.Repository
	cleanUp func()
}

func NewGogitRepoCollector(localDir string, repoId string, store models.Store, logger log.Logger) (*GogitRepoCollector, errors.Error) {
	repo, err := gogit.PlainOpen(localDir)
	if err != nil {
		return nil, errors.Convert(err)
	}
	return &GogitRepoCollector{
		id:     repoId,
		logger: logger,
		store:  store,
		repo:   repo,
	}, nil
}

func (r *GogitRepoCollector) SetCleanUp(f func()) error {
	if f != nil {
		r.cleanUp = f
	}
	return nil
}

func (r *GogitRepoCollector) Close(ctx context.Context) error {
	if err := r.store.Close(); err != nil {
		return err
	}
	if r.cleanUp != nil {
		r.cleanUp()
	}
	return nil
}

// CollectAll The main parser subtask
func (r *GogitRepoCollector) CollectAll(subtaskCtx plugin.SubTaskContext) error {
	subtaskCtx.SetProgress(0, -1)
	err := r.CollectTags(subtaskCtx)
	if err != nil {
		return err
	}
	err = r.CollectBranches(subtaskCtx)
	if err != nil {
		return err
	}
	err = r.CollectCommits(subtaskCtx)
	if err != nil {
		return err
	}
	return r.CollectDiffLine(subtaskCtx)
}

// CountTags Count git tags subtask
func (r *GogitRepoCollector) CountTags(ctx context.Context) (int, error) {
	iter, err := r.repo.Tags()
	if err != nil {
		return 0, err
	}
	var tagsCount int
	if err := iter.ForEach(func(reference *plumbing.Reference) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		tagsCount += 1
		return nil
	}); err != nil {
		return 0, err
	}
	return tagsCount, nil
}

// CountBranches count the number of branches in a git repo
func (r *GogitRepoCollector) CountBranches(ctx context.Context) (int, error) {
	refIter, err := r.repo.Storer.IterReferences()
	if err != nil {
		return 0, err
	}
	branchIter := storer.NewReferenceFilteredIter(
		func(r *plumbing.Reference) bool {
			return r.Name().IsBranch() || r.Name().IsRemote()
		}, refIter)
	var branchesCount int

	headRef, err := r.repo.Head()
	if err != nil {
		return 0, err
	}
	if err := branchIter.ForEach(func(reference *plumbing.Reference) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if reference.Name() != headRef.Name() {
			branchesCount += 1
		}
		return nil
	}); err != nil {
		return 0, err
	}
	return branchesCount, nil
}

// CountCommits count the number of commits in a git repo
func (r *GogitRepoCollector) CountCommits(ctx context.Context) (int, error) {
	iter, err := r.repo.CommitObjects()
	if err != nil {
		return 0, err
	}
	var count int
	if err := iter.ForEach(func(commit *object.Commit) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		count += 1
		return nil
	}); err != nil {
		return 0, err
	}
	return count, nil
}

// CollectTags Collect Tags data
func (r *GogitRepoCollector) CollectTags(subtaskCtx plugin.SubTaskContext) error {
	tagIter, err := r.repo.Tags()
	if err != nil {
		return err
	}
	if err := tagIter.ForEach(func(ref *plumbing.Reference) error {
		select {
		case <-subtaskCtx.GetContext().Done():
			return subtaskCtx.GetContext().Err()
		default:
		}
		tagCommit := ref.Hash().String()
		_, err := r.repo.CommitObject(ref.Hash())
		if err != nil && errors.Is(err, plumbing.ErrObjectNotFound) {
			h, err := r.repo.ResolveRevision(plumbing.Revision(ref.Name()))
			if err != nil {
				return err
			}
			tagCommit = h.String()
		}
		name := ref.Name().String()
		if tagCommit != "" {
			codeRef := &code.Ref{
				DomainEntityExtended: domainlayer.DomainEntityExtended{Id: fmt.Sprintf("%s:%s", r.id, name)},
				RepoId:               r.id,
				Name:                 name,
				CommitSha:            tagCommit,
				RefType:              TAG,
			}
			err = r.store.Refs(codeRef)
			if err != nil {
				return err
			}
			subtaskCtx.IncProgress(1)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// CollectBranches Collect branch data
func (r *GogitRepoCollector) CollectBranches(subtaskCtx plugin.SubTaskContext) error {
	refIter, err := r.repo.Storer.IterReferences()
	if err != nil {
		return err
	}
	branchIter := storer.NewReferenceFilteredIter(
		func(r *plumbing.Reference) bool {
			return r.Name().IsBranch() || r.Name().IsRemote()
		}, refIter)
	headRef, err := r.repo.Head()
	if err != nil {
		return err
	}
	if err := branchIter.ForEach(func(ref *plumbing.Reference) error {
		select {
		case <-subtaskCtx.GetContext().Done():
			return subtaskCtx.GetContext().Err()
		default:
		}
		name := ref.Name().Short()
		sha := ref.Hash().String()
		_, err := r.repo.CommitObject(ref.Hash())
		if err != nil && errors.Is(err, plumbing.ErrObjectNotFound) {
			// handle commit sha like "0000000000000000000000000000000000000000"
			h, err := r.repo.ResolveRevision(plumbing.Revision(ref.Name()))
			if err != nil {
				return err
			}
			sha = h.String()
		}
		codeRef := &code.Ref{
			DomainEntityExtended: domainlayer.DomainEntityExtended{Id: fmt.Sprintf("%s:%s", r.id, name)},
			RepoId:               r.id,
			Name:                 name,
			CommitSha:            sha,
			RefType:              BRANCH,
			IsDefault:            ref.Name() == headRef.Name(),
		}
		if err := r.store.Refs(codeRef); err != nil {
			return err
		}
		subtaskCtx.IncProgress(1)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (r *GogitRepoCollector) getComponentMap(subtaskCtx plugin.SubTaskContext) (map[string]*regexp.Regexp, error) {
	db := subtaskCtx.GetDal()
	components := make([]code.Component, 0)
	err := db.All(&components, dal.From(components), dal.Where("repo_id= ?", r.id))
	if err != nil {
		return nil, err
	}
	componentMap := make(map[string]*regexp.Regexp)
	for _, component := range components {
		componentMap[component.Name] = regexp.MustCompile(component.PathRegex)
	}
	return componentMap, nil
}

// CollectCommits Collect data from each commit, we can also get the diff line
func (r *GogitRepoCollector) CollectCommits(subtaskCtx plugin.SubTaskContext) (err error) {
	taskOpts := subtaskCtx.GetData().(*GitExtractorTaskData).Options
	// check it first
	componentMap, err := r.getComponentMap(subtaskCtx)
	if err != nil {
		return err
	}

	repo := r.repo
	store := r.store

	commitsObjectsIter, err := repo.CommitObjects()
	if err != nil {
		return err
	}

	if err := commitsObjectsIter.ForEach(func(commit *object.Commit) error {
		select {
		case <-subtaskCtx.GetContext().Done():
			return subtaskCtx.GetContext().Err()
		default:
		}
		commitSha := commit.Hash.String()

		if commit.NumParents() != 0 {
			_, err := commit.Parents().Next()
			if err != nil {
				if err == plumbing.ErrObjectNotFound {
					// Skip calculating commit statistics when there are parent commits, but the first one cannot be fetched from the ODB.
					// This usually happens during a shallow clone for incremental collection. Otherwise, we might end up overwriting
					// the correct addition/deletion data in the database with an absurdly large addition number.
					r.logger.Info("skip commit %s because it has no parent commit", commitSha)
					return nil
				}
				return err
			}
		}
		codeCommit := &code.Commit{
			Sha:            commitSha,
			Message:        commit.Message,
			AuthorName:     commit.Author.Name,
			AuthorEmail:    commit.Author.Email,
			AuthorId:       commit.Author.Email,
			AuthoredDate:   commit.Author.When,
			CommitterName:  commit.Committer.Name,
			CommitterEmail: commit.Committer.Email,
			CommitterId:    commit.Committer.Email,
			CommittedDate:  commit.Committer.When,
		}
		if err = r.storeParentCommits(commitSha, commit); err != nil {
			return err
		}

		if !*taskOpts.SkipCommitStat {
			stats, err := commit.StatsContext(subtaskCtx.GetContext())
			if err != nil {
				return err
			} else {
				excluded := map[string]struct{}{}
				for _, ext := range taskOpts.ExcludeFileExtensions {
					e := strings.ToLower(strings.TrimSpace(ext))
					if e == "" {
						continue
					}
					excluded[e] = struct{}{}
				}
				for _, stat := range stats {
					nameLower := strings.ToLower(stat.Name)
					skip := false
					for ext := range excluded {
						if strings.HasSuffix(nameLower, ext) {
							skip = true
							break
						}
					}
					if skip {
						continue
					}
					codeCommit.Additions += stat.Addition
					// In some repos, deletion may be zero, which is different from git log --stat.
					// It seems go-git doesn't get the correct changes.
					// I have run object.DiffTreeWithOptions manually with different diff algorithms,
					// but get the same result with StatsContext.
					// I cannot reproduce it with another repo.
					// A similar issue: https://github.com/go-git/go-git/issues/367
					codeCommit.Deletions += stat.Deletion
				}
			}
		}

		err = store.Commits(codeCommit)
		if err != nil {
			return err
		}

		codeRepoCommit := &code.RepoCommit{
			RepoId:    r.id,
			CommitSha: commitSha,
		}
		err = store.RepoCommits(codeRepoCommit)
		if err != nil {
			return err
		}
		if !*taskOpts.SkipCommitFiles {
			if err := r.storeDiffCommitFilesComparedToParent(subtaskCtx, componentMap, commit, taskOpts.ExcludeFileExtensions); err != nil {
				return err
			}
		}
		subtaskCtx.IncProgress(1)
		return nil
	}); err != nil {
		return err
	}
	return
}

func (r *GogitRepoCollector) storeParentCommits(commitSha string, commit *object.Commit) error {
	if commit == nil {
		return nil
	}
	var commitParents []*code.CommitParent
	for i := 0; i < commit.NumParents(); i++ {
		parent, err := commit.Parent(i)
		if err != nil {
			// parent commit might not exist when repo is shallow cloned (tradeoff of supporting timeAfter parameter)
			if err.Error() == "object not found" {
				continue
			}
			return err
		}
		if parent != nil {
			if parentCommitSha := parent.Hash.String(); parentCommitSha != "" {
				commitParents = append(commitParents, &code.CommitParent{
					CommitSha:       commitSha,
					ParentCommitSha: parentCommitSha,
				})
			}
		}
	}
	return r.store.CommitParents(commitParents)
}

func (r *GogitRepoCollector) getCurrentAndParentTree(ctx context.Context, commit *object.Commit) (*object.Tree, *object.Tree, error) {
	if _, err := commit.Stats(); err != nil {
		return nil, nil, err
	}
	commitTree, err := commit.Tree()
	if err != nil {
		return nil, nil, err
	}
	var firstParentTree *object.Tree
	if commit.NumParents() > 0 {
		firstParent, err := commit.Parents().Next()
		if err != nil {
			return nil, nil, err
		}
		firstParentTree, err = firstParent.Tree()
		if err != nil {
			return nil, nil, err
		}
	}
	return commitTree, firstParentTree, nil
}

func (r *GogitRepoCollector) storeDiffCommitFilesComparedToParent(subtaskCtx plugin.SubTaskContext, componentMap map[string]*regexp.Regexp, commit *object.Commit, excludeExts []string) (err error) {
	commitTree, firstParentTree, err := r.getCurrentAndParentTree(subtaskCtx.GetContext(), commit)
	if err != nil {
		return err
	}
	// no parent, doesn't need to patch
	patch, err := firstParentTree.PatchContext(subtaskCtx.GetContext(), commitTree)
	if err != nil {
		return err
	}
	// normalize exclusions
	excluded := map[string]struct{}{}
	for _, ext := range excludeExts {
		e := strings.ToLower(strings.TrimSpace(ext))
		if e == "" {
			continue
		}
		excluded[e] = struct{}{}
	}
	for _, p := range patch.Stats() {
		commitFile := &code.CommitFile{
			CommitSha: commit.Hash.String(),
		}
		fileName := p.Name
		commitFile.FilePath = fileName
		if len(excluded) > 0 {
			lower := strings.ToLower(fileName)
			skip := false
			for ext := range excluded {
				if strings.HasSuffix(lower, ext) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}
		commitFile.Id = genCommitFileId(commitFile.CommitSha, fileName)
		commitFile.Deletions = p.Deletion
		commitFile.Additions = p.Addition
		if err := r.storeCommitFileComponents(subtaskCtx, componentMap, commitFile.Id, commitFile.FilePath); err != nil {
			return err
		}
		err = r.store.CommitFiles(commitFile)
		if err != nil {
			r.logger.Error(err, "CommitFiles error")
			return nil
		}
	}
	return nil
}

// With some long path,the varchar(255) was not enough both ID and file_path
// So we use the hash to compress the path in ID and add length of file_path.
// Use commitSha and the sha256 of FilePath to create id
func genCommitFileId(commitSha, filePath string) string {
	shaFilePath := sha256.New()
	shaFilePath.Write([]byte(filePath))
	return commitSha + ":" + hex.EncodeToString(shaFilePath.Sum(nil))
}

func (r *GogitRepoCollector) storeCommitFileComponents(subtaskCtx plugin.SubTaskContext, componentMap map[string]*regexp.Regexp, commitFileId string, commitFilePath string) error {
	if commitFileId == "" || commitFilePath == "" {
		return errors.Default.New("commit id r commit file path is empty")
	}
	commitFileComponent := &code.CommitFileComponent{
		CommitFileId:  commitFileId,
		ComponentName: "Default",
	}
	for component, reg := range componentMap {
		if reg.MatchString(commitFilePath) {
			commitFileComponent.ComponentName = component
			break
		}
	}
	return r.store.CommitFileComponents(commitFileComponent)
}

// storeRepoSnapshot depends on commit list's order.
func (r *GogitRepoCollector) storeRepoSnapshot(subtaskCtx plugin.SubTaskContext, commitList []*object.Commit) error {
	ctx := subtaskCtx.GetContext()
	snapshot := make(map[string][]string) // {"filePathAndName": ["line1 commit sha", "line2 commit sha"]}
	for _, commit := range commitList {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		commitTree, firstParentTree, err := r.getCurrentAndParentTree(ctx, commit)
		if err != nil {
			return err
		}
		patch, err := firstParentTree.PatchContext(subtaskCtx.GetContext(), commitTree)
		if err != nil {
			return err
		}
		for _, p := range patch.Stats() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			fileName := p.Name
			if _, ok := snapshot[fileName]; !ok {
				snapshot[fileName] = []string{}
			}
			blameResults, err := gogit.Blame(commit, fileName)
			if err != nil {
				return err
			}
			var newBlames []string
			for _, blameResult := range blameResults.Lines {
				newBlames = append(newBlames, blameResult.Hash.String())
			}
			snapshot[fileName] = newBlames
		}
	}
	// store snapshots
	for fileName, lineBlames := range snapshot {
		for idx, lineBlameHash := range lineBlames {
			lineNo := idx + 1
			repoSnapshot := &code.RepoSnapshot{
				RepoId:    r.id,
				CommitSha: lineBlameHash,
				FilePath:  fileName,
				LineNo:    lineNo,
			}
			if err := r.store.RepoSnapshot(repoSnapshot); err != nil {
				r.logger.Error(err, "store RepoSnapshot error")
				return err
			}
		}
	}
	return nil
}

func (r *GogitRepoCollector) GetCommitList(subtaskCtx plugin.SubTaskContext) ([]*object.Commit, error) {
	var commitList []*object.Commit
	// get current head commit sha, default is master branch
	// check branch, if not master, checkout to branch's head
	commitOid, err := r.repo.Head()
	if err != nil {
		return nil, err
	}
	// get head commit object and add into commitList
	commit, err := r.repo.CommitObject(commitOid.Hash())
	if err != nil {
		return nil, err
	}
	commitList = append(commitList, commit)
	// if current head has parents, get parent commit sha
	for commit != nil && commit.NumParents() > 0 {
		parentCommit, err := commit.Parent(0)
		if err != nil {
			return nil, err
		}
		commit, err = r.repo.CommitObject(parentCommit.Hash)
		if err != nil {
			return nil, err
		}
		commitList = append(commitList, commit)
	}
	// reverse commitList
	// use slices.Reverse(commitList) in higher golang version.
	for i, j := 0, len(commitList)-1; i < j; i, j = i+1, j-1 {
		commitList[i], commitList[j] = commitList[j], commitList[i]
	}
	return commitList, nil
}

func (r *GogitRepoCollector) CollectDiffLine(subtaskCtx plugin.SubTaskContext) error {
	commitList, err := r.GetCommitList(subtaskCtx)
	if err != nil {
		return err
	}
	if err := r.storeRepoSnapshot(subtaskCtx, commitList); err != nil {
		return err
	}
	// fixme: collecting CommitLineChange is not implemented.
	// There is no way to get such information with go-git, and table commit_line_change is not used by any dashboards
	// So we just ignore it.
	return nil
}
