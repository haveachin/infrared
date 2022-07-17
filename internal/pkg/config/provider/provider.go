package provider

type Type byte

const (
	NilType Type = iota
	BaseType
	FileType
	DockerType
)

func (t Type) String() string {
	switch t {
	case BaseType:
		return "base"
	case FileType:
		return "file"
	case DockerType:
		return "docker"
	}
	return "unknown"
}

type Data struct {
	Type   Type
	Config map[string]interface{}
}

func (d Data) IsNil() bool {
	return d.Type == NilType || d.Config == nil
}

type Provider interface {
	Provide(chan<- Data) (Data, error)
	Close() error
}
