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
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/log"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/gitextractor/models"
)

// minGitVersion is the first release with --diff-merges=first-parent, which is
// required to diff merge commits against their first parent only.
const minGitVersion = 2*10000 + 31*100

// commitBatchSize bounds how many commits are fetched and diffed at a time so
// that a repository with a long history does not need to be held in memory.
const commitBatchSize = 1024

var _ RepoCollector = (*GitcliRepoCollector)(nil)

// GitcliRepoCollector collects git data with the git command line. It replaces
// the previous libgit2 and go-git implementations, so the server no longer
// needs a git library or cgo.
type GitcliRepoCollector struct {
	id      string
	logger  log.Logger
	store   models.Store
	git     *gitRunner
	cleanup func()
}

func NewGitcliRepoCollector(localDir string, repoId string, store models.Store, logger log.Logger) (*GitcliRepoCollector, errors.Error) {
	git := newGitRunner(localDir)
	if _, err := git.Output(context.Background(), "rev-parse", "--git-dir"); err != nil {
		return nil, errors.Convert(err)
	}
	version, err := gitVersion(git)
	if err != nil {
		return nil, errors.Convert(err)
	}
	if version < minGitVersion {
		return nil, errors.Default.New(fmt.Sprintf(
			"git %s is not supported, gitextractor requires git 2.31 or newer", formatGitVersion(version)))
	}
	return &GitcliRepoCollector{
		id:     repoId,
		logger: logger,
		store:  store,
		git:    git,
	}, nil
}

func (r *GitcliRepoCollector) SetCleanUp(f func()) error {
	if f != nil {
		r.cleanup = f
	}
	return nil
}

func (r *GitcliRepoCollector) Close(ctx context.Context) error {
	defer func() {
		if r.cleanup != nil {
			r.cleanup()
		}
	}()
	return r.store.Close()
}

// CollectAll The main parser subtask
func (r *GitcliRepoCollector) CollectAll(subtaskCtx plugin.SubTaskContext) error {
	subtaskCtx.SetProgress(0, -1)
	if err := r.CollectTags(subtaskCtx); err != nil {
		return err
	}
	if err := r.CollectBranches(subtaskCtx); err != nil {
		return err
	}
	if err := r.CollectCommits(subtaskCtx); err != nil {
		return err
	}
	if *subtaskCtx.GetData().(*GitExtractorTaskData).Options.SkipCommitStat {
		return nil
	}
	return r.CollectDiffLine(subtaskCtx)
}

// CountTags Count git tags subtask
func (r *GitcliRepoCollector) CountTags(ctx context.Context) (int, error) {
	tags, err := r.tagRefs(ctx)
	if err != nil {
		return 0, err
	}
	return len(tags), nil
}

