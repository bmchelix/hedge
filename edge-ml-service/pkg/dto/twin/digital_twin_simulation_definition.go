package twin

func NewSimulationDefinition(mlModelName, twinDefinitionName string) *SimulationDefinition {
	return &SimulationDefinition{
		RunType:              OneTimeRun,
		RunControl:           UserDriven,
		TwinDefinitionName:   twinDefinitionName,
		SimulationParameters: make([]ExternalParameter, 0),
	}
}
