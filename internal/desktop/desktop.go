package desktop

type Application struct {
	Name, Path, ID, Class, Exec string
	Args                        []string
}