// CountBranches count the number of branches in a git repo
func (r *GitcliRepoCollector) CountBranches(ctx context.Context) (int, error) {
	branches, err := r.branchRefs(ctx)
	if err != nil {
		return 0, err
	}
	head, err := r.headRef(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, branch := range branches {
		if branch.refname != head {
			count++
		}
	}
	return count, nil
}

// CountCommits count the number of commits in a git repo
func (r *GitcliRepoCollector) CountCommits(ctx context.Context) (int, error) {
	out, err := r.git.Output(ctx, "rev-list", "--all", "--count")
	if err != nil {
		return 0, err
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("unexpected commit count %q", strings.TrimSpace(string(out)))
	}
	return count, nil
}

// CollectTags Collect Tags data
func (r *GitcliRepoCollector) CollectTags(subtaskCtx plugin.SubTaskContext) error {
	tags, err := r.tagRefs(subtaskCtx.GetContext())
	if err != nil {
		return err
	}
	for _, tag := range tags {
		commitSha := tag.peeledSha
		if commitSha == "" {
			commitSha = tag.sha
		}
		ref := &code.Ref{
			DomainEntityExtended: domainlayer.DomainEntityExtended{Id: fmt.Sprintf("%s:%s", r.id, tag.refname)},
			RepoId:               r.id,
			Name:                 tag.refname,
			CommitSha:            commitSha,
			RefType:              TAG,
		}
		if err := r.store.Refs(ref); err != nil {
			return err
		}
		subtaskCtx.IncProgress(1)
	}
	return nil
}

// CollectBranches Collect branch data
func (r *GitcliRepoCollector) CollectBranches(subtaskCtx plugin.SubTaskContext) error {
	branches, err := r.branchRefs(subtaskCtx.GetContext())
	if err != nil {
		return err
	}
	head, err := r.headRef(subtaskCtx.GetContext())
	if err != nil {
		return err
	}
	for _, branch := range branches {
		ref := &code.Ref{
			DomainEntityExtended: domainlayer.DomainEntityExtended{Id: fmt.Sprintf("%s:%s", r.id, branch.name)},
			RepoId:               r.id,
			Name:                 branch.name,
			CommitSha:            branch.sha,
			RefType:              BRANCH,
			IsDefault:            branch.refname == head,
		}
		if err := r.store.Refs(ref); err != nil {
			return err
		}
		subtaskCtx.IncProgress(1)
	}
	return nil
}

// CollectCommits Collect data from each commit, we can also get the diff line
func (r *GitcliRepoCollector) CollectCommits(subtaskCtx plugin.SubTaskContext) (err error) {
	ctx := subtaskCtx.GetContext()
	taskOpts := subtaskCtx.GetData().(*GitExtractorTaskData).Options

	shas, err := r.git.OutputLines(ctx, "rev-list", "--all")
	if err != nil {
		return err
	}
	if len(shas) == 0 {
		return nil
	}
	present := make(map[string]struct{}, len(shas))
	for _, sha := range shas {
		present[sha] = struct{}{}
	}
	shallow, err := r.shallowCommits(ctx)
	if err != nil {
		return err
	}
	var componentMap map[string]*regexp.Regexp
	if !*taskOpts.SkipCommitFiles {
		componentMap, err = r.componentMap(subtaskCtx)
		if err != nil {
			return err
		}
	}
	excluded := normalizeExtensions(taskOpts.ExcludeFileExtensions)

	for start := 0; start < len(shas); start += commitBatchSize {
		end := start + commitBatchSize
		if end > len(shas) {
			end = len(shas)
		}
		batch := shas[start:end]

		var statsBySha map[string]*commitStats
		if !*taskOpts.SkipCommitStat {
			statsBySha, err = r.commitStats(ctx, batch)
			if err != nil {
				return err
			}
		}
		commits, err := r.readCommits(ctx, batch)
		if err != nil {
			return err
		}
		for _, metadata := range commits {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// A shallow boundary commit's parent is not in the object
			// database. Diffing it would produce a full-tree diff and inflate
			// the addition count, so it is skipped entirely.
			if _, ok := shallow[metadata.Sha]; ok {
				r.logger.Info("skip commit %s because it is a shallow boundary", metadata.Sha)
				continue
			}
			// The same guard as above, for repositories where the boundary is
			// not recorded in the shallow file.
			if len(metadata.Parents) > 0 {
				if _, ok := present[metadata.Parents[0]]; !ok {
					r.logger.Info("skip commit %s because it has no parent commit", metadata.Sha)
					continue
				}
			}

			commit := &code.Commit{
				Sha:            metadata.Sha,
				Message:        metadata.Message,
				AuthorName:     metadata.Author.Name,
				AuthorEmail:    metadata.Author.Email,
				AuthorId:       metadata.Author.Email,
				AuthoredDate:   metadata.Author.When,
				CommitterName:  metadata.Committer.Name,
				CommitterEmail: metadata.Committer.Email,
				CommitterId:    metadata.Committer.Email,
				CommittedDate:  metadata.Committer.When,
			}

			var files []commitFileStat
			if stats := statsBySha[metadata.Sha]; stats != nil {
				files = stats.Files
			}
			for _, file := range files {
				if isExcludedPath(file.Path, excluded) {
					continue
				}
				commit.Additions += file.Additions
				commit.Deletions += file.Deletions
			}

			if err := r.storeParentCommits(metadata, present); err != nil {
				return err
			}
			if err := r.store.Commits(commit); err != nil {
				return err
			}
			if err := r.store.RepoCommits(&code.RepoCommit{RepoId: r.id, CommitSha: commit.Sha}); err != nil {
				return err
			}

			if !*taskOpts.SkipCommitFiles {
				for _, file := range files {
					if isExcludedPath(file.Path, excluded) {
						continue
					}
					if err := r.storeCommitFile(commit.Sha, file, componentMap); err != nil {
						return err
					}
				}
			}
			subtaskCtx.IncProgress(1)
		}
	}
	return nil
}

func (r *GitcliRepoCollector) storeParentCommits(commit *parsedCommit, present map[string]struct{}) errors.Error {
	var parents []*code.CommitParent
	for _, parentSha := range commit.Parents {
		// Skip parents that are not in the object database, which happens for
		// shallow clones, instead of storing a dangling reference.
		if _, ok := present[parentSha]; !ok {
			continue
		}
		parents = append(parents, &code.CommitParent{
			CommitSha:       commit.Sha,
			ParentCommitSha: parentSha,
		})
	}
	return r.store.CommitParents(parents)
}

// storeCommitFile stores one changed file of a commit. Component rows are only
// written when the repository has components configured: without them every row
// would be "Default" for every changed file, which is pure write amplification.
func (r *GitcliRepoCollector) storeCommitFile(commitSha string, file commitFileStat, componentMap map[string]*regexp.Regexp) errors.Error {
	commitFile := &code.CommitFile{
		CommitSha: commitSha,
		FilePath:  file.Path,
		Additions: file.Additions,
		Deletions: file.Deletions,
	}
	commitFile.Id = genCommitFileId(commitFile.CommitSha, commitFile.FilePath)

	if len(componentMap) > 0 {
		componentName := "Default"
		for _, name := range sortedComponentNames(componentMap) {
			if componentMap[name].MatchString(commitFile.FilePath) {
				componentName = name
				break
			}
		}
		if err := r.store.CommitFileComponents(&code.CommitFileComponent{
			CommitFileId:  commitFile.Id,
			ComponentName: componentName,
		}); err != nil {
			return err
		}
	}
	return r.store.CommitFiles(commitFile)
}

// CollectDiffLine replays the first-parent history of HEAD and records the
// origin commit of every line, plus the per-line change rows.
func (r *GitcliRepoCollector) CollectDiffLine(subtaskCtx plugin.SubTaskContext) error {
	ctx := subtaskCtx.GetContext()

	if _, err := r.git.Output(ctx, "rev-parse", "--verify", "HEAD"); err != nil {
		// an empty repository has no HEAD and therefore no snapshot
		r.logger.Info("skip diff line collection because the repository has no HEAD")
		return nil
	}

	reader, wait, err := r.git.Stream(ctx,
		"log", "--first-parent", "--reverse", "--no-renames", "-p", "-U3",
		"--format="+patchRecordMarker+"%H",
		"--no-color", "--no-ext-diff", "--no-show-signature", "--no-notes",
		"HEAD",
	)
	if err != nil {
		return err
	}

	snapshot := make(map[string]*models.FileBlame)
	parseErr := parsePatchStream(reader, func(patch *commitPatch) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		return r.storeSnapshotForCommit(patch, snapshot)
	})
	waitErr := wait()
	if parseErr != nil {
		return parseErr
	}
	if waitErr != nil {
		return waitErr
	}

	db := subtaskCtx.GetDal()
	if err := db.Delete(&code.RepoSnapshot{}, dal.Where("repo_id= ?", r.id)); err != nil {
		return err
	}

	files := make([]string, 0, len(snapshot))
	for file := range snapshot {
		files = append(files, file)
	}
	sort.Strings(files)
	for _, file := range files {
		lineNo := 0
		for element := snapshot[file].Lines.Front(); element != nil; element = element.Next() {
			lineNo++
			commitSha, ok := element.Value.(string)
			if !ok {
				// A placeholder without a commit should not happen, but do not
				// fail the whole collection if it does.
				r.logger.Warn(nil, "empty snapshot line for %s:%d", file, lineNo)
				continue
			}
			if err := r.store.RepoSnapshot(&code.RepoSnapshot{
				RepoId:    r.id,
				CommitSha: commitSha,
				FilePath:  file,
				LineNo:    lineNo,
			}); err != nil {
				return err
			}
		}
	}
	r.logger.Info("collect snapshot finished")
	return nil
}

