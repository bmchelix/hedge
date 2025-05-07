/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package helpers

import (
	"encoding/json"
	"strconv"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
)

const (
	ANOMALY_ALGO_TYPE        = "Anomaly"
	TIMESERIES_ALGO_TYPE     = "Timeseries"
	CLASSIFICATION_ALGO_TYPE = "Classification"
	REGRESSION_ALGO_TYPE     = "Regression"

	ANOMALY_PAYLOAD_SAMPLE        = "{\n\t\"correlation-id-01\" : [3, 3.14, 5],\n\t\"correlation-id-02\" : [5, 2.71, 3]\n}"
	TIMESERIES_PAYLOAD_SAMPLE     = "{\n\t\"correlation-id-01\" : {\n\t\t\"data\" : [[3, 3.14, 5]],\n\t\t\"confidence_interval\" : {\n\t\t\t\"min\" : 0.6,\n\t\t\t\"max\" : 0.95\n\t\t}\n\t},\n\t\"correlation-id-02\" : {\n\t\t\"data\" : [[5, 2.71, 3],[5.4, 1.41, 6],[5.4, 1.41, 6]],\n\t\t\"confidence_interval\" : {\n\t\t\t\"min\" : 0.6,\n\t\t\t\"max\" : 0.95\n\t\t}\n\t}\n}"
	CLASSIFICATION_PAYLOAD_SAMPLE = "{\n\t\"correlation-id-01\" : [3, 3.14, 5],\n\t\"correlation-id-02\" : [5, 2.71, 3]\n}"
	REGRESSION_PAYLOAD_SAMPLE     = "{\n\t\"correlation-id-01\" : [3, 3.14, 5],\n\t\"correlation-id-02\" : [5, 2.71, 3]\n}"

	ANOMALY_PAYLOAD_TEMPLATE        = "{\n\t{{- $length := len .CollectedData -}}\n\t{{- $counter := 0 -}}\n\t{{- range $correlationID, $inferenceData := .CollectedData }}\n\t\"{{ $correlationID }}\" : [\n\t\t{{- range $i, $value := index $inferenceData.Data 0 -}}\n\t\t{{ if $i }}, {{ end }}{{- if eq (printf \"%T\" $value) \"string\" -}}\"{{ $value }}\"{{- else -}}{{ $value }}{{- end -}}\n\t\t{{- end -}}\n\t]{{ if lt (add $counter 1) $length }},{{ end }}\n\t{{- $counter = (add $counter 1) -}}\n\t{{- end -}}\n\t}"
	TIMESERIES_PAYLOAD_TEMPLATE     = "{\n\t{{- $length := len .CollectedData -}}\n\t{{- $counter := 0 -}}\n\t{{- range $correlationID, $inferenceData := .CollectedData }}\n\t\"{{ $correlationID }}\" : {\n\t\t\"data\" : [\n\t\t\t{{- range $rowIndex, $row := $inferenceData.Data -}}\n\t\t\t[{{- range $i, $value := $row -}}{{ if $i }}, {{ end }}{{- if eq (printf \"%T\" $value) \"string\" -}}\"{{ $value }}\"{{- else -}}{{ $value }}{{- end -}}{{- end }}]{{ if lt $rowIndex (sub (len $inferenceData.Data) 1) }},{{ end }}\n\t\t\t{{- end -}}\n\t\t]\n\t\t{{- if and (ne $.HyperParameters.confidence_min nil) (ne $.HyperParameters.confidence_max nil) -}},\n\t\t\"confidence_interval\" : {\n\t\t\t\"min\" : {{ $.HyperParameters.confidence_min }},\n\t\t\t\"max\" : {{ $.HyperParameters.confidence_max }}\n\t\t}\n\t\t{{- end -}}\n\t}{{ if lt (add 1 $counter) $length }},{{ end }}\n\t{{- $counter = (add $counter 1) -}}\n\t{{- end -}}\n}"
	CLASSIFICATION_PAYLOAD_TEMPLATE = "{\n\t{{- $length := len .CollectedData -}}\n\t{{- $counter := 0 -}}\n\t{{- range $correlationID, $inferenceData := .CollectedData }}\n\t\"{{ $correlationID }}\" : [\n\t\t{{- range $i, $value := index $inferenceData.Data 0 -}}\n\t\t{{ if $i }}, {{ end }}{{- if eq (printf \"%T\" $value) \"string\" -}}\"{{ $value }}\"{{- else -}}{{ $value }}{{- end -}}\n\t\t{{- end -}}\n\t]{{ if lt (add $counter 1) $length }},{{ end }}\n\t{{- $counter = (add $counter 1) -}}\n\t{{- end -}}\n\t}"
	REGRESSION_PAYLOAD_TEMPLATE     = "{\n\t{{- $length := len .CollectedData -}}{{- $counter := 0 -}}{{- range $correlationID, $inferenceData := .CollectedData }}\n\t\"{{ $correlationID }}\" : [{{- $lastIndex := sub (len (index $inferenceData.Data 0)) 1 -}}{{- range $i, $value := index $inferenceData.Data 0 -}}{{- if lt $i $lastIndex -}}{{ if $i }}, {{ end }}{{- if eq (printf \"%T\" $value) \"string\" -}}\"{{ $value }}\"{{- else -}}{{ $value }}{{- end -}}{{- end -}}{{- end -}}]{{ if lt (add $counter 1) $length }},{{ end }}\n\t{{- $counter = (add $counter 1) -}}{{- end -}}\n\t}"
)

const DATA_COLLECTOR_NAME string = "DataCollector"

func BuildFeatureName(profile string, featureName string) string {
	// if feature.Type == "METRIC" {
	return profile + "#" + featureName

}

// Convert sample content of CSV Array to JSON, of size nLines
func ConvCSVArrayToJSON(cSVArray []string) ([]byte, string) {

	if len(cSVArray) < 2 {
		return nil, "data not available"
	}

	// header data
	headersArr := strings.Split(cSVArray[0], ",")

	jSONStr := "["
	for i := 1; i < len(cSVArray); i++ {

		// append comma to JSON string for 2nd or higher data point
		if i > 1 {
			jSONStr += ","
		}

		jSONStr += "{"
		// array splitRow used for getting data of each feature
		splitRow := strings.Split(cSVArray[i], ",")
		for j, y := range splitRow {
			jSONStr += `"` + headersArr[j] + `":`
			_, fErr := strconv.ParseFloat(y, 32)
			_, bErr := strconv.ParseBool(y)
			switch {
			case fErr == nil:
				jSONStr += y
			case bErr == nil:
				jSONStr += strings.ToLower(y)
			default:
				jSONStr += `"` + y + `"`
			}
			// end of property, array splitRow size is used for correct JSON string structure
			if j < len(splitRow)-1 {
				jSONStr += ","
			}

		}
		// end of object of the array
		jSONStr += "}"

	}
	jSONStr += `]`

	rawMessage := json.RawMessage(jSONStr)
	x, err := json.MarshalIndent(rawMessage, "", "  ")
	if err != nil {
		// failure to convert input data string array to JSON
		return nil, err.Error()
	} else {
		return x, ""
	}
}

func CreateTemplate(t string, algoType string) (*template.Template, error) {
	funcMap := sprig.TxtFuncMap()
	tmpl, err := template.New(algoType).Funcs(funcMap).Parse(t)
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}
