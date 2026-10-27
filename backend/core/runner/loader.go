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

package runner

import (
	"fmt"
	"io/fs"
	"path/filepath"
	goplugin "plugin"
	"strings"
	"sync"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/server/services/remote"
	"github.com/apache/devlake/server/services/remote/bridge"
	"github.com/apache/devlake/server/services/remote/models"
)

// LoadPlugins load plugins from local directory
func LoadPlugins(basicRes context.BasicRes) errors.Error {
	err := LoadGoPlugins(basicRes)
	if err != nil {
		return err
	}
	if !basicRes.GetConfigReader().GetBool("DISABLED_REMOTE_PLUGINS") {
		err = LoadRemotePlugins(basicRes)
		if err != nil {
			return err
		}
	}
	return nil
}

func LoadGoPlugins(basicRes context.BasicRes) errors.Error {
	pluginsDir := basicRes.GetConfig("PLUGIN_DIR")
	var wg sync.WaitGroup
	walkErr := filepath.WalkDir(pluginsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fileName := d.Name()
		if strings.HasSuffix(fileName, ".so") && fileName != ".so" {
			pluginName := fileName[0 : len(d.Name())-3]
			plug, loadErr := goplugin.Open(path)
			if loadErr != nil {
				return loadErr
			}
			symPluginEntry, pluginEntryError := plug.Lookup("PluginEntry")
			if pluginEntryError != nil {
				return pluginEntryError
			}
			pluginMeta, ok := symPluginEntry.(plugin.PluginMeta)
			if !ok {
				return errors.Default.New(fmt.Sprintf("%s PluginEntry must implement PluginMeta interface", pluginName))
			}
			wg.Add(1)
			go func(pluginName string, pluginMeta plugin.PluginMeta) {
				defer func() {
					wg.Done()
				}()
				err = plugin.RegisterPlugin(pluginName, pluginMeta)
				if err != nil {
					panic(err)
				}
				basicRes.GetLogger().Info(`plugin loaded %s`, pluginName)
			}(pluginName, pluginMeta)
		}
		return nil
	})
	wg.Wait()
	return errors.Convert(walkErr)
}

func LoadRemotePlugins(basicRes context.BasicRes) errors.Error {
	remotePluginDir := basicRes.GetConfig("REMOTE_PLUGIN_DIR")
	if remotePluginDir != "" {
		basicRes.GetLogger().Info("Loading remote plugins")
		remote.Init(basicRes)
		walkErr := filepath.WalkDir(remotePluginDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			fileName := d.Name()
			if fileName == "run.sh" {
				invoker := bridge.NewCmdInvoker(path)
				result := invoker.Call("plugin-info", bridge.DefaultContext)
				if result.Err != nil {
					return errors.Default.Wrap(result.Err, "Error calling plugin-info")
				}
				pluginInfo := &models.PluginInfo{}
				err := result.Get(pluginInfo)
				if err != nil {
					return err
				}
				remotePlugin, err := remote.NewRemotePlugin(pluginInfo)
				if err != nil {
					return err
				}
				err = plugin.RegisterPlugin(pluginInfo.Name, remotePlugin)
				if err != nil {
					return err
				}
				basicRes.GetLogger().Info(`remote plugin loaded %s`, pluginInfo.Name)
			}
			return nil
		})
		return errors.Convert(walkErr)
	}
	return nil
}