func (r *GitcliRepoCollector) storeSnapshotForCommit(patch *commitPatch, snapshot map[string]*models.FileBlame) error {
	pending := &snapshotChanges{}
	lastFile := ""

	for _, file := range patch.Files {
		if _, ok := snapshot[file.OldPath]; !ok {
			fileBlame, err := models.NewFileBlame()
			if err != nil {
				return err
			}
			snapshot[file.OldPath] = fileBlame
		}
		if lastFile == "" {
			lastFile = file.NewPath
		} else if lastFile != file.NewPath {
			applySnapshotChanges(snapshot, lastFile, patch.Sha, pending)
			pending = &snapshotChanges{}
			lastFile = file.NewPath
		}

		for _, line := range file.Lines {
			change := &code.CommitLineChange{
				CommitSha:   patch.Sha,
				ChangedType: lineChangeType(line.Origin),
				LineNoNew:   line.NewLine,
				LineNoOld:   line.OldLine,
				OldFilePath: file.OldPath,
				NewFilePath: file.NewPath,
				HunkNum:     line.HunkNum,
				Id:          fmt.Sprintf("%s:%s:%d:%d", patch.Sha, file.NewPath, line.OldLine, line.NewLine),
			}
			switch line.Origin {
			case patchChangeAddition:
				pending.added = append(pending.added, line)
			case patchChangeDeletion:
				if blame := snapshot[file.OldPath]; blame != nil {
					if element := blame.Find(line.OldLine); element != nil && element.Value != nil {
						change.PrevCommit = element.Value.(string)
					}
				}
				pending.deleted = append(pending.deleted, line)
			}
			if err := r.store.CommitLineChange(change); err != nil {
				return err
			}
		}
	}
	applySnapshotChanges(snapshot, lastFile, patch.Sha, pending)
	return nil
}

