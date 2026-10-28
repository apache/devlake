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
	"testing"

	"github.com/apache/devlake/plugins/aireview/models"
	"github.com/stretchr/testify/assert"
)

func TestDecodeTaskOptions(t *testing.T) {
	t.Run("valid options with repoId", func(t *testing.T) {
		opts := map[string]any{
			"repoId": "github:GithubRepo:1:100",
		}
		result, err := DecodeTaskOptions(opts)
		assert.Nil(t, err)
		assert.Equal(t, "github:GithubRepo:1:100", result.RepoId)
	})

	t.Run("valid options with projectName", func(t *testing.T) {
		opts := map[string]any{
			"projectName": "my-project",
		}
		result, err := DecodeTaskOptions(opts)
		assert.Nil(t, err)
		assert.Equal(t, "my-project", result.ProjectName)
	})

	t.Run("valid options with both fields", func(t *testing.T) {
		opts := map[string]any{
			"repoId":      "github:GithubRepo:1:100",
			"projectName": "my-project",
		}
		result, err := DecodeTaskOptions(opts)
		assert.Nil(t, err)
		assert.Equal(t, "github:GithubRepo:1:100", result.RepoId)
		assert.Equal(t, "my-project", result.ProjectName)
	})

	t.Run("empty map produces empty options", func(t *testing.T) {
		opts := map[string]any{}
		result, err := DecodeTaskOptions(opts)
		assert.Nil(t, err)
		assert.Empty(t, result.RepoId)
		assert.Empty(t, result.ProjectName)
	})

	t.Run("optional fields decoded", func(t *testing.T) {
		opts := map[string]any{
			"repoId":         "repo1",
			"sourcePlatform": "github",
			"timeAfter":      "2024-01-01",
		}
		result, err := DecodeTaskOptions(opts)
		assert.Nil(t, err)
		assert.Equal(t, "github", result.SourcePlatform)
		assert.Equal(t, "2024-01-01", result.TimeAfter)
	})
}

func TestValidateTaskOptions(t *testing.T) {
	t.Run("repoId only is valid", func(t *testing.T) {
		err := ValidateTaskOptions(&AiReviewOptions{RepoId: "repo1"})
		assert.Nil(t, err)
	})

	t.Run("projectName only is valid", func(t *testing.T) {
		err := ValidateTaskOptions(&AiReviewOptions{ProjectName: "proj1"})
		assert.Nil(t, err)
	})

	t.Run("both set is valid", func(t *testing.T) {
		err := ValidateTaskOptions(&AiReviewOptions{RepoId: "repo1", ProjectName: "proj1"})
		assert.Nil(t, err)
	})

	t.Run("neither set returns error", func(t *testing.T) {
		err := ValidateTaskOptions(&AiReviewOptions{})
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "repoId")
	})
}

func TestCompilePatterns_AllToolsEnabled(t *testing.T) {
	config := models.GetDefaultScopeConfig()
	taskData := &AiReviewTaskData{
		Options: &AiReviewOptions{
			ScopeConfig: config,
		},
	}
	err := CompilePatterns(taskData)
	assert.Nil(t, err)

	assert.NotNil(t, taskData.CodeRabbitUsernameRegex)
	assert.NotNil(t, taskData.CodeRabbitPatternRegex)
	assert.NotNil(t, taskData.QodoUsernameRegex)
	assert.NotNil(t, taskData.QodoPatternRegex)
	assert.NotNil(t, taskData.GeminiUsernameRegex)
	assert.NotNil(t, taskData.GeminiPatternRegex)
	assert.NotNil(t, taskData.RiskHighPatternRegex)
	assert.NotNil(t, taskData.RiskMediumPatternRegex)
	assert.NotNil(t, taskData.RiskLowPatternRegex)
	assert.NotNil(t, taskData.AiCommitPatternsRegex)
}

func TestCompilePatterns_NilConfigUsesDefaults(t *testing.T) {
	taskData := &AiReviewTaskData{
		Options: &AiReviewOptions{
			ScopeConfig: nil,
		},
	}
	err := CompilePatterns(taskData)
	assert.Nil(t, err)
	assert.NotNil(t, taskData.Options.ScopeConfig, "nil config should be replaced with defaults")
	assert.NotNil(t, taskData.CodeRabbitPatternRegex)
}

func TestCompilePatterns_InvalidRegexReturnsError(t *testing.T) {
	config := models.GetDefaultScopeConfig()
	config.CodeRabbitPattern = "[invalid"
	taskData := &AiReviewTaskData{
		Options: &AiReviewOptions{
			ScopeConfig: config,
		},
	}
	err := CompilePatterns(taskData)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "codeRabbitPattern")
}

