package provider

type Type byte

const (
	NilType Type = iota
	ConfigType
	FileType
	DockerType
	KubernetesType
)

func (t Type) String() string {
	switch t {
	case ConfigType:
		return "config"
	case FileType:
		return "file"
	case DockerType:
		return "docker"
	case KubernetesType:
		return "kubernetes"
	}

	return "unknown"
}

type Data struct {
	Type   Type
	Config map[string]any
}

func (d Data) IsNil() bool {
	return d.Type == NilType || d.Config == nil
}

type Provider interface {
	Provide(chan<- Data) (Data, error)
	Close() error
}