type snapshotChanges struct {
	deleted []patchLine
	added   []patchLine
}

// applySnapshotChanges applies the changes of one file to the line snapshot:
// deletions remove the old line numbers and additions insert the commit sha at
// the new line numbers.
func applySnapshotChanges(snapshot map[string]*models.FileBlame, filePath, commitSha string, changes *snapshotChanges) {
	fileBlame, ok := snapshot[filePath]
	if !ok || fileBlame == nil || (len(changes.deleted) == 0 && len(changes.added) == 0) {
		return
	}
	sort.SliceStable(changes.deleted, func(i, j int) bool {
		return changes.deleted[i].OldLine > changes.deleted[j].OldLine
	})
	for _, line := range changes.deleted {
		fileBlame.RemoveLine(line.OldLine)
	}
	for _, line := range changes.added {
		fileBlame.AddLine(line.NewLine, commitSha)
	}
}

func lineChangeType(origin byte) string {
	switch origin {
	case patchChangeAddition:
		return "Addition"
	case patchChangeDeletion:
		return "Deletion"
	default:
		return "Context"
	}
}

// readCommits returns the commit objects of the batch in the order of the batch.
func (r *GitcliRepoCollector) readCommits(ctx context.Context, shas []string) ([]*parsedCommit, error) {
	stdin := []byte(strings.Join(shas, "\n") + "\n")
	reader, wait, err := r.git.Pipe(ctx, stdin, "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	buffered := bufio.NewReaderSize(reader, 256*1024)
	commits := make([]*parsedCommit, 0, len(shas))
	for {
		sha, content, readErr := readCatFileObject(buffered)
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = wait()
			return nil, readErr
		}
		commit, parseErr := parseCommitObject(sha, content)
		if parseErr != nil {
			_ = wait()
			return nil, parseErr
		}
		commits = append(commits, commit)
	}
	if err := wait(); err != nil {
		return nil, err
	}
	return commits, nil
}

