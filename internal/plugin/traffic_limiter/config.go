package traffic_limiter

import (
	"fmt"

	"github.com/c2h5oh/datasize"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock"
	"github.com/haveachin/infrared/internal/pkg/java"
	"github.com/imdario/mergo"
)

type trafficLimiterConfig struct {
	ServerIDs             []string                `mapstructure:"serverIds"`
	File                  string                  `mapstructure:"file"`
	TrafficLimit          datasize.ByteSize       `mapstructure:"trafficLimit"`
	ResetCron             string                  `mapstructure:"resetCron"`
	OutOfBandwidthStatus  java.ServerStatusConfig `mapstructure:"outOfBandwidthStatus"`
	OutOfBandwidthMessage string                  `mapstructure:"outOfBandwidthMessage"`
}

func (cfg PluginConfig) loadTrafficLimiterConfigs() (map[infrared.ServerID]trafficLimiter, error) {
	trafficLimiters := map[infrared.ServerID]trafficLimiter{}
	storages := map[string]storage{}
	for _, bwCfg := range cfg.TrafficLimiter.TrafficLimiters {
		if err := mergo.Merge(&bwCfg, cfg.Defaults.TrafficLimiter); err != nil {
			return nil, err
		}

		storage, ok := storages[bwCfg.File]
		if !ok {
			var err error
			storage, err = newYAMLStorage(bwCfg.File)
			if err != nil {
				return nil, err
			}
			storages[bwCfg.File] = storage
		}

		statusJSON, err := java.NewServerStatus(bwCfg.OutOfBandwidthStatus)
		if err != nil {
			return nil, err
		}

		javaDisconnecter, err := java.NewPlayerDisconnecter(statusJSON.ResponseJSON(), bwCfg.OutOfBandwidthMessage)
		if err != nil {
			return nil, err
		}

		disconnecters := map[infrared.Edition]infrared.PlayerDisconnecter{
			infrared.JavaEdition:    javaDisconnecter,
			infrared.BedrockEdition: bedrock.NewPlayerDisconnecter(bwCfg.OutOfBandwidthMessage),
		}

		for _, sID := range bwCfg.ServerIDs {
			serverID := infrared.ServerID(sID)
			_, ok := trafficLimiters[serverID]
			if ok {
				return nil, fmt.Errorf("server with ID %q already has a traffic limiter", sID)
			}

			trafficLimiters[serverID] = trafficLimiter{
				file:                       bwCfg.File,
				trafficLimit:               bwCfg.TrafficLimit,
				resetCron:                  bwCfg.ResetCron,
				storage:                    storage,
				OutOfBandwidthDisconnecter: infrared.NewMultiPlayerDisconnecter(disconnecters),
			}
		}
	}

	return trafficLimiters, nil
}
