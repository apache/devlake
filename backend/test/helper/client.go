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

package helper

import (
	"bytes"
	"context"
	"encoding/json"
	goerror "errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	dora "github.com/apache/incubator-devlake/plugins/dora/impl"
	org "github.com/apache/incubator-devlake/plugins/org/impl"
	refdiff "github.com/apache/incubator-devlake/plugins/refdiff/impl"

	"github.com/apache/incubator-devlake/core/config"
	corectx "github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/migration"
	"github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/core/runner"
	contextimpl "github.com/apache/incubator-devlake/impls/context"
	"github.com/apache/incubator-devlake/impls/dalgorm"
	"github.com/apache/incubator-devlake/impls/logruslog"
	"github.com/apache/incubator-devlake/server/api"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var (
	throwawayDir           string
	initService            = new(sync.Once)
	dbTruncationExclusions = []string{
		migration.MigrationHistory{}.TableName(),
		models.LockingHistory{}.TableName(),
		models.LockingStub{}.TableName(),
	}
)

func init() {
	tempDir, err := errors.Convert01(os.MkdirTemp("", "devlake_test"+"_*"))
	if err != nil {
		panic(err)
	}
	throwawayDir = tempDir
}

// DevlakeClient FIXME
type (
	DevlakeClient struct {
		Endpoint               string
		db                     *gorm.DB
		log                    log.Logger
		cfg                    *viper.Viper
		testCtx                *testing.T
		basicRes               corectx.BasicRes
		timeout                time.Duration
		pipelineTimeout        time.Duration
		expectedStatusCode     int
		lastReturnedStatusCode int
		isRemote               bool
	}
	LocalClientConfig struct {
		ServerPort      uint
		DbURL           string
		CreateServer    bool
		DropDb          bool
		TruncateDb      bool
		Plugins         []plugin.PluginMeta
		Timeout         time.Duration
		PipelineTimeout time.Duration
	}
	RemoteClientConfig struct {
		Endpoint   string
		DbURL      string
		TruncateDb bool
	}
)

// ConnectRemoteServer returns a client to an existing server based on the config
func ConnectRemoteServer(t *testing.T, clientConfig *RemoteClientConfig) *DevlakeClient {
	var db *gorm.DB
	var err errors.Error
	logger := logruslog.Global.Nested("test")
	cfg := config.GetConfig()
	if clientConfig.DbURL != "" {
		cfg.Set("DB_URL", clientConfig.DbURL)
		db, err = runner.NewGormDb(cfg, logger)
		require.NoError(t, err)
	}
	logger.Info("Connecting to remote server: %s", clientConfig.Endpoint)
	client := &DevlakeClient{
		isRemote: true,
		Endpoint: clientConfig.Endpoint,
		db:       db,
		cfg:      cfg,
		log:      logger,
		testCtx:  t,
		basicRes: contextimpl.NewDefaultBasicRes(cfg, logger, dalgorm.NewDalgorm(db)),
	}
	client.prepareDB(&LocalClientConfig{
		TruncateDb: clientConfig.TruncateDb,
	})
	return client
}

// ConnectLocalServer spins up a local server from the config and returns a client connected to it
func ConnectLocalServer(t *testing.T, clientConfig *LocalClientConfig) *DevlakeClient {
	t.Helper()
	fmt.Printf("Using test temp directory: %s\n", throwawayDir)
	logger := logruslog.Global.Nested("test")
	cfg := config.GetConfig()
	// E2E helpers issue direct API requests without session or API-key auth.
	// Keep local test servers aligned with that contract.
	t.Setenv("AUTH_ENABLED", "false")
	cfg.Set("AUTH_ENABLED", false)
	cfg.Set("DB_URL", clientConfig.DbURL)
	db, err := runner.NewGormDb(cfg, logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		d, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, d.Close())
	})
	addr := fmt.Sprintf("http://localhost:%d", clientConfig.ServerPort)
	d := &DevlakeClient{
		Endpoint:        addr,
		db:              db,
		log:             logger,
		cfg:             cfg,
		basicRes:        contextimpl.NewDefaultBasicRes(cfg, logger, dalgorm.NewDalgorm(db)),
		testCtx:         t,
		timeout:         clientConfig.Timeout,
		pipelineTimeout: clientConfig.PipelineTimeout,
	}
	if d.timeout == 0 {
		d.timeout = 10 * time.Second
	}
	if d.pipelineTimeout == 0 {
		d.pipelineTimeout = 30 * time.Second
	}
	if clientConfig.CreateServer {
		d.configureEncryption()
		d.initPlugins(clientConfig)
		if clientConfig.DropDb || clientConfig.TruncateDb {
			d.prepareDB(clientConfig)
		}
		cfg.Set("PORT", clientConfig.ServerPort)
		cfg.Set("PLUGIN_DIR", throwawayDir)
		cfg.Set("LOGGING_DIR", throwawayDir)
		go func() {
			initService.Do(func() {
				api.CreateAndRunApiServer()
			})
		}()
		// NOTE: /proceed-db-migration now requires authentication. If AUTH_ENABLED=true, this test must provide credentials.
		req, err2 := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/proceed-db-migration", addr), nil)
		require.NoError(t, err2)
		d.forceSendHttpRequest(100, req, func(err errors.Error) bool {
			e := err.Unwrap()
			return goerror.Is(e, syscall.ECONNREFUSED)
		})
		logger.Info("New DevLake server initialized")
	}
	return d
}

