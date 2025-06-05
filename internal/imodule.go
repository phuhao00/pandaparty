package internal

type IModule interface {
	OnStart()
	OnStop()
	GetName() string
}
