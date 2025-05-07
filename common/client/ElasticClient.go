/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/hashicorp/go-uuid"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/util"
	"github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/startup"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/elastic/go-elasticsearch/v8/esutil"
	"github.com/pkg/errors"
	bmcmodel "hedge/common/models"
)

type ElasticClientInterface interface {
	SearchEvents(luceneQuery string) ([]*bmcmodel.HedgeEvent, error)
	SearchCommandLogs(luceneQuery string) ([]*bmcmodel.CommandExecutionLog, error)
	Search(luceneQuery string, indexName string) (map[string]interface{}, error)
	IndexEvent(bmcEvent *bmcmodel.HedgeEvent) error
	//IndexCommandLog(remediateAction *bmcmodel.CommandExecutionLog) error
	//IndexMlPrediction(mlPrediction *bmcmodel.MLPrediction) error
	Index(req esapi.IndexRequest) error
	ConvertToBMCEvents(result map[string]interface{}) ([]*bmcmodel.HedgeEvent, error)
	ConvertToCommandLogs(hitResults map[string]interface{}) ([]*bmcmodel.CommandExecutionLog, error)
	BuildSearchRequest(luceneQuery string, indexName string) esapi.SearchRequest
	Ingest(indexName string, prediction string) error
}

type HedgeElasticClient struct {
	*elasticsearch.Client
	logger                    logger.LoggingClient
	PredictionTemplatePattern string
}

type BulkResponse struct {
	Status int  `json:"status" validate:"required"`
	Errors bool `json:"errors" validate:"required"`
}

const (
	ElasticEventIndexName = "event_index"
	// ElasticRemediateIndexName is not used anymore
	ElasticRemediateIndexName = "commandlog_index"
	// ElasticMLPredictionIndexName is not completely implemented
	ElasticMLPredictionIndexName = "ml_prediction_index"
	ElasticSearchTimeout         = 60 * time.Second
)

/*
New Elastic Client creation from configuration in configuration.toml
The following configuration is required
OpenSearchURL, secrets -- username, password
elastic index name to be passed as param:
event_index for events
remediate_index for remediation
*/

func NewHedgeElasticClient(service interfaces.ApplicationService) *HedgeElasticClient {

	var elasticClient *HedgeElasticClient
	var err error

	logger := service.LoggingClient()
	logger.Info("About to create the elastic client")

	startupTimer := startup.NewStartUpTimer("elastic-client")
	for startupTimer.HasNotElapsed() {

		elasticClient, err = createHedgeElasticClient(service)
		if err == nil {
			break
		}
		elasticClient = nil
		fmt.Printf("Couldn't create Elastic client: %v", err.Error())
		startupTimer.SleepForInterval()
	}
	if elasticClient == nil {
		fmt.Printf("Failed to create Elastic client in allotted time")
		os.Exit(1)
	}
	return elasticClient
}

func createHedgeElasticClient(service interfaces.ApplicationService) (*HedgeElasticClient, error) {

	logger := service.LoggingClient()

	var elasticURL []string
	var templatePattern, username, password string
	//var secrets map[string]string
	var err error
	var skipCertVerification bool // false by default

	const (
		OPENSEARCHURL        = "OpenSearchURL"
		TEMPLATEPATTERN      = "TemplatePattern"
		SECRETS              = "secrets"
		SKIPCERTVERIFICATION = "SkipCertVerification"
	)

	properties := []string{OPENSEARCHURL, SECRETS, SKIPCERTVERIFICATION}
	errorData := ""

	for _, p := range properties {
		switch p {
		case OPENSEARCHURL:
			elasticURL, err = service.GetAppSettingStrings(OPENSEARCHURL)
			errorData = "elasticURL"
		case TEMPLATEPATTERN:
			templatePattern, err = service.GetAppSetting(TEMPLATEPATTERN)
			errorData = "templatePattern"
		case SECRETS:
			/*
				// Below code doesn't work since we don't add opensearch secrets as part of install
					secrets, err = service.SecretProvider().GetSecret("opensearch", "username", "password")
				username = secrets["username"]
				password = secrets["password"]
				errorData = "secrets"*/
			// Below works mostly may be because security is disabled, need to fix this
			username = ""
			password = ""

		case SKIPCERTVERIFICATION:
			value, _ := service.GetAppSetting(SKIPCERTVERIFICATION)
			skipCertVerification, _ = strconv.ParseBool(value)
			logger.Infof("SkipCertVerification: %t", skipCertVerification)
		}
		if err != nil {
			logger.Errorf("Could not read %s, error: %s", errorData, err.Error())
			return nil, errors.Wrapf(err, "Could not read %s", errorData)
		}
	}
	logger.Infof("OpenSearch URL: %s, template pattern: %s, skip certificate verification: %t",
		elasticURL, templatePattern, skipCertVerification)
	logger.Infof("Read opensearch secrets")

	tp := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipCertVerification},
	}
	cfg := elasticsearch.Config{
		Addresses: elasticURL,
		Username:  username,
		Password:  password,
		Transport: tp,
	}

	elasticClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		logger.Error(fmt.Sprintf("ERROR: Unable to create client: %v\n", err))
		return nil, errors.Wrapf(err, "failed to create elasticsearch client for %s", elasticURL)
	}

	res, err := elasticClient.Info()
	if err != nil {
		logger.Errorf("ERROR: Unable to get response: %s", err)
		return nil, err
	}
	defer res.Body.Close()

	return &HedgeElasticClient{
		Client: elasticClient,
		logger: logger,
		//PredictionTemplatePattern: templatePattern,
	}, nil
}