// commitStats returns the per-file line counts of the batch.
func (r *GitcliRepoCollector) commitStats(ctx context.Context, shas []string) (map[string]*commitStats, error) {
	stdin := []byte(strings.Join(shas, "\n") + "\n")
	reader, wait, err := r.git.Pipe(ctx, stdin,
		"log", "--no-walk=unsorted", "--stdin", "--root", "--diff-merges=first-parent",
		"--numstat", "--no-renames", "-z", "--format="+patchRecordMarker+"%H",
		"--no-color", "--no-ext-diff", "--no-show-signature", "--no-notes",
	)
	if err != nil {
		return nil, err
	}
	statsBySha := make(map[string]*commitStats, len(shas))
	err = parseNumstatStream(reader, func(stats *commitStats) error {
		statsBySha[stats.Sha] = stats
		return nil
	})
	waitErr := wait()
	if err != nil {
		return nil, err
	}
	if waitErr != nil {
		return nil, waitErr
	}
	return statsBySha, nil
}

// shallowCommits returns the commits recorded as shallow boundaries.
func (r *GitcliRepoCollector) shallowCommits(ctx context.Context) (map[string]struct{}, error) {
	shallow := make(map[string]struct{})
	out, err := r.git.Output(ctx, "rev-parse", "--git-path", "shallow")
	if err != nil {
		return shallow, err
	}
	content, err := os.ReadFile(resolveRepoPath(r.git.dir, strings.TrimSpace(string(out))))
	if os.IsNotExist(err) {
		return shallow, nil
	}
	if err != nil {
		return shallow, err
	}
	for _, line := range strings.Split(string(content), "\n") {
		if sha := strings.TrimSpace(line); sha != "" {
			shallow[sha] = struct{}{}
		}
	}
	return shallow, nil
}

// resolveRepoPath turns a git path that is relative to the repository into an
// absolute one. `git rev-parse --git-path` returns paths relative to the
// repository root, but the collector's working directory is not the repository.
func resolveRepoPath(repoDir, gitPath string) string {
	if filepath.IsAbs(gitPath) {
		return gitPath
	}
	return filepath.Join(repoDir, gitPath)
}

type tagRef struct {
	refname   string
	sha       string
	peeledSha string
}

func (r *GitcliRepoCollector) tagRefs(ctx context.Context) ([]tagRef, error) {
	lines, err := r.git.OutputLines(ctx, "for-each-ref",
		"--format=%(refname)%00%(objectname)%00%(*objectname)",
		"refs/tags")
	if err != nil {
		return nil, err
	}
	tags := make([]tagRef, 0, len(lines))
	for _, line := range lines {
		fields := strings.SplitN(line, "\x00", 3)
		if len(fields) != 3 {
			return nil, fmt.Errorf("unexpected tag ref %q", line)
		}
		tags = append(tags, tagRef{refname: fields[0], sha: fields[1], peeledSha: fields[2]})
	}
	return tags, nil
}

type branchRef struct {
	refname string
	name    string
	sha     string
}

