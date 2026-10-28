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

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/spf13/viper"
)

// TODO move this to impl/
const defaultConfigName = ".env"

// Lowcase V for private this. You can use it by call GetConfig.
var v *viper.Viper

// GetConfig return a viper.Viper
func GetConfig() *viper.Viper {
	return v
}

func initConfig(v *viper.Viper) {
	if envFile := os.Getenv("ENV_FILE"); envFile != "" {
		v.SetConfigFile(envFile)
	} else {
		v.SetConfigName(getConfigName())
		v.SetConfigType("env")
		envPath := getEnvPath()
		v.AddConfigPath(envPath)

		paths := []string{
			"./../../../../..",
			"./../../../..",
			"./../../..",
			"./../..",
			"./..",
			"./",
		}
		for _, path := range paths {
			v.AddConfigPath(path)
		}

		for _, path := range paths {
			filePath := filepath.Join(path, getConfigName())
			fileInfo, err := os.Stat(filePath)
			if err == nil && !fileInfo.IsDir() {
				envFile = filePath
				break
			}
		}
		v.SetConfigFile(envFile)
	}

	if _, err := os.Stat(v.ConfigFileUsed()); err != nil {
		if os.IsNotExist(err) {
			logrus.Info("no [.env] file, devlake will read configuration from environment, please make sure you have set correct environment variable.")
		} else {
			panic(fmt.Errorf("failed to get config file info: %v", err))
		}
	} else {
		if err := v.ReadInConfig(); err != nil {
			panic(fmt.Errorf("failed to read configuration file: %v", err))
		}
		// This line is essential for reading
		v.WatchConfig()
	}

	v.AutomaticEnv()
	setDefaultValue(v)
}

func getConfigName() string {
	return defaultConfigName
}

// return the env path
func getEnvPath() string {
	envPath := os.Getenv("ENV_PATH")
	return filepath.Dir(envPath)
}

// Set default value for no .env or .env not set it
func setDefaultValue(v *viper.Viper) {
	v.SetDefault("PORT", "8080")
	v.SetDefault("PLUGIN_DIR", "bin/plugins")
	v.SetDefault("REMOTE_PLUGIN_DIR", "python/plugins")
	v.SetDefault("SWAGGER_DOCS_DIR", "resources/swagger")
	v.SetDefault("RESUME_PIPELINES", true)
	v.SetDefault("CORS_ALLOW_ORIGIN", "*")
	v.SetDefault("CONSUME_PIPELINES", true)
}

func init() {
	// create the object and load the .env file
	v = viper.New()
	initConfig(v)
}
