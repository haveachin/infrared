package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesConfig struct {
	AnnotationPrefix string        `mapstructure:"annotationPrefix"`
	Namespace        string        `mapstructure:"namespace"`
	ClientTimeout    time.Duration `mapstructure:"clientTimeout"`
	Watch            bool          `mapstructure:"watch"`
	Config           string        `mapstructure:"config"`
}

type Kubernetes struct {
	KubernetesConfig
	clientset *kubernetes.Clientset
	logger    *zap.Logger
}

func NewKubernetes(cfg KubernetesConfig, logger *zap.Logger) Provider {
	return &Kubernetes{
		KubernetesConfig: cfg,
		logger:           logger,
	}
}

func (p *Kubernetes) Provide(dataCh chan<- Data) (Data, error) {
	var config *rest.Config
	var err error

	// see if any config is specified at all and if so if we run inCluster
	switch p.Config {
	case "":
		return Data{}, nil
	case "inCluster":
		config, err = clientcmd.BuildConfigFromFlags("", "")
	default:
		config, err = clientcmd.BuildConfigFromFlags("", p.Config)
	}

	if err != nil {
		return Data{}, errors.Wrap(err, "Could not load kube config file")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return Data{}, errors.Wrap(err, "Could not create kube clientset")
	}

	p.clientset = clientset

	p.logger.Info("Monitoring Kubernetes for Minecraft services")

	cfg, err := p.readConfigData()
	if err != nil {
		return Data{}, err
	}

	if p.Watch {
		go func() {
			if err := p.watch(dataCh); err != nil {
				p.logger.Error("failed while watching provider",
					zap.Error(err),
					zap.String("provider", KubernetesType.String()),
				)
			}
		}()
	}

	return Data{
		Type:   KubernetesType,
		Config: cfg,
	}, nil
}

func (p Kubernetes) readConfigData() (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.ClientTimeout)
	defer cancel()

	services, err := p.clientset.CoreV1().Services(p.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	cfg := map[string]any{}
	for _, service := range services.Items {
		annotations := service.GetAnnotations()

		for key, value := range annotations {
			if !strings.HasPrefix(key, p.AnnotationPrefix) {
				continue
			}

			key = strings.TrimPrefix(key, p.AnnotationPrefix)

			if strings.HasPrefix(value, "[") {
				value = strings.Trim(value, "[]")
				setNestedValue(cfg, key, strings.Split(value, ","))
			} else {
				setNestedValue(cfg, key, value)
			}
		}

	}
	return cfg, nil
}

func (p Kubernetes) watch(dataCh chan<- Data) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher, err := p.clientset.CoreV1().Services(p.Namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	ch := watcher.ResultChan()
	for {
		// Wait for any update
		event := <-ch

		// There will be an item in the channel for *every* service that
		// is updated. Especially on start-up there will be a storm of
		// events but we really want to update just once. So wait a short
		// time for more events and then drain the channel.
		time.Sleep(100 * time.Millisecond)
		for drained := false; drained == false; {
			select {
			case <-ch:
			default:
				drained = true
			}
		}

		if event.Type == watch.Added || event.Type == watch.Deleted {
			fmt.Printf("Service %s\n", event.Type)
			cfg, err := p.readConfigData()
			if err != nil {
				p.logger.Info("failed to read data", zap.Error(err))
				continue
			}

			dataCh <- Data{
				Type:   KubernetesType,
				Config: cfg,
			}
		}

	}
}

func (p Kubernetes) Close() error {
	return nil
}