func (e *HedgeElasticClient) SearchEvents(luceneQuery string) ([]*bmcmodel.HedgeEvent, error) {
	// Print the response status and indexed document version.
	hits, err := e.Search(luceneQuery, ElasticEventIndexName)
	if err != nil {
		return nil, err
	}
	events, err := e.ConvertToBMCEvents(hits)
	//hits := hits["hits"].(map[string]interface{})
	return events, err
}

func (e *HedgeElasticClient) SearchCommandLogs(luceneQuery string) ([]*bmcmodel.CommandExecutionLog, error) {
	// Print the response status and indexed document version.
	hits, err := e.Search(luceneQuery, ElasticRemediateIndexName)
	if err != nil || hits == nil {
		return nil, err
	}
	remediateActions, err := e.ConvertToCommandLogs(hits)
	//hits := hits["hits"].(map[string]interface{})
	return remediateActions, err
}

func (e *HedgeElasticClient) Search(luceneQuery string, indexName string) (map[string]interface{}, error) {
	searchReq := e.BuildSearchRequest(luceneQuery, indexName)

	// Perform the SearchEvents request.
	res, err := searchReq.Do(context.Background(), e.Client)

	if err != nil {
		e.logger.Error(fmt.Sprintf("\"Error getting response: %v\n", err))
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		e.logger.Error(fmt.Sprintf("[%s] Error retrieving the document\n", res.Status()))
		return nil, err
	}
	// Deserialize the response into a map.
	//var hits map[string]interface{}
	hits := make(map[string]interface{}, 1)
	if err := json.NewDecoder(res.Body).Decode(&hits); err != nil {
		e.logger.Warn(fmt.Sprintf("search: Error parsing the response body: %v\n", err))
		return nil, err
	}
	return hits, nil
}

func (e *HedgeElasticClient) IndexEvent(bmcEvent *bmcmodel.HedgeEvent) error {
	e.logger.Debug("Adding to event database")
	currentTime := time.Now().UnixNano() / 1000000
	if bmcEvent.Created == 0 {
		bmcEvent.Created = currentTime // Don't overwrite createDate for existing docs
	}
	bmcEvent.Modified = currentTime

	// If ID is empty, generate one
	if len(bmcEvent.Id) == 0 {
		bmcEvent.Id, _ = uuid.GenerateUUID()
	}
	// Set up the request object.
	event, err := util.CoerceType(bmcEvent)
	if err != nil {
		return err
	}
	req := esapi.IndexRequest{
		Index:      ElasticEventIndexName,
		DocumentID: bmcEvent.Id,
		Body:       strings.NewReader(string(event)),
		Refresh:    "true",
	}

	e.logger.Debug(fmt.Sprintf("Document being saved=%s", bmcEvent.Id))
	return e.Index(req)
}

func (e *HedgeElasticClient) IndexCommandLog(remediateAction *bmcmodel.CommandExecutionLog) error {
	e.logger.Debug("Adding to Remediate action to database")
	currentTime := time.Now().UnixNano() / 1000000
	if remediateAction.Created == 0 {
		remediateAction.Created = currentTime // Don't overwrite createDate for existing docs
	}
	remediateAction.Modified = currentTime

	// If ID is empty, generate one
	if len(remediateAction.Id) == 0 {
		remediateAction.Id, _ = uuid.GenerateUUID()
	}
	// Set up the request object.
	remediation, err := util.CoerceType(remediateAction)
	if err != nil {
		return err
	}
	req := esapi.IndexRequest{
		Index:      ElasticRemediateIndexName,
		DocumentID: remediateAction.Id,
		Body:       strings.NewReader(string(remediation)),
		Refresh:    "true",
	}

	e.logger.Debug(fmt.Sprintf("Document being saved=%s", remediateAction.Id))
	err = e.Index(req)
	return err
}

