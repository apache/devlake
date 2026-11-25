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
	"io"
	"strings"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
)

const maxMemory = 32 << 20 // 32 MB

// ImportIssue accepts a CSV file, parses and saves it to the database
// @Summary      Upload issues.csv file
// @Description  Upload issues.csv file. 3 tables(boards, issues, board_issues) would be affected.
// @Tags 		 plugins/customize
// @Accept       multipart/form-data
// @Param        boardId formData string true "the ID of the board"
// @Param        boardName formData string true "the name of the board"
// @Param        incremental formData bool false "whether to import incrementally"
// @Param        file formData file true "select file to upload"
// @Produce      json
// @Success      200
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router       /plugins/customize/csvfiles/issues.csv [post]
func (h *Handlers) ImportIssue(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	file, err := h.extractFile(input)
	if err != nil {
		return nil, err
	}
	// nolint
	defer file.Close()
	incremental := false
	if input.Request.FormValue("incremental") == "true" {
		incremental = true
	}
	boardId := strings.TrimSpace(input.Request.FormValue("boardId"))
	if boardId == "" {
		return nil, errors.BadInput.New("empty boardId")
	}
	boardName := strings.TrimSpace(input.Request.FormValue("boardName"))
	if boardName == "" {
		return nil, errors.BadInput.New("empty boardName")
	}
	return nil, h.svc.ImportIssueToBoard(boardId, boardName, file, incremental)
}

// ImportIssueCommit accepts a CSV file, parses and saves it to the database
// @Summary      Upload issue_commits.csv file
// @Description  Upload issue_commits.csv file
// @Tags 		 plugins/customize
// @Accept       multipart/form-data
// @Param        boardId formData string true "the ID of the board"
// @Param        file formData file true "select file to upload"
// @Produce      json
// @Success      200
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router       /plugins/customize/csvfiles/issue_commits.csv [post]
func (h *Handlers) ImportIssueCommit(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	file, err := h.extractFile(input)
	if err != nil {
		return nil, err
	}
	// nolint
	defer file.Close()
	boardId := strings.TrimSpace(input.Request.FormValue("boardId"))
	if boardId == "" {
		return nil, errors.Default.New("empty boardId")
	}
	return nil, h.svc.ImportIssueCommit(boardId, file)
}

// ImportIssueRepoCommit accepts a CSV file, parses and saves it to the database
// @Summary      Upload issue_repo_commits.csv file
// @Description  Upload issue_repo_commits.csv file
// @Tags 		 plugins/customize
// @Accept       multipart/form-data
// @Param        boardId formData string true "the ID of the board"
// @Param        incremental formData bool false "whether to import incrementally"
// @Param        file formData file true "select file to upload"
// @Produce      json
// @Success      200
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router       /plugins/customize/csvfiles/issue_repo_commits.csv [post]
func (h *Handlers) ImportIssueRepoCommit(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	file, err := h.extractFile(input)
	if err != nil {
		return nil, err
	}
	// nolint
	defer file.Close()
	boardId := strings.TrimSpace(input.Request.FormValue("boardId"))
	if boardId == "" {
		return nil, errors.Default.New("empty boardId")
	}
	incremental := false
	if input.Request.FormValue("incremental") == "true" {
		incremental = true
	}
	return nil, h.svc.ImportIssueRepoCommit(boardId, file, incremental)
}

// ImportSprint accepts a CSV file, parses and saves it to the database
// @Summary      Upload sprints.csv file
// @Description  Upload sprints.csv file
// @Tags 		 plugins/customize
// @Accept       multipart/form-data
// @Param        boardId formData string true "the ID of the board"
// @Param        file formData file true "select file to upload"
// @Param        incremental formData string true "whether to save only new data"
// @Produce      json
// @Success      200
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router       /plugins/customize/csvfiles/sprints.csv [post]
func (h *Handlers) ImportSprint(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	file, err := h.extractFile(input)
	if err != nil {
		return nil, err
	}
	// nolint
	defer file.Close()
	boardId := strings.TrimSpace(input.Request.FormValue("boardId"))
	if boardId == "" {
		return nil, errors.Default.New("empty boardId")
	}
	incremental := false
	if input.Request.FormValue("incremental") == "true" {
		incremental = true
	}
	return nil, h.svc.ImportSprint(boardId, file, incremental)
}

// ImportIssueChangelog accepts a CSV file, parses and saves it to the database
// @Summary      Upload issue_changelogs.csv file
// @Description  Upload issue_changelogs.csv file
// @Tags 		 plugins/customize
// @Accept       multipart/form-data
// @Param        boardId formData string true "the ID of the board"
// @Param        file formData file true "select file to upload"
// @Param 		 incremental formData boolean false "Whether to incrementally update changelogs" default(false)
// @Produce      json
// @Success      200
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router       /plugins/customize/csvfiles/issue_changelogs.csv [post]
func (h *Handlers) ImportIssueChangelog(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	file, err := h.extractFile(input)
	if err != nil {
		return nil, err
	}
	// nolint
	defer file.Close()
	boardId := strings.TrimSpace(input.Request.FormValue("boardId"))
	if boardId == "" {
		return nil, errors.Default.New("empty boardId")
	}
	incremental := false
	if input.Request.FormValue("incremental") == "true" {
		incremental = true
	}
	return nil, h.svc.ImportIssueChangelog(boardId, file, incremental)
}

// ImportIssueWorklog accepts a CSV file, parses and saves it to the database
// @Summary      Upload issue_worklogs.csv file
// @Description  Upload issue_worklogs.csv file
// @Tags 		 plugins/customize
// @Accept       multipart/form-data
// @Param        boardId formData string true "the ID of the board"
// @Param        file formData file true "select file to upload"
// @Param        incremental formData boolean false "Whether to do incremental sync (default false)"
// @Produce      json
// @Success      200
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router       /plugins/customize/csvfiles/issue_worklogs.csv [post]
func (h *Handlers) ImportIssueWorklog(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	file, err := h.extractFile(input)
	if err != nil {
		return nil, err
	}
	// nolint
	defer file.Close()
	boardId := strings.TrimSpace(input.Request.FormValue("boardId"))
	if boardId == "" {
		return nil, errors.Default.New("empty boardId")
	}
	incremental := false
	if input.Request.FormValue("incremental") == "true" {
		incremental = true
	}
	return nil, h.svc.ImportIssueWorklog(boardId, file, incremental)
}

func (h *Handlers) extractFile(input *plugin.ApiResourceInput) (io.ReadCloser, errors.Error) {
	if input.Request == nil {
		return nil, errors.Default.New("request is nil")
	}
	if input.Request.MultipartForm == nil {
		if err := input.Request.ParseMultipartForm(maxMemory); err != nil {
			return nil, errors.Convert(err)
		}
	}
	f, fh, err := input.Request.FormFile("file")
	if err != nil {
		return nil, errors.Convert(err)
	}
	// nolint
	f.Close()
	file, err := fh.Open()
	if err != nil {
		return nil, errors.Convert(err)
	}
	return file, nil
}
