package packages

import (
	"package-operator.run/internal/packages/internal/packagerepository"
)

type (
	RepositoryIndex      = packagerepository.RepositoryIndex
	MultiRepositoryIndex = packagerepository.MultiRepositoryIndex
	Entry                = packagerepository.Entry
	RepositoryStore      = packagerepository.RepositoryStore
)

var (
	NewMultiRepositoryIndex = packagerepository.NewMultiRepositoryIndex
	NewRepositoryIndex      = packagerepository.NewRepositoryIndex
	LoadRepositoryFromFile  = packagerepository.LoadRepositoryFromFile
	LoadRepository          = packagerepository.LoadRepository
	SaveRepositoryToFile    = packagerepository.SaveRepositoryToFile
	SaveRepositoryToOCI     = packagerepository.SaveRepositoryToOCI
	LoadRepositoryFromOCI   = packagerepository.LoadRepositoryFromOCI
	NewRepositoryStore      = packagerepository.NewRepositoryStore
)