func (e *HedgeElasticClient) Index(req esapi.IndexRequest) error {
	// Perform the request with the client.
	res, err := req.Do(context.Background(), e.Client)
	if err != nil {
		e.logger.Error(fmt.Sprintf("\"Error getting response: %v\n", err))
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		bodyBytes, _ := io.ReadAll(res.Body)
		e.logger.Error(fmt.Sprintf("[%s] Error response from Elasticsearch: %s", res.Status(), string(bodyBytes)))
		e.logger.Error(fmt.Sprintf("[%s] Error indexing document ID=%s\n", res.Status(), req.DocumentID))
		return errors.New(fmt.Sprintf("Error indexing document: %s\n", res.String()))
	}
	// Deserialize the response into a map.
	//var r map[string]interface{}
	r := make(map[string]interface{}, 1)

	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		e.logger.Warn(fmt.Sprintf("IndexRequest: Error parsing the response body: %v\n", err))
		return err
	}
	// Print the response status and indexed document version.
	version := int(r["_version"].(float64))
	e.logger.Debug(fmt.Sprintf("[%s] %s; version=%d", res.Status(), r["result"], version))
	return nil
}

func (e *HedgeElasticClient) ConvertToBMCEvents(result map[string]interface{}) ([]*bmcmodel.HedgeEvent, error) {

	events := make([]*bmcmodel.HedgeEvent, 0)
	if result["hits"] == nil {
		return events, nil
	}
	hits := result["hits"].(map[string]interface{})
	if hits == nil || hits["hits"] == nil {
		e.logger.Info("No data found")
		return events, nil
	}

	hitsArray := hits["hits"].([]interface{})

	for i := range hitsArray {
		element := hitsArray[i].(map[string]interface{})
		bmcEvent := new(bmcmodel.HedgeEvent)
		bmcEvent.Id = element["_id"].(string)
		events = append(events, bmcEvent)

		var source map[string]interface{}
		ok := false
		if source, ok = element["_source"].(map[string]interface{}); !ok {
			return events, nil
		}

		*bmcEvent = bmcmodel.HedgeEvent{
			Id:             e.getString(element, "_id"),
			DeviceName:     e.getString(source, "device_name"),
			Class:          e.getString(source, "class"),
			EventType:      e.getString(source, "event_type"),
			Name:           e.getString(source, "name"),
			Msg:            e.getString(source, "msg"),
			SourceNode:     e.getString(source, "source_node"),
			Severity:       e.getString(source, "severity"),
			RelatedMetrics: e.getStringList(source, "related_metrics"),
			Thresholds:     e.getMap(source, "threshold"),
			ActualValues:   e.getMap(source, "actual_values"),
			Unit:           e.getString(source, "unit"),
			Location:       e.getString(source, "location"),
			CorrelationId:  e.getString(source, "correlation_id"),
			Created:        e.getInt(source, "created"),
			Modified:       e.getInt(source, "modified"),
			Status:         e.getString(source, "status"),
			Profile:        e.getString(source, "profile"),
		}

		if source["remediations"] != nil && source["remediations"].([]interface{}) != nil {
			remediationsCollection := source["remediations"].([]interface{})
			remediations := make([]bmcmodel.Remediation, len(remediationsCollection))
			for i, remediation := range remediationsCollection {
				remediationMap := remediation.(map[string]interface{})
				remediations[i] = bmcmodel.Remediation{
					Id:           e.getString(remediationMap, "id"),
					Type:         e.getString(remediationMap, "type"),
					Summary:      e.getString(remediationMap, "summary"),
					Status:       e.getString(remediationMap, "status"),
					ErrorMessage: e.getString(remediationMap, "error_message"),
				}

			}
			bmcEvent.Remediations = remediations
		}

		// if the status has not changed, retain the additional data that was present earlier
		// The rule keeps firing and rule output doesn't include additionalData
		additionalData, ok := source["additional_data"].(map[string]interface{})
		if ok {
			bmcEvent.AdditionalData = make(map[string]string)
			for key, val := range additionalData {
				if _, ok := val.(string); ok {
					bmcEvent.AdditionalData[key], _ = val.(string)
				}
			}
		}

	}

	return events, nil
}

