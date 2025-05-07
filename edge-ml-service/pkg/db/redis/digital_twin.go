package redis

import (
	"hedge/common/db"
	"hedge/common/db/redis"
	hedgeErrors "hedge/common/errors"
	"hedge/edge-ml-service/pkg/dto/twin"
	"errors"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	redigo "github.com/gomodule/redigo/redis"
)

type TwinDB interface {
	DBAddTwinDefinition(twinDefinition twin.DigitalTwinDefinition) error
	DBUpdateTwinDefinition(twinDefinition twin.DigitalTwinDefinition) error
	DBGetTwinDefinition(key string) (*twin.DigitalTwinDefinition, error)
	DBDeleteTwinDefinition(id string) error

	DBGetTwinDefinitions() ([]*twin.DigitalTwinDefinition, error)

	// Add methods for twin instances

	DBGetDefinition(key string) (string, error)
	DBGetDefinitions(key string) ([]string, error)
	DBAddSimulationDefinition(jsonData []byte, name string) error
	DBGetSimulationDefinitions(key string) ([]string, error)
	DBGetSimulationDefinition(key string) (string, error)
	DBGetObject(data []byte, unMarshalCallback func(data []byte) (interface{}, error)) (interface{}, error)
	DBQueryByKey(key string) (interface{}, error)

	DBDeleteSimulationDefinition(id string) error
}

type DBLayer struct {
	*redis.DBClient
}

func NewDBLayer(c *redis.DBClient, lc logger.LoggingClient) TwinDB {
	dbLayer := &DBLayer{DBClient: c}
	dbLayer.Logger = lc
	return dbLayer
}

func (d *DBLayer) DBAddTwinDefinition(twinDefinition twin.DigitalTwinDefinition) error {

	errorMessage := fmt.Sprintf("Error creating algorithm %s", twinDefinition.Name)

	dbClient := d.DBClient
	lc := dbClient.Logger
	conn := dbClient.Pool.Get()
	defer conn.Close()

	key := d.twinDefinitionKey(twinDefinition.Name)

	exists, err := redigo.Bool(conn.Do("HEXISTS", key, twinDefinition.Name))
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	if exists {
		lc.Errorf("%s: %v", errorMessage, fmt.Sprintf("DigitalTwin Definition '%s' already exists", twinDefinition.Name))
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("Algorithm '%s' already exists", twinDefinition.Name))
	}

	m, err := marshalObject(twinDefinition)
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	_ = conn.Send("MULTI")
	_ = conn.Send("SET", key, m)
	_ = conn.Send("SADD", db.TwinDefinition, key)
	_ = conn.Send("HSET", db.TwinDefinition+":name", twinDefinition.Name, twinDefinition.Name)

	_, err = conn.Do("EXEC")
	if err != nil {
		lc.Errorf("%s: error saving algorithm definition: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}
	return nil

}

func (d *DBLayer) DBUpdateTwinDefinition(twinDefinition twin.DigitalTwinDefinition) error {

	errorMessage := fmt.Sprintf("Error creating algorithm %s", twinDefinition.Name)

	dbClient := d.DBClient
	lc := dbClient.Logger
	conn := dbClient.Pool.Get()
	defer conn.Close()

	exists, err := redigo.Bool(conn.Do("HEXISTS", db.TwinDefinition+":name", twinDefinition.Name))
	if errors.Is(err, redigo.ErrNil) {
		lc.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("digitalTwin '%s' not found", twinDefinition.Name))
	}

	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	if !exists {
		lc.Errorf("%s: %v", errorMessage, fmt.Sprintf("digitalTwin '%s' not found", twinDefinition.Name))
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("digitalTwin '%s' not found", twinDefinition.Name))
	}

	m, err := marshalObject(twinDefinition)
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	key := d.twinDefinitionKey(twinDefinition.Name)
	_ = conn.Send("MULTI")
	_ = conn.Send("SET", key, m)

	_, err = conn.Do("EXEC")
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return nil
}

