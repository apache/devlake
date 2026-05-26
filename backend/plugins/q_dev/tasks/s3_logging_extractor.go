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
	"compress/gzip"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/q_dev/models"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

var _ plugin.SubTaskEntryPoint = ExtractQDevLoggingData

const (
	loggingBatchSize   = 50 // number of files to process per DB transaction
	s3DownloadWorkers  = 10 // parallel S3 download goroutines
	s3DownloadChanSize = 20 // buffered channel size for download results
)

// downloadResult holds the parsed records from one S3 file
type downloadResult struct {
	FileMeta *models.QDevS3FileMeta
	ChatLogs []*models.QDevChatLog
	CompLogs []*models.QDevCompletionLog
	Err      error
}

// ExtractQDevLoggingData extracts logging data from S3 JSON.gz files
func ExtractQDevLoggingData(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*QDevTaskData)
	db := taskCtx.GetDal()

	cursor, err := db.Cursor(
		dal.From(&models.QDevS3FileMeta{}),
		dal.Where("connection_id = ? AND processed = ? AND file_name LIKE ?",
			data.Options.ConnectionId, false, "%.json.gz"),
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to get logging file metadata cursor")
	}
	defer cursor.Close()

	// Collect all file metas first
	var fileMetas []*models.QDevS3FileMeta
	for cursor.Next() {
		fm := &models.QDevS3FileMeta{}
		if err := db.Fetch(cursor, fm); err != nil {
			return errors.Default.Wrap(err, "failed to fetch file metadata")
		}
		fileMetas = append(fileMetas, fm)
	}

	if len(fileMetas) == 0 {
		return nil
	}

	taskCtx.SetProgress(0, len(fileMetas))
	taskCtx.GetLogger().Info("Processing %d logging files with %d workers", len(fileMetas), s3DownloadWorkers)

	// Display name cache to avoid repeated IAM calls
	displayNameCache := &sync.Map{}

	// Process in batches
	for batchStart := 0; batchStart < len(fileMetas); batchStart += loggingBatchSize {
		batchEnd := batchStart + loggingBatchSize
		if batchEnd > len(fileMetas) {
			batchEnd = len(fileMetas)
		}
		batch := fileMetas[batchStart:batchEnd]

		// Parallel download and parse
		results := parallelDownloadAndParse(taskCtx, data, batch, displayNameCache)

		// Check for download errors
		for _, r := range results {
			if r.Err != nil {
				return errors.Default.Wrap(errors.Convert(r.Err),
					"failed to download/parse "+r.FileMeta.FileName)
			}
		}

		// Batch write to DB in a single transaction
		tx := db.Begin()
		var txErr errors.Error
		for _, r := range results {
			for _, chatLog := range r.ChatLogs {
				if txErr = tx.CreateOrUpdate(chatLog); txErr != nil {
					break
				}
			}
			if txErr != nil {
				break
			}
			for _, compLog := range r.CompLogs {
				if txErr = tx.CreateOrUpdate(compLog); txErr != nil {
					break
				}
			}
			if txErr != nil {
				break
			}
			r.FileMeta.Processed = true
			now := time.Now()
			r.FileMeta.ProcessedTime = &now
			if txErr = tx.Update(r.FileMeta); txErr != nil {
				break
			}
		}
		if txErr != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				taskCtx.GetLogger().Error(rbErr, "failed to rollback transaction")
			}
			return errors.Default.Wrap(txErr, "failed to write logging batch")
		}
		if err := tx.Commit(); err != nil {
			return errors.Default.Wrap(err, "failed to commit batch")
		}

		taskCtx.IncProgress(len(batch))
	}

	return nil
}

// parallelDownloadAndParse downloads and parses S3 files concurrently
func parallelDownloadAndParse(
	taskCtx plugin.SubTaskContext,
	data *QDevTaskData,
	fileMetas []*models.QDevS3FileMeta,
	displayNameCache *sync.Map,
) []downloadResult {
	results := make([]downloadResult, len(fileMetas))
	jobs := make(chan int, s3DownloadChanSize)
	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < s3DownloadWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				fm := fileMetas[idx]
				result := downloadAndParseFile(taskCtx, data, fm, displayNameCache)
				results[idx] = result
			}
		}()
	}

	// Send jobs
	for i := range fileMetas {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return results
}

