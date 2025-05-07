package twin

func NewDigitalTwinDefinition() *DigitalTwinDefinition {
	return &DigitalTwinDefinition{
		Connections: make([]Connection, 0),
		//ExternalParameters: make([]ExternalParameter, 0),
		Entities: make([]Entity, 0),
		Status:   StatusTypeEnabled,
	}
}

func (d *DigitalTwinDefinition) AddTwinConnection(c Connection) *DigitalTwinDefinition {

	if d.Connections == nil {
		d.Connections = make([]Connection, 0)
	}
	d.Connections = append(d.Connections, c)

	return d
}

func (d *DigitalTwinDefinition) AddEntity(entity Entity) *DigitalTwinDefinition {
	if d.Entities == nil {
		d.Entities = make([]Entity, 0)
	}
	d.Entities = append(d.Entities, entity)
	return d
}