// SetTimeout override the timeout of api requests
func (d *DevlakeClient) SetTimeout(timeout time.Duration) {
	d.timeout = timeout
}

// SetTimeout override the timeout of pipeline run success expectation
func (d *DevlakeClient) SetPipelineTimeout(timeout time.Duration) {
	d.pipelineTimeout = timeout
}

// SetExpectedStatusCode override the expected status code of the next API call. If it's anything but this, the test will fail.
func (d *DevlakeClient) SetExpectedStatusCode(code int) *DevlakeClient {
	d.expectedStatusCode = code
	return d
}

// SetExpectedStatusCode return the last http status code
func (d *DevlakeClient) LastReturnedStatusCode() int {
	return d.lastReturnedStatusCode
}

// GetDal get a reference to the dal.Dal used by the server
func (d *DevlakeClient) GetDal() dal.Dal {
	return dalgorm.NewDalgorm(d.db)
}

// AwaitPluginAvailability wait for this plugin to become available on the server given a timeout. Returns false if this condition does not get met.
func (d *DevlakeClient) AwaitPluginAvailability(pluginName string, timeout time.Duration) {
	if d.isRemote {
		return
	}
	err := runWithTimeout(timeout, func() (bool, errors.Error) {
		_, err := plugin.GetPlugin(pluginName)
		return err == nil, nil
	})
	require.NoError(d.testCtx, err)
}

// RunPlugin manually execute a plugin directly (local server only)
func (d *DevlakeClient) RunPlugin(ctx context.Context, pluginName string, pluginTask plugin.PluginTask, options map[string]interface{}, subtaskNames ...string) errors.Error {
	if len(subtaskNames) == 0 {
		subtaskNames = GetSubtaskNames(pluginTask.SubTaskMetas()...)
	}
	task := &models.Task{
		Plugin:   pluginName,
		Options:  options,
		Subtasks: subtaskNames,
	}
	return runner.RunPluginSubTasks(
		ctx,
		d.basicRes,
		task,
		pluginTask,
		nil,
		nil,
	)
}

func (d *DevlakeClient) configureEncryption() {
	v := config.GetConfig()
	encryptionSecret := v.GetString(plugin.EncodeKeyEnvStr)
	// only test environment should have this set
	if encryptionSecret == "" {
		// default value
		v.Set(plugin.EncodeKeyEnvStr, "DFLFZLMBBFDDCYWRECDCIYUROPPAKQDFQMMJEFPIKVFVHZBRGAZIHKRJIJZMOHWEVRSCETAGGONPSULGOXITVXISVCQGPSFAOGRDLUANEYDQFBDKVMYYHUZFHYVYGPPT")
	}
}

func (d *DevlakeClient) forceSendHttpRequest(retries uint, req *http.Request, onError func(err errors.Error) bool) {
	d.testCtx.Helper()
	for {
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			if !onError(errors.Default.WrapRaw(err)) {
				require.NoError(d.testCtx, err)
			}
		} else {
			if res.StatusCode != http.StatusOK {
				panic(fmt.Sprintf("received HTTP status %d", res.StatusCode))
			}
			return
		}
		retries--
		if retries == 0 {
			panic("retry limit exceeded")
		}
		fmt.Printf("retrying http call to %s\n", req.URL.String())
		time.Sleep(1 * time.Second)
	}
}

func (d *DevlakeClient) initPlugins(cfg *LocalClientConfig) {
	d.testCtx.Helper()
	// default plugins
	cfg.Plugins = append(cfg.Plugins, []plugin.PluginMeta{
		org.Org{},
		dora.Dora{},
		refdiff.RefDiff{},
	}...)
	// register and init plugins
	for _, p := range cfg.Plugins {
		require.NoError(d.testCtx, plugin.RegisterPlugin(p.Name(), p))
	}
}

