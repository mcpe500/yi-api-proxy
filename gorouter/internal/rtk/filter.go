package rtk

type Filter interface {
	Process(content string) (string, int)
	Name() string
}