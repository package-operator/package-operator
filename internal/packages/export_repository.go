package packages

import "package-operator.run/internal/packages/internal/packagerepository"

type RepositoryIndex = packagerepository.RepositoryIndex

var (
	NewRepositoryIndex     = packagerepository.NewRepositoryIndex
	LoadRepositoryFromFile = packagerepository.LoadRepositoryFromFile
	LoadRepository         = packagerepository.LoadRepository
	SaveRepositoryToFile   = packagerepository.SaveRepositoryToFile
)
