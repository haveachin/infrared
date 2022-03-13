package cmd

import (
	"bytes"
	"embed"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock"
	"github.com/haveachin/infrared/internal/pkg/java"
	"github.com/haveachin/infrared/internal/plugin/webhook"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	v     *viper.Viper
	files embed.FS

	configPath string
	workingDir string

	rootCmd = &cobra.Command{
		Use:   "infrared",
		Short: "Starts the infrared proxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger, err := zap.NewDevelopment()
			if err != nil {
				return fmt.Errorf("failed to init logger; err: %s", err)
			}

			if err := os.Chdir(workingDir); err != nil {
				return err
			}

			logger.Info("loading proxy from config",
				zap.String("config", configPath),
			)

			if err := createIconsIfNotExist(); err != nil {
				return err
			}

			if err := loadConfig(); err != nil {
				return err
			}

			if err := loadConfigsFromProvieders(); err != nil {
				return err
			}

			bedrockProxy, err := infrared.NewProxy(&bedrock.ProxyConfig{Viper: v})
			if err != nil {
				return err
			}

			javaProxy, err := infrared.NewProxy(&java.ProxyConfig{Viper: v})
			if err != nil {
				return err
			}

			pluginManager := infrared.PluginManager{
				Plugins: []infrared.Plugin{
					&webhook.Plugin{
						Viper: v,
					},
				},
				Log: logger,
			}

			if err := pluginManager.EnablePlugins(); err != nil {
				return err
			}

			logger.Info("starting proxy")

			go bedrockProxy.ListenAndServe(logger)
			go javaProxy.ListenAndServe(logger)

			sc := make(chan os.Signal, 1)
			signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
			<-sc

			logger.Info("disabeling plugins")
			return pluginManager.DisablePlugins()
		},
	}
)

func init() {
	v = viper.New()
	v.SetEnvPrefix("INFRARED")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	rootCmd.Flags().StringVarP(&configPath, "config", "c", "config.yml", "path of the config file")
	rootCmd.Flags().StringVarP(&workingDir, "working-dir", "w", ".", "set the working directory")
	viper.BindPFlag("CONFIG", rootCmd.Flags().Lookup("config"))

	rootCmd.AddCommand(licenseCmd)
}

// Execute executes the root command.
func Execute(fs embed.FS) error {
	files = fs
	return rootCmd.Execute()
}

func createIconsIfNotExist() error {
	if _, err := os.Stat("icons/default.png"); err == nil || !os.IsNotExist(err) {
		return nil
	}

	if err := os.Mkdir("icons", 0755); err != nil {
		return err
	}

	bb, err := files.ReadFile("configs/icons/default.png")
	if err != nil {
		return err
	}

	return ioutil.WriteFile("icons/default.png", bb, 0666)
}

func loadConfig() error {
	configPath = strings.TrimSpace(configPath)
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok || os.IsNotExist(err) {
			return writeDefaultConfigs()
		}
		return err
	}
	return nil
}

func writeDefaultConfigs() error {
	if err := writeConfigFromEmbedFS("configs/default.yml", configPath); err != nil {
		return err
	}
	return writeDirFromEmbedFS("configs/proxies", "proxies")
}

func writeDirFromEmbedFS(embedPath, cfgPath string) error {
	entries, err := files.ReadDir(embedPath)
	if err != nil {
		return err
	}

	dir := path.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	for _, e := range entries {
		name := e.Name()
		name = strings.TrimSuffix(name, path.Ext(name))
		name += path.Ext(configPath)
		ePath := fmt.Sprintf("%s/%s", embedPath, e.Name())
		cPath := fmt.Sprintf("%s/%s", cfgPath, name)
		if e.IsDir() {
			if err := writeDirFromEmbedFS(ePath, cPath); err != nil {
				return err
			}
			continue
		}

		if err := writeConfigFromEmbedFS(ePath, cPath); err != nil {
			return err
		}
	}
	return nil
}

func writeConfigFromEmbedFS(embedPath, cfgPath string) error {
	bb, err := readEmbedFile(embedPath)
	if err != nil {
		return err
	}

	v.SetConfigType("yml")
	if err := v.ReadConfig(bytes.NewReader(bb)); err != nil {
		return err
	}
	return writeConfig(v, cfgPath, bb)
}

func readEmbedFile(path string) ([]byte, error) {
	f, err := files.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ioutil.ReadAll(f)
}

func writeConfig(v *viper.Viper, cfgPath string, value []byte) error {
	dir := path.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	switch ext := path.Ext(cfgPath); ext {
	case ".yml", ".yaml":
		return ioutil.WriteFile(cfgPath, value, 0666)
	case ".json", ".toml", ".properties", ".props", ".prop", ".hcl", ".tfvars", ".dotenv", ".env", ".ini":
		return v.SafeWriteConfigAs(cfgPath)
	default:
		return fmt.Errorf("unsupported config format %q", ext)
	}
}

func loadConfigsFromProvieders() error {
	dir := v.GetString("providers.file.directory")
	if dir != "" {
		if err := loadConfigsFromDir(dir); err != nil {
			return err
		}
	}
	return nil
}

func loadConfigsFromDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("%q is not a dir", dir)
	}

	filePaths, err := filePathsFromDir(dir)
	if err != nil {
		return err
	}

	for _, path := range filePaths {
		if err := readConfigFile(path); err != nil {
			return err
		}
	}
	return nil
}

func readConfigFile(path string) error {
	path = strings.TrimSpace(path)
	vpr := viper.New()
	vpr.SetConfigFile(path)

	if err := vpr.ReadInConfig(); err != nil {
		return err
	}
	return v.MergeConfigMap(vpr.AllSettings())
}

func filePathsFromDir(path string) ([]string, error) {
	var filePaths []string
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePaths = append(filePaths, filepath.Join(path, file.Name()))
	}

	return filePaths, err
}
