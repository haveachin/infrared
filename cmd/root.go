package cmd

import (
	"bytes"
	_ "embed"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"
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

//go:embed config.default.yml
var defaultConfig []byte

var (
	v          *viper.Viper
	configPath string

	rootCmd = &cobra.Command{
		Use:   "infrared",
		Short: "Starts the infrared proxy",
		Run: func(cmd *cobra.Command, args []string) {
			logger, err := zap.NewDevelopment()
			if err != nil {
				log.Fatalf("Failed to init logger; err: %s", err)
			}

			logger.Info("loading proxy from config",
				zap.String("config", configPath),
			)

			if err := loadConfig(); err != nil {
				logger.Error("failed to load config",
					zap.Error(err),
					zap.String("config", configPath),
				)
			}

			bedrockProxy, err := infrared.NewProxy(&bedrock.ProxyConfig{Viper: v})
			if err != nil {
				logger.Error("failed to load proxy", zap.Error(err))
				return
			}

			javaProxy, err := infrared.NewProxy(&java.ProxyConfig{Viper: v})
			if err != nil {
				logger.Error("failed to load proxy", zap.Error(err))
				return
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
				logger.Error("failed to enable plugins", zap.Error(err))
				return
			}

			logger.Info("starting proxy")

			go bedrockProxy.ListenAndServe(logger)
			go javaProxy.ListenAndServe(logger)

			sc := make(chan os.Signal, 1)
			signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
			<-sc

			logger.Info("disabeling plugins")
			pluginManager.DisablePlugins()
		},
	}
)

func init() {
	v = viper.New()
	v.SetEnvPrefix("INFRARED")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	rootCmd.Flags().StringVarP(&configPath, "config", "c", "config.yml", "path of the config file")
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func loadConfig() error {
	v.SetConfigType("yml")
	if err := v.ReadConfig(bytes.NewReader(defaultConfig)); err != nil {
		return err
	}

	configPath = strings.TrimSpace(configPath)
	dir, file := path.Split(configPath)
	ext := path.Ext(file)
	fileName := strings.TrimSuffix(file, ext)
	if dir == "" {
		dir = "."
	}

	v.SetConfigName(fileName)
	v.AddConfigPath(dir)
	v.SetConfigType(strings.TrimPrefix(ext, "."))

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return ioutil.WriteFile(configPath, defaultConfig, 0666)
		}
		return err
	}
	return nil
}