func (e *HedgeElasticClient) ConvertToCommandLogs(hitResults map[string]interface{}) ([]*bmcmodel.CommandExecutionLog, error) {

	hits := hitResults["hits"].(map[string]interface{})

	commandLogs := make([]*bmcmodel.CommandExecutionLog, 0)
	if hits == nil || hits["hits"] == nil {
		e.logger.Info("No data found")
		return commandLogs, nil
	}

	hitsArray := hits["hits"].([]interface{})

	for i := range hitsArray {
		element := hitsArray[i].(map[string]interface{})
		commandLog := new(bmcmodel.CommandExecutionLog)
		commandLog.Id = element["_id"].(string)
		commandLogs = append(commandLogs, commandLog)
		var source map[string]interface{}

		ok := false
		if source, ok = element["_source"].(map[string]interface{}); !ok {
			return commandLogs, nil
		}

		*commandLog = bmcmodel.CommandExecutionLog{
			Id:            e.getString(element, "_id"),
			DeviceName:    e.getString(source, "deviceName"),
			CommandType:   e.getString(source, "commandType"),
			Problem:       e.getString(source, "problem"),
			TicketId:      e.getString(source, "ticketId"),
			Summary:       e.getString(source, "summary"),
			Severity:      e.getString(source, "severity"),
			EventId:       e.getString(source, "eventId"),
			CorrelationId: e.getString(source, "correlationId"),
			Created:       e.getInt(source, "created"),
			Modified:      e.getInt(source, "modified"),
			Status:        e.getString(source, "status"),
			SLA:           e.getString(source, "SLA"),
			TicketType:    e.getString(source, "ticketType"),
		}
		if len(commandLog.TicketId) > 0 {
			e.logger.Info(fmt.Sprintf("Event found with msg: %s\n", commandLog.TicketId))
		}
	}
	return commandLogs, nil
}

/*
Utility method to build a SearchRequest
luceneQuery example:
correlationId: \"someCorreId\" AND _id:"\someid\"
*/
func (e *HedgeElasticClient) BuildSearchRequest(luceneQuery string, indexName string) esapi.SearchRequest {
	index := []string{indexName}
	searchReq := esapi.SearchRequest{}
	searchReq.Index = index
	searchReq.Query = luceneQuery // lucene query
	searchReq.Pretty = true
	return searchReq
}

func (e *HedgeElasticClient) Ingest(indexName string, toIngest string) error {

	if e.PredictionTemplatePattern == "" {
		return errors.New("PredictionTemplatePattern is not set")
	}

	// Construct the request body for ingesting the sensor data into Elasticsearch.
	indexName = e.PredictionTemplatePattern + strings.ToLower(indexName)
	e.logger.Infof("Ingestion Data: %s", toIngest)

	bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Index:   indexName, // The default index name
		Client:  e.Client,  // The Elasticsearch client
		Refresh: "wait_for",
		Timeout: ElasticSearchTimeout,
		OnError: func(ctx context.Context, err error) {
			if err != nil {
				e.logger.Errorf("ERROR: %s", err)
			}
		},
	})
	if err != nil {
		e.logger.Infof("error in creating bulk indexer: %s", err.Error())
		return err
	}

	e.logger.Infof("Ingesting data with bulk indexer, index name: %s", indexName)
	ctx, cancelFunc := context.WithTimeout(context.Background(), ElasticSearchTimeout)
	defer cancelFunc()
	err = bi.Add(ctx, esutil.BulkIndexerItem{
		Index:      indexName,
		Action:     "create",
		DocumentID: "_prediction-" + time.Now().GoString(), // The default document ID
		Body:       strings.NewReader(toIngest),            // The sensor data as the document body
		OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
			if err != nil {
				e.logger.Errorf("error: %s", err)
			} else {
				e.logger.Errorf("error: %s: %s", res.Error.Type, res.Error.Reason)
			}
		},
		OnSuccess: func(ctx context.Context, bii esutil.BulkIndexerItem, biri esutil.BulkIndexerResponseItem) {
			e.logger.Infof("Ingested document ID: %+v", biri)
		},
	})
	defer e.closeBulkIndexer(bi)

	if err != nil {
		e.logger.Errorf("Ingestion failed: %s", err.Error())
		return err
	}

	e.logger.Info("Ingestion succeeded")

	return nil
}

func (e *HedgeElasticClient) closeBulkIndexer(bi esutil.BulkIndexer) error {
	err := bi.Close(context.Background())
	if err != nil {
		e.logger.Errorf("Could not close bulk indexer: %s", err.Error())
	}
	return err
}

func (e *HedgeElasticClient) getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		return val.(string)
	}
	return ""
}

func (e *HedgeElasticClient) getStringList(data map[string]interface{}, key string) []string {
	if val, ok := data[key]; ok && val != nil {
		if val2, ok := val.([]string); ok {
			return val2
		}
	}
	return []string{}
}

func (e *HedgeElasticClient) getInt(data map[string]interface{}, key string) int64 {
	if val, ok := data[key]; ok && val != nil {
		if val2, ok := val.(int64); ok {
			return val2
		}
	}
	return 0
}

func (e *HedgeElasticClient) getMap(data map[string]interface{}, key string) map[string]interface{} {
	if val, ok := data[key]; ok && val != nil {
		if val2, ok := val.(map[string]interface{}); ok {
			return val2
		}
	}
	return map[string]interface{}{}
}
