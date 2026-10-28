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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/swag"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/impls/logruslog"
	"github.com/apache/devlake/server/api/auth"
	_ "github.com/apache/devlake/server/api/docs"
	"github.com/apache/devlake/server/api/ping"
	"github.com/apache/devlake/server/api/shared"
	"github.com/apache/devlake/server/api/version"
	"github.com/apache/devlake/server/services"
)

const DB_MIGRATION_REQUIRED = `
New migration scripts detected. Database migration is required to launch DevLake.
WARNING: Performing migration may wipe collected data for consistency and re-collecting data may be required.
To proceed, please send a request to <config-ui-endpoint>/api/proceed-db-migration (or <devlake-endpoint>/proceed-db-migration).
Alternatively, you may downgrade back to the previous DevLake version.
`
const DB_MIGRATING = `Database migration is in progress. Please wait until it is completed.`

var basicRes context.BasicRes

// parseCORSAllowOrigins splits comma-separated origins so env vars like
// "https://a.com,https://b.com" (often a single slice element from Viper) become multiple origins.
func parseCORSAllowOrigins(origins []string) []string {
	if len(origins) == 0 {
		return nil
	}
	out := make([]string, 0, len(origins))
	for _, s := range origins {
		for _, part := range strings.Split(s, ",") {
			if o := strings.TrimSpace(part); o != "" {
				out = append(out, o)
			}
		}
	}
	return out
}

func Init() {
	services.Init()
	basicRes = services.GetBasicRes()
	auth.Init(basicRes)
}

func InjectCustomService(pipelineNotifier services.PipelineNotificationService, projectService services.ProjectService) errors.Error {
	return services.InjectCustomService(pipelineNotifier, projectService)
}

// @title  DevLake Swagger API
// @version 0.1
// @description  <h2>This is the main page of devlake api</h2>
// @license.name Apache-2.0
// @host localhost:8080
// @BasePath /
func CreateAndRunApiServer() {
	// Setup and run the server
	Init()
	services.InitExecuteMigration()
	router := CreateApiServer()
	SetupApiServer(router)
	RunApiServer(router)
}

func CreateApiServer() *gin.Engine {
	// Create router
	router := gin.Default()

	// Enable CORS (split comma-separated origins: env vars often come as one string)
	cfg := basicRes.GetConfigReader()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     parseCORSAllowOrigins(cfg.GetStringSlice("CORS_ALLOW_ORIGIN")),
		AllowMethods:     []string{"PUT", "PATCH", "POST", "GET", "OPTIONS", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           2 * time.Hour,
	}))

	// For both protected and unprotected routes
	router.GET("/ping", ping.Get)
	router.GET("/ready", ping.Ready)
	router.GET("/health", ping.Health)
	router.GET("/version", version.Get)

	// Auth chain order matters: REST API key first (its own short-circuit),
	// then the push API key gate, then OIDC session, then oauth2-proxy header
	// (only sets USER if not yet set and the forwarded secret matches), then the terminal 401 gate,
	// finally CSRF on unsafe methods.
	router.Use(RestAuthentication(router, basicRes))
	router.Use(RequirePushAuthentication(basicRes))
	router.Use(auth.OIDCAuthentication())
	router.Use(OAuth2ProxyAuthentication(basicRes))
	router.Use(auth.RequireAuth())
	router.Use(auth.CSRFProtect())

	return router
}

func SetupApiServer(router *gin.Engine) {
	// Set gin mode
	gin.SetMode(basicRes.GetConfig("MODE"))
	// Required for `/projects/hello%20%2F%20world` to be parsed properly with `/projects/:projectName`
	// end up with `name = "hello / world"`
	router.UseRawPath = true
	// router.UnescapePathValues = false

	// Endpoint to proceed database migration — listed in auth.publicPaths because
	// auth tables may not exist yet when migration is pending.
	router.GET("/proceed-db-migration", func(ctx *gin.Context) {
		// Surface the real migration error to the client instead of a bare 500.
		if err := services.ExecuteMigration(); err != nil {
			shared.ApiOutputError(ctx, err)
			return
		}
		shared.ApiOutputSuccess(ctx, nil, http.StatusOK)
	})

	// Restrict access if database migration is required
	router.Use(func(ctx *gin.Context) {
		serviceStatus := services.CurrentStatus()
		if serviceStatus == services.SERVICE_STATUS_WAIT_CONFIRM {
			// Return error response
			shared.ApiOutputError(
				ctx,
				errors.HttpStatus(http.StatusPreconditionRequired).New(DB_MIGRATION_REQUIRED),
			)
			ctx.Abort()
		} else if serviceStatus == services.SERVICE_STATUS_MIGRATING {
			// Return error response
			shared.ApiOutputError(
				ctx,
				errors.HttpStatus(http.StatusPreconditionRequired).New(DB_MIGRATING),
			)
			ctx.Abort()
		}
	})

	// Add swagger handlers
	router.GET("/swagger/*any", modifyBasePath, ginSwagger.WrapHandler(swaggerFiles.Handler))
	registerExtraOpenApiSpecs(router)

	// Add debug logging for endpoints
	gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
		logruslog.Global.Printf("endpoint %v %v %v %v", httpMethod, absolutePath, handlerName, nuHandlers)
	}
	// Register API endpoints
	RegisterRouter(router, basicRes)
}

func RunApiServer(router *gin.Engine) {
	// Get port from config
	port := basicRes.GetConfig("PORT")
	// Trim any : from the start
	port = strings.TrimLeft(port, ":")
	// Convert to int
	portNum, err := strconv.Atoi(port)
	if err != nil {
		// Panic if PORT is not an int
		panic(fmt.Errorf("PORT [%s] must be int: %s", port, err.Error()))
	}

	// Start the server
	err = router.Run(fmt.Sprintf(":%d", portNum))
	if err != nil {
		panic(err)
	}
}

func registerExtraOpenApiSpecs(router *gin.Engine) {
	for name, pluginMeta := range plugin.AllPlugins() {
		if pluginOpenApiSpec, ok := pluginMeta.(plugin.PluginOpenApiSpec); ok {
			spec := &swag.Spec{
				InfoInstanceName: name,
				SwaggerTemplate:  pluginOpenApiSpec.OpenApiSpec(),
			}
			swag.Register(name, spec)
			router.GET(
				fmt.Sprintf("/plugins/swagger/%s/*any", name),
				ginSwagger.CustomWrapHandler(
					&ginSwagger.Config{
						URL:                      "doc.json",
						DocExpansion:             "list",
						InstanceName:             name,
						Title:                    fmt.Sprintf("%s API", name),
						DefaultModelsExpandDepth: 1,
						DeepLinking:              true,
						PersistAuthorization:     false,
					},
					swaggerFiles.Handler,
				),
			)
		}
	}
}

type bodyTamper struct {
	gin.ResponseWriter
}

// Write intercepts the response body and modifies the base path
func (w bodyTamper) Write(b []byte) (int, error) {
	var m map[string]interface{}
	err := json.Unmarshal(b, &m)
	if err != nil {
		return 0, err
	}
	m["basePath"] = "/api"
	b, err = json.Marshal(m)
	if err != nil {
		return 0, err
	}
	return w.ResponseWriter.Write(b)
}

func modifyBasePath(c *gin.Context) {
	if !strings.HasSuffix(c.Request.URL.Path, "swagger/doc.json") {
		return
	}
	u, _ := url.Parse(c.GetHeader("Referer"))
	if u == nil || !strings.HasPrefix(u.Path, "/api") {
		return
	}
	blw := &bodyTamper{ResponseWriter: c.Writer}
	c.Writer = blw
}
