package internal

type IServer interface {
	Start()
	Stop()
	GetServerName() string
}