// downloadAndParseFile downloads one S3 file and parses it into model records
func downloadAndParseFile(
	taskCtx plugin.SubTaskContext,
	data *QDevTaskData,
	fileMeta *models.QDevS3FileMeta,
	displayNameCache *sync.Map,
) downloadResult {
	result := downloadResult{FileMeta: fileMeta}

	getResult, err := data.S3Client.S3.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(data.S3Client.Bucket),
		Key:    aws.String(fileMeta.S3Path),
	})
	if err != nil {
		result.Err = err
		return result
	}
	defer getResult.Body.Close()

	gzReader, err := gzip.NewReader(getResult.Body)
	if err != nil {
		result.Err = err
		return result
	}
	defer gzReader.Close()

	var logFile loggingFile
	if err := json.NewDecoder(gzReader).Decode(&logFile); err != nil {
		result.Err = err
		return result
	}

	isChatLog := strings.Contains(fileMeta.S3Path, "GenerateAssistantResponse")

	for _, rawRecord := range logFile.Records {
		if isChatLog {
			chatLog, err := parseChatRecord(rawRecord, fileMeta, data.IdentityClient, displayNameCache)
			if err != nil {
				result.Err = err
				return result
			}
			if chatLog != nil {
				result.ChatLogs = append(result.ChatLogs, chatLog)
			}
		} else {
			compLog, err := parseCompletionRecord(rawRecord, fileMeta, data.IdentityClient, displayNameCache)
			if err != nil {
				result.Err = err
				return result
			}
			if compLog != nil {
				result.CompLogs = append(result.CompLogs, compLog)
			}
		}
	}

	return result
}

// cachedResolveDisplayName resolves display name with caching
func cachedResolveDisplayName(userId string, identityClient UserDisplayNameResolver, cache *sync.Map) string {
	if v, ok := cache.Load(userId); ok {
		return v.(string)
	}
	if identityClient == nil {
		cache.Store(userId, userId)
		return userId
	}
	displayName, err := identityClient.ResolveUserDisplayName(userId)
	if err != nil || displayName == "" {
		cache.Store(userId, userId)
		return userId
	}
	cache.Store(userId, displayName)
	return displayName
}

// JSON structures for logging data

type loggingFile struct {
	Records []json.RawMessage `json:"records"`
}

type chatLogRecord struct {
	Request  *chatLogRequest  `json:"generateAssistantResponseEventRequest"`
	Response *chatLogResponse `json:"generateAssistantResponseEventResponse"`
}

type chatLogRequest struct {
	UserID           string  `json:"userId"`
	Timestamp        string  `json:"timeStamp"`
	ChatTriggerType  string  `json:"chatTriggerType"`
	CustomizationArn *string `json:"customizationArn"`
	ModelID          string  `json:"modelId"`
	Prompt           string  `json:"prompt"`
}

type chatLogResponse struct {
	RequestID         string `json:"requestId"`
	AssistantResponse string `json:"assistantResponse"`
	FollowupPrompts   string `json:"followupPrompts"`
	MessageMetadata   struct {
		ConversationID *string `json:"conversationId"`
		UtteranceID    *string `json:"utteranceId"`
	} `json:"messageMetadata"`
	CodeReferenceEvents        []json.RawMessage `json:"codeReferenceEvents"`
	SupplementaryWebLinksEvent []json.RawMessage `json:"supplementaryWebLinksEvent"`
}

type completionLogRecord struct {
	Request  *completionLogRequest  `json:"generateCompletionsEventRequest"`
	Response *completionLogResponse `json:"generateCompletionsEventResponse"`
}

type completionLogRequest struct {
	UserID           string  `json:"userId"`
	Timestamp        string  `json:"timeStamp"`
	FileName         string  `json:"fileName"`
	CustomizationArn *string `json:"customizationArn"`
	LeftContext      string  `json:"leftContext"`
	RightContext     string  `json:"rightContext"`
}

type completionLogResponse struct {
	RequestID   string            `json:"requestId"`
	Completions []json.RawMessage `json:"completions"`
}

