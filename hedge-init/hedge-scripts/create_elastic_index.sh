#!/bin/sh
# For ES index creation, make sure one of valid ES profile is set: "all", "es" or "demo"
case "$COMPOSE_PROFILES" in
  *all*|*es*|*demo*)
    echo "COMPOSE_PROFILES=$COMPOSE_PROFILES" ;;
  *)
    echo -e "COMPOSE_PROFILES flag does not contain either "all", "es" or "demo" profile names. COMPOSE_PROFILES=$COMPOSE_PROFILES. Skip creating elasticsearch index..";
    return 0 ;;
esac

echo -e "Waiting for Hedge Elasticsearch to be up";
# 'getent' for DNS check, 'nc' for port availability check
while ! ( getent hosts hedge-elasticsearch > /dev/null &&
          nc -z hedge-elasticsearch 9200 );
do sleep 5;
  echo -e "still waiting for Hedge Elasticsearch...";
done;
echo -e "  >> Hedge Elasticsearch has started. Continuing ahead..";

#### Validation ends ####
##########################
echo -e "\nCreating event index..\n"
retrycnt=0
# 10 retry attempts to create the ES index. This is to ensure ES is ready to create new indices
while [ $((retrycnt+=1)) -lt 10 ]
do
  http_response=$(curl --location -k --request PUT -o - -w "%{http_code}" 'http://hedge-elasticsearch:9200/event_index' \
  --header 'Content-Type: application/json' \
  --data-raw '{
      "settings": {
          "number_of_shards": 1,
          "number_of_replicas": 1
      },
      "mappings": {
          "properties": {
              "device_name": {
                  "type": "keyword"
              },
              "class": {
                  "type": "keyword"
              },
              "name": {
                  "type": "keyword"
              },
              "msg": {
                  "type": "text"
              },
              "status": {
                  "type": "keyword"
              },
              "severity": {
                  "type": "keyword"
              },
              "additional_data" :{
               "type" : "flat_object"
              },
              "thresholds": {
                 "type": "flat_object"
              },
              "actual_values": {
                  "type": "flat_object"
              },
              "remediations": {
                  "type": "flat_object"
              },
              "version": {
                  "type": "keyword"
              },
              "created": {
                  "type": "date",
                  "format": "epoch_millis"
              },
              "modified": {
                  "type": "date",
                  "format": "epoch_millis"
              },
              "source_node": {
                  "type": "keyword"
              },
              "correlation_id": {
                  "type": "keyword"
              }
          }
      }
  }')

  resp_code=$(echo "$http_response" | tail -c 4)            # Last 3 characters are the status code
  resp_body=$(echo "$http_response" | head -c -3)       # All except the last 3 characters

  if [ "$resp_code" -eq 400 ] || [ "$resp_code" -eq 200 ]; then
    echo "Response status=$http_response"
    break
  else
    echo "Failed to create ES index. Retrying [Attempt #$retrycnt].."
  fi
done


echo -e "\n\nCreating devicemap index.."
curl --location -k --request PUT 'http://hedge-elasticsearch:9200/devicemap_index' \
--header 'Content-Type: application/json' \
--data-raw '{
    "settings": {
        "number_of_shards": 1,
        "number_of_replicas": 1
    },
    "mappings": {
        "properties": {
            "deviceName": {
                "type": "keyword"
            },
            "region":{
                "type": "keyword"
            },
            "profile":{
                "type": "keyword"
            },
            "latitude": {
                "type": "keyword"
            },
            "longitude": {
                "type": "keyword"
            },
            "created": {
                "type": "date",
                "format": "epoch_millis"
            }
        }
    }
}'

echo -e "\n\nCreating ml_prediction index.."
curl --location -k --request PUT 'http://hedge-elasticsearch:9200/ml_prediction_index' \
--header 'Content-Type: application/json' \
--data-raw '{
    "settings": {
        "number_of_shards": 1,
        "number_of_replicas": 1
    },
    "mappings": {
        "properties": {
            "entity_type": {
                "type": "keyword"
            },
            "entity_name": {
                "type": "keyword"
            },
            "devices": {
                "type": "text"
            },
            "prediction_message": {
                "type": "text"
            },
            "ml_algo_name": {
                "type": "keyword"
            },
            "model_name": {
                "type": "keyword"
            },
            "prediction": {
                "type": "object"
            },
            "prediction_thresholds": {
                "type": "object"
            },
            "input_data_by_device": {
                "type": "nested",
                "properties": {
                    "key": {
                        "type": "keyword"
                    },
                    "value": {
                        "type": "object"
                    }
                }
            },
            "created": {
                "type": "date",
                "format": "epoch_millis"
            },
            "modified": {
                "type": "date",
                "format": "epoch_millis"
            },
            "correlation_id": {
                "type": "keyword"
            }
        }
    }
}'

echo -e "\n\nElastic Index creation complete!!\n"