func TestCompilePatterns_DisabledToolSkipsCompilation(t *testing.T) {
	config := models.GetDefaultScopeConfig()
	config.QodoEnabled = false
	taskData := &AiReviewTaskData{
		Options: &AiReviewOptions{
			ScopeConfig: config,
		},
	}
	err := CompilePatterns(taskData)
	assert.Nil(t, err)
	assert.Nil(t, taskData.QodoUsernameRegex, "disabled tool should not compile username regex")
	assert.Nil(t, taskData.QodoPatternRegex, "disabled tool should not compile pattern regex")
}

func TestCompilePatterns_RiskPatterns(t *testing.T) {
	config := models.GetDefaultScopeConfig()
	taskData := &AiReviewTaskData{
		Options: &AiReviewOptions{
			ScopeConfig: config,
		},
	}
	err := CompilePatterns(taskData)
	assert.Nil(t, err)

	assert.True(t, taskData.RiskHighPatternRegex.MatchString("security vulnerability found"))
	assert.True(t, taskData.RiskMediumPatternRegex.MatchString("warning: possible issue"))
	assert.True(t, taskData.RiskLowPatternRegex.MatchString("minor style fix"))
}

func TestCompilePatterns_CursorBugbotEnabled(t *testing.T) {
	config := models.GetDefaultScopeConfig()
	config.CursorBugbotEnabled = true
	config.CursorBugbotUsername = "cursor-bugbot"
	config.CursorBugbotPattern = `(?i)(cursor|bugbot)`
	taskData := &AiReviewTaskData{
		Options: &AiReviewOptions{
			ScopeConfig: config,
		},
	}
	err := CompilePatterns(taskData)
	assert.Nil(t, err)
	assert.NotNil(t, taskData.CursorBugbotUsernameRegex)
	assert.NotNil(t, taskData.CursorBugbotPatternRegex)
	assert.True(t, taskData.CursorBugbotPatternRegex.MatchString("cursor review"))
}

func TestCompilePatterns_InvalidQodoPattern(t *testing.T) {
	config := models.GetDefaultScopeConfig()
	config.QodoPattern = "[invalid"
	taskData := &AiReviewTaskData{
		Options: &AiReviewOptions{
			ScopeConfig: config,
		},
	}
	err := CompilePatterns(taskData)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "qodoPattern")
}

func TestCompilePatterns_InvalidGeminiPattern(t *testing.T) {
	config := models.GetDefaultScopeConfig()
	config.GeminiPattern = "[invalid"
	taskData := &AiReviewTaskData{
		Options: &AiReviewOptions{
			ScopeConfig: config,
		},
	}
	err := CompilePatterns(taskData)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "geminiPattern")
}

func TestCompilePatterns_InvalidRiskMediumPattern(t *testing.T) {
	config := models.GetDefaultScopeConfig()
	config.RiskMediumPattern = "[invalid"
	taskData := &AiReviewTaskData{
		Options: &AiReviewOptions{
			ScopeConfig: config,
		},
	}
	err := CompilePatterns(taskData)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "riskMediumPattern")
}

func TestCompilePatterns_InvalidBugLinkPattern(t *testing.T) {
	config := models.GetDefaultScopeConfig()
	config.BugLinkPattern = "[invalid"
	taskData := &AiReviewTaskData{
		Options: &AiReviewOptions{
			ScopeConfig: config,
		},
	}
	err := CompilePatterns(taskData)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "bugLinkPattern")
}

func TestCompilePatterns_InvalidAiPrLabelPattern(t *testing.T) {
	config := models.GetDefaultScopeConfig()
	config.AiPrLabelPattern = "[invalid"
	taskData := &AiReviewTaskData{
		Options: &AiReviewOptions{
			ScopeConfig: config,
		},
	}
	err := CompilePatterns(taskData)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "aiPrLabelPattern")
}

func TestCompilePatterns_AiCommitPatterns(t *testing.T) {
	config := models.GetDefaultScopeConfig()
	taskData := &AiReviewTaskData{
		Options: &AiReviewOptions{
			ScopeConfig: config,
		},
	}
	err := CompilePatterns(taskData)
	assert.Nil(t, err)
	assert.NotNil(t, taskData.AiCommitPatternsRegex)
	assert.True(t, taskData.AiCommitPatternsRegex.MatchString("Co-Authored-By: ChatGPT <noreply@openai.com>"))
}

func TestCompilePatterns_InvalidAiCommitPatterns(t *testing.T) {
	config := models.GetDefaultScopeConfig()
	config.AiCommitPatterns = "[invalid"
	taskData := &AiReviewTaskData{
		Options: &AiReviewOptions{
			ScopeConfig: config,
		},
	}
	err := CompilePatterns(taskData)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "aiCommitPatterns")
}

func TestCompilePatterns_EmptyAiCommitPatternsSkips(t *testing.T) {
	config := models.GetDefaultScopeConfig()
	config.AiCommitPatterns = ""
	taskData := &AiReviewTaskData{
		Options: &AiReviewOptions{
			ScopeConfig: config,
		},
	}
	err := CompilePatterns(taskData)
	assert.Nil(t, err)
	assert.Nil(t, taskData.AiCommitPatternsRegex)
}