func parseChatRecord(raw json.RawMessage, fileMeta *models.QDevS3FileMeta, identityClient UserDisplayNameResolver, cache *sync.Map) (*models.QDevChatLog, error) {
	var record chatLogRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, err
	}

	if record.Request == nil || record.Response == nil {
		return nil, nil
	}

	ts, err := time.Parse(time.RFC3339Nano, record.Request.Timestamp)
	if err != nil {
		ts = time.Now()
	}

	userId := normalizeUserId(record.Request.UserID)
	chatLog := &models.QDevChatLog{
		ConnectionId:     fileMeta.ConnectionId,
		ScopeId:          fileMeta.ScopeId,
		RequestId:        record.Response.RequestID,
		UserId:           userId,
		DisplayName:      cachedResolveDisplayName(userId, identityClient, cache),
		Timestamp:        ts,
		ChatTriggerType:  record.Request.ChatTriggerType,
		HasCustomization: record.Request.CustomizationArn != nil && *record.Request.CustomizationArn != "",
		ModelId:          record.Request.ModelID,
		PromptLength:     len(record.Request.Prompt),
		ResponseLength:   len(record.Response.AssistantResponse),
	}

	// Parse structured info from prompt
	prompt := record.Request.Prompt
	chatLog.OpenFileCount = countOpenFiles(prompt)
	chatLog.ActiveFileName, chatLog.ActiveFileExtension = parseActiveFile(prompt)
	chatLog.HasSteering = strings.Contains(prompt, ".kiro/steering")
	chatLog.IsSpecMode = strings.Contains(prompt, "implicit-rules")

	if record.Response.MessageMetadata.ConversationID != nil {
		chatLog.ConversationId = *record.Response.MessageMetadata.ConversationID
	}
	if record.Response.MessageMetadata.UtteranceID != nil {
		chatLog.UtteranceId = *record.Response.MessageMetadata.UtteranceID
	}

	// New fields from docs: codeReferenceEvents, supplementaryWebLinksEvent, followupPrompts
	chatLog.CodeReferenceCount = len(record.Response.CodeReferenceEvents)
	chatLog.WebLinkCount = len(record.Response.SupplementaryWebLinksEvent)
	chatLog.HasFollowupPrompts = record.Response.FollowupPrompts != ""

	return chatLog, nil
}

// countOpenFiles counts <file name="..."> tags within <OPEN-EDITOR-FILES> block
func countOpenFiles(prompt string) int {
	start := strings.Index(prompt, "<OPEN-EDITOR-FILES>")
	if start == -1 {
		return 0
	}
	end := strings.Index(prompt, "</OPEN-EDITOR-FILES>")
	if end == -1 {
		return 0
	}
	block := prompt[start:end]
	return strings.Count(block, "<file name=")
}

// parseActiveFile extracts the active file name and extension from prompt
func parseActiveFile(prompt string) (string, string) {
	start := strings.Index(prompt, "<ACTIVE-EDITOR-FILE>")
	if start == -1 {
		return "", ""
	}
	end := strings.Index(prompt[start:], "</ACTIVE-EDITOR-FILE>")
	if end == -1 {
		return "", ""
	}
	block := prompt[start : start+end]
	nameStart := strings.Index(block, "name=\"")
	if nameStart == -1 {
		return "", ""
	}
	nameStart += len("name=\"")
	nameEnd := strings.Index(block[nameStart:], "\"")
	if nameEnd == -1 {
		return "", ""
	}
	fileName := block[nameStart : nameStart+nameEnd]
	ext := filepath.Ext(fileName)
	return fileName, ext
}

func parseCompletionRecord(raw json.RawMessage, fileMeta *models.QDevS3FileMeta, identityClient UserDisplayNameResolver, cache *sync.Map) (*models.QDevCompletionLog, error) {
	var record completionLogRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, err
	}

	if record.Request == nil || record.Response == nil {
		return nil, nil
	}

	ts, err := time.Parse(time.RFC3339Nano, record.Request.Timestamp)
	if err != nil {
		ts = time.Now()
	}

	userId := normalizeUserId(record.Request.UserID)
	return &models.QDevCompletionLog{
		ConnectionId:       fileMeta.ConnectionId,
		ScopeId:            fileMeta.ScopeId,
		RequestId:          record.Response.RequestID,
		UserId:             userId,
		DisplayName:        cachedResolveDisplayName(userId, identityClient, cache),
		Timestamp:          ts,
		FileName:           record.Request.FileName,
		FileExtension:      filepath.Ext(record.Request.FileName),
		HasCustomization:   record.Request.CustomizationArn != nil && *record.Request.CustomizationArn != "",
		CompletionsCount:   len(record.Response.Completions),
		LeftContextLength:  len(record.Request.LeftContext),
		RightContextLength: len(record.Request.RightContext),
	}, nil
}

// normalizeUserId strips the "d-{directoryId}." prefix from Identity Center user IDs
// so that logging user IDs match the short UUID format used in user-report CSVs.
func normalizeUserId(userId string) string {
	if idx := strings.LastIndex(userId, "."); idx != -1 && strings.HasPrefix(userId, "d-") {
		return userId[idx+1:]
	}
	return userId
}

var ExtractQDevLoggingDataMeta = plugin.SubTaskMeta{
	Name:             "extractQDevLoggingData",
	EntryPoint:       ExtractQDevLoggingData,
	EnabledByDefault: true,
	Description:      "Extract logging data from S3 JSON.gz files (chat and completion events)",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Dependencies:     []*plugin.SubTaskMeta{&CollectQDevS3FilesMeta},
}
