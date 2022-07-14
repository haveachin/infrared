package provider

type Type byte

const (
	FileType Type = iota
	DockerType
)

type Data struct {
	Type   Type
	Config map[string]interface{}
}

type Provider interface {
	Provide(chan<- Data) error
	Close() error
}