func (d *DBLayer) DBGetTwinDefinition(name string) (*twin.DigitalTwinDefinition, error) {
	errorMessage := fmt.Sprintf("Error geting digitalTwinDefinition %s", name)
	dbClient := d.DBClient
	lc := dbClient.Logger
	conn := dbClient.Pool.Get()
	defer conn.Close()

	key := d.twinDefinitionKey(name)

	m, err := redigo.Bytes(conn.Do("GET", key))
	if err != nil {
		// not found
		if errors.Is(err, redigo.ErrNil) {
			return nil, nil
		}
		lc.Errorf("Error getting algorithm definition: %s", err.Error())
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	var digiTwinDefn twin.DigitalTwinDefinition
	err = unmarshalObject(m, &digiTwinDefn)
	if err != nil {
		lc.Errorf("error while unmarshalling digital twin definition: %s", err.Error())
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	return &digiTwinDefn, nil
}

func (d *DBLayer) DBDeleteTwinDefinition(name string) error {
	dbClient := d.DBClient
	lc := dbClient.Logger
	conn := dbClient.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Error deleting twin definition %s", name)

	key := d.twinDefinitionKey(name)

	_ = conn.Send("MULTI")
	_ = conn.Send("DEL", key)
	_ = conn.Send("SREM", db.TwinDefinition, key)
	_ = conn.Send("HDEL", db.TwinDefinition+":name", name)

	_, err := conn.Do("EXEC")
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}
	return nil
}

func (d *DBLayer) DBGetTwinDefinitions() ([]*twin.DigitalTwinDefinition, error) {
	dbClient := d.DBClient
	lc := dbClient.Logger
	conn := dbClient.Pool.Get()
	defer conn.Close()
	errorMessage := "Error getting all twin definitions"

	objects, err := redis.GetObjectsByValue(conn, db.TwinDefinition)
	if err != nil {
		lc.Errorf("Error getting twin definitions from DB: %v", err)
		return []*twin.DigitalTwinDefinition{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	twins := make([]*twin.DigitalTwinDefinition, len(objects))
	for i, object := range objects {
		err = unmarshalObject(object, &twins[i])
		if err != nil {
			lc.Errorf("Error unmarshalling digital twin definition: %v", err)
			return []*twin.DigitalTwinDefinition{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		}
	}

	return twins, nil
}

func (d *DBLayer) DBAddSimulationDefinition(jsonData []byte, name string) error {
	key := db.SimulationDefinition + ":" + name
	_, err := d.Do("SET", key, string(jsonData))
	if err != nil {
		return err
	}
	return nil
}
func (d *DBLayer) DBGetDefinition(key string) (string, error) {
	return redigo.String(d.DBQueryByKey(key))
}

func (d *DBLayer) DBGetSimulationDefinition(key string) (string, error) {
	return d.DBGetDefinition(key)
}

func (d *DBLayer) DBGetDefinitions(key string) ([]string, error) {
	return redigo.Strings(d.QueryByPattern(key))
}
func (d *DBLayer) DBGetSimulationDefinitions(key string) ([]string, error) {
	return d.DBGetDefinitions(key)
}

func (d *DBLayer) DBGetObject(data []byte, unMarshalCallback func(data []byte) (interface{}, error)) (interface{}, error) {
	return unMarshalCallback(data)
}

func (d *DBLayer) DBQueryByKey(key string) (interface{}, error) {
	return d.Do("GET", key)
}

func (d *DBLayer) QueryByPattern(pattern string) (interface{}, error) {
	return d.Do("KEYS", pattern)
}

func (d *DBLayer) DBDeleteSimulationDefinition(id string) error {
	key := db.SimulationDefinition + ":" + id
	_, err := d.Do("DEL", key)
	return err
}

func (d *DBLayer) twinDefinitionKey(name string) string {
	return db.TwinDefinition + "|" + name
}