func (d *DevlakeClient) prepareDB(cfg *LocalClientConfig) {
	d.testCtx.Helper()
	migrator := d.db.Migrator()
	tables, err := migrator.GetTables()
	require.NoError(d.testCtx, err)
	d.log.Debug("Existing DB tables: %v", tables)
	if cfg.DropDb {
		d.log.Info("Dropping %d tables", len(tables))
		var tablesRaw []any
		for _, table := range tables {
			tablesRaw = append(tablesRaw, table)
		}
		err = migrator.DropTable(tablesRaw...)
		require.NoError(d.testCtx, err)
	} else if cfg.TruncateDb {
		if len(tables) > len(dbTruncationExclusions) {
			d.log.Info("Truncating %d tables", len(tables)-len(dbTruncationExclusions))
		}
		for _, table := range tables {
			excluded := false
			for _, exclusion := range dbTruncationExclusions {
				if exclusion == table {
					excluded = true
					break
				}
			}
			if !excluded {
				err = d.db.Exec("DELETE FROM " + table).Error
				require.NoError(d.testCtx, err)
			}
		}
	}
}

func runWithTimeout(timeout time.Duration, f func() (bool, errors.Error)) errors.Error {
	if timeout == 0 {
		timeout = math.MaxInt
	}
	type response struct {
		err       errors.Error
		completed bool
	}
	timer := time.After(timeout)
	resChan := make(chan response)
	resp := response{}
	for {
		go func() {
			done, err := f()
			resChan <- response{err, done}
		}()
		select {
		case <-timer:
			if !resp.completed {
				return errors.Default.New(fmt.Sprintf("timed out calling function after %d milliseconds", timeout.Milliseconds()))
			}
			return nil
		case resp = <-resChan:
			if resp.err != nil {
				return resp.err
			}
			if resp.completed {
				return nil
			}
			time.Sleep(1 * time.Second)
			continue
		}
	}
}

func sendHttpRequest[Res any](t *testing.T, timeout time.Duration, ctx *testContext, httpMethod string, endpoint string, headers map[string]string, body any) Res {
	t.Helper()
	defer func() {
		ctx.client.expectedStatusCode = 0
	}()
	b := ToJson(body)
	if ctx.printPayload {
		coloredPrintf("calling:\n\t%s %s\nwith:\n%s\n", httpMethod, endpoint, string(ToCleanJson(ctx.inlineJson, body)))
	}
	var result Res
	err := runWithTimeout(timeout, func() (bool, errors.Error) {
		request, err := http.NewRequest(httpMethod, endpoint, bytes.NewReader(b))
		if err != nil {
			return false, errors.Convert(err)
		}
		request.Close = true
		request.Header.Add("Content-Type", "application/json")
		for header, headerVal := range headers {
			request.Header.Add(header, headerVal)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return false, errors.Convert(err)
		}
		defer func() {
			ctx.client.lastReturnedStatusCode = response.StatusCode
		}()
		if ctx.client.expectedStatusCode > 0 || response.StatusCode >= 300 {
			if ctx.client.expectedStatusCode == 0 || ctx.client.expectedStatusCode != response.StatusCode {
				if err = response.Body.Close(); err != nil {
					return false, errors.Convert(err)
				}
				response.Close = true
				return false, errors.HttpStatus(response.StatusCode).New(fmt.Sprintf("unexpected http status code calling [%s] %s: %d", httpMethod, endpoint, response.StatusCode))
			}
		}
		b, _ = io.ReadAll(response.Body)
		if err = json.Unmarshal(b, &result); err != nil {
			if response.StatusCode < 300 {
				return false, errors.Convert(err)
			}
			// it's probably ok since the request failed anyway
		}
		if ctx.printPayload {
			coloredPrintf("result: %s\n", ToCleanJson(ctx.inlineJson, b))
		}
		if err = response.Body.Close(); err != nil {
			return false, errors.Convert(err)
		}
		response.Close = true
		return true, nil
	})
	require.NoError(t, err)
	return result
}

func coloredPrintf(msg string, args ...any) {
	msg = fmt.Sprintf(msg, args...)
	colorifier := "\033[1;33m%+v\033[0m" //yellow
	fmt.Printf(colorifier, msg)
}

type testContext struct {
	printPayload bool
	inlineJson   bool
	client       *DevlakeClient
}
