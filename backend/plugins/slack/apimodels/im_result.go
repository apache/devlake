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

package apimodels

import "encoding/json"

type SlackChannelApiResult struct {
	Ok               bool              `json:"ok"`
	Error            string            `json:"error"`
	Channels         []json.RawMessage `json:"channels"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

type SlackChannelMessageApiResult struct {
	Ok               bool              `json:"ok"`
	Error            string            `json:"error"`
	Messages         []json.RawMessage `json:"messages"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

type SlackChannelMessageResultItem struct {
	ClientMsgId string `json:"client_msg_id"`
	Type        string `json:"type"`
	Subtype     string `json:"subtype"`
	Ts          string `json:"ts"`
	ThreadTs    string `json:"thread_ts"`
	User        string `json:"user"`
	Text        string `json:"text"`

	Team            string   `json:"team"`
	ReplyCount      int      `json:"reply_count"`
	ReplyUsersCount int      `json:"reply_users_count"`
	LatestReply     string   `json:"latest_reply"`
	ReplyUsers      []string `json:"reply_users"`
	IsLocked        bool     `json:"is_locked"`
	Subscribed      bool     `json:"subscribed"`
	ParentUserId    string   `json:"parent_user_id"`

	Files []struct {
		Id                 string `json:"id"`
		Created            int    `json:"created"`
		Timestamp          int    `json:"timestamp"`
		Name               string `json:"name"`
		Title              string `json:"title"`
		Mimetype           string `json:"mimetype"`
		Filetype           string `json:"filetype"`
		PrettyType         string `json:"pretty_type"`
		User               string `json:"user"`
		UserTeam           string `json:"user_team"`
		Editable           bool   `json:"editable"`
		Size               int    `json:"size"`
		Mode               string `json:"mode"`
		IsExternal         bool   `json:"is_external"`
		ExternalType       string `json:"external_type"`
		IsPublic           bool   `json:"is_public"`
		PublicUrlShared    bool   `json:"public_url_shared"`
		DisplayAsBot       bool   `json:"display_as_bot"`
		Username           string `json:"username"`
		UrlPrivate         string `json:"url_private"`
		UrlPrivateDownload string `json:"url_private_download"`
		MediaDisplayType   string `json:"media_display_type"`
		Thumb64            string `json:"thumb_64"`
		Thumb80            string `json:"thumb_80"`
		Thumb360           string `json:"thumb_360"`
		Thumb360W          int    `json:"thumb_360_w"`
		Thumb360H          int    `json:"thumb_360_h"`
		Thumb480           string `json:"thumb_480"`
		Thumb480W          int    `json:"thumb_480_w"`
		Thumb480H          int    `json:"thumb_480_h"`
		Thumb160           string `json:"thumb_160"`
		Thumb720           string `json:"thumb_720"`
		Thumb720W          int    `json:"thumb_720_w"`
		Thumb720H          int    `json:"thumb_720_h"`
		Thumb800           string `json:"thumb_800"`
		Thumb800W          int    `json:"thumb_800_w"`
		Thumb800H          int    `json:"thumb_800_h"`
		Thumb960           string `json:"thumb_960"`
		Thumb960W          int    `json:"thumb_960_w"`
		Thumb960H          int    `json:"thumb_960_h"`
		Thumb1024          string `json:"thumb_1024"`
		Thumb1024W         int    `json:"thumb_1024_w"`
		Thumb1024H         int    `json:"thumb_1024_h"`
		OriginalW          int    `json:"original_w"`
		OriginalH          int    `json:"original_h"`
		ThumbTiny          string `json:"thumb_tiny"`
		Permalink          string `json:"permalink"`
		PermalinkPublic    string `json:"permalink_public"`
		IsStarred          bool   `json:"is_starred"`
		HasRichPreview     bool   `json:"has_rich_preview"`
		FileAccess         string `json:"file_access"`
	} `json:"files"`
	Upload bool `json:"upload"`
	Blocks []struct {
		Type     string `json:"type"`
		BlockId  string `json:"block_id"`
		Elements []struct {
			Type     string `json:"type"`
			Elements []struct {
				Type  string `json:"type"`
				Text  string `json:"text"`
				Style struct {
					Bold bool `json:"bold"`
				} `json:"style,omitempty"`
			} `json:"elements"`
		} `json:"elements"`
	} `json:"blocks"`

	Root struct {
		ClientMsgId string `json:"client_msg_id"`
		Type        string `json:"type"`
		Text        string `json:"text"`
		User        string `json:"user"`
		Ts          string `json:"ts"`
		Blocks      []struct {
			Type     string `json:"type"`
			BlockId  string `json:"block_id"`
			Elements []struct {
				Type     string `json:"type"`
				Elements []struct {
					Type    string `json:"type"`
					Text    string `json:"text,omitempty"`
					Name    string `json:"name,omitempty"`
					Unicode string `json:"unicode,omitempty"`
				} `json:"elements"`
			} `json:"elements"`
		} `json:"blocks"`
		Team            string   `json:"team"`
		ThreadTs        string   `json:"thread_ts"`
		ReplyCount      int      `json:"reply_count"`
		ReplyUsersCount int      `json:"reply_users_count"`
		LatestReply     string   `json:"latest_reply"`
		ReplyUsers      []string `json:"reply_users"`
		IsLocked        bool     `json:"is_locked"`
		Subscribed      bool     `json:"subscribed"`
	} `json:"root"`
	Reactions []struct {
		Name  string   `json:"name"`
		Users []string `json:"users"`
		Count int      `json:"count"`
	} `json:"reactions"`
}

type SlackThreadsApiResult struct {
	Ok               bool              `json:"ok"`
	Error            string            `json:"error"`
	Threads          []json.RawMessage `json:"messages"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}