func (r *GitcliRepoCollector) branchRefs(ctx context.Context) ([]branchRef, error) {
	lines, err := r.git.OutputLines(ctx, "for-each-ref",
		"--format=%(refname)%00%(objectname)",
		"refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	branches := make([]branchRef, 0, len(lines))
	for _, line := range lines {
		fields := strings.SplitN(line, "\x00", 2)
		if len(fields) != 2 {
			return nil, fmt.Errorf("unexpected branch ref %q", line)
		}
		name := strings.TrimPrefix(fields[0], "refs/heads/")
		name = strings.TrimPrefix(name, "refs/remotes/")
		branches = append(branches, branchRef{refname: fields[0], name: name, sha: fields[1]})
	}
	return branches, nil
}

func (r *GitcliRepoCollector) headRef(ctx context.Context) (string, error) {
	out, err := r.git.Output(ctx, "symbolic-ref", "-q", "HEAD")
	if err != nil {
		// a detached HEAD has no default branch
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *GitcliRepoCollector) componentMap(subtaskCtx plugin.SubTaskContext) (map[string]*regexp.Regexp, error) {
	db := subtaskCtx.GetDal()
	components := make([]code.Component, 0)
	if err := db.All(&components, dal.From(components), dal.Where("repo_id= ?", r.id)); err != nil {
		return nil, err
	}
	componentMap := make(map[string]*regexp.Regexp, len(components))
	for _, component := range components {
		componentMap[component.Name] = regexp.MustCompile(component.PathRegex)
	}
	return componentMap, nil
}

func sortedComponentNames(componentMap map[string]*regexp.Regexp) []string {
	names := make([]string, 0, len(componentMap))
	for name := range componentMap {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// genCommitFileId builds the commit file primary key. With some long paths the
// varchar(255) columns were not enough for both the id and the file path, so
// the path is compressed with sha256.
func genCommitFileId(commitSha, filePath string) string {
	shaFilePath := sha256.New()
	shaFilePath.Write([]byte(filePath))
	return commitSha + ":" + hex.EncodeToString(shaFilePath.Sum(nil))
}

func normalizeExtensions(extensions []string) map[string]struct{} {
	excluded := make(map[string]struct{}, len(extensions))
	for _, extension := range extensions {
		value := strings.ToLower(strings.TrimSpace(extension))
		if value == "" {
			continue
		}
		excluded[value] = struct{}{}
	}
	return excluded
}

func isExcludedPath(path string, excluded map[string]struct{}) bool {
	if len(excluded) == 0 {
		return false
	}
	lower := strings.ToLower(path)
	for extension := range excluded {
		if strings.HasSuffix(lower, extension) {
			return true
		}
	}
	return false
}

// gitVersion returns the version of git as major*10000 + minor*100 + patch.
func gitVersion(git *gitRunner) (int, error) {
	out, err := git.Output(context.Background(), "version")
	if err != nil {
		return 0, err
	}
	return parseGitVersion(string(out))
}

func parseGitVersion(output string) (int, error) {
	// e.g. "git version 2.43.0", "git version 2.39.2.windows.1"
	fields := strings.Fields(output)
	if len(fields) < 3 || fields[0] != "git" || fields[1] != "version" {
		return 0, fmt.Errorf("unexpected git version output %q", strings.TrimSpace(output))
	}
	parts := strings.SplitN(fields[2], ".", 3)
	if len(parts) < 2 {
		return 0, fmt.Errorf("unexpected git version %q", fields[2])
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("unexpected git version %q", fields[2])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("unexpected git version %q", fields[2])
	}
	patch := 0
	if len(parts) == 3 {
		if idx := strings.IndexFunc(parts[2], func(r rune) bool { return r < '0' || r > '9' }); idx >= 0 {
			parts[2] = parts[2][:idx]
		}
		if parts[2] != "" {
			patch, _ = strconv.Atoi(parts[2])
		}
	}
	return major*10000 + minor*100 + patch, nil
}

func formatGitVersion(version int) string {
	return fmt.Sprintf("%d.%d.%d", version/10000, version%10000/100, version%100)
}
