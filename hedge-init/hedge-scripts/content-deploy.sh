#!/bin/sh

echo -e "Waiting for Hedge Admin to be up";
# 'getent' for DNS check, 'nc' for port availability check
while ! ( getent hosts hedge-admin > /dev/null &&
          nc -z hedge-admin 48098 );
do sleep 5;
  echo -e "still waiting for Hedge Admin...";
done;
echo -e "  >> Hedge Admin has started. Continuing ahead..";

# Hedge admin will makes calls to Kuiper, Nodered and ES for content deployment
echo -e "Waiting for Edgex Kuiper to be up";
while ! ( getent hosts edgex-kuiper > /dev/null &&
          nc -z edgex-kuiper 59720 );
do sleep 5;
  echo -e "still waiting for Edgex Kuiper...";
done;
echo -e "  >> Edgex Kuiper has started. Continuing ahead..";

echo -e "Waiting for Hedge Nodered to be up";
while ! ( getent hosts hedge-node-red > /dev/null &&
          nc -z hedge-node-red 1880 );
do sleep 5;
  echo -e "still waiting for Hedge Nodered...";
done;
echo -e "  >> Hedge Nodered has started. Continuing ahead..";

# Only if COMPOSE_PROFILES contains one of the valid ES profile names: all/es/demo, AND its a CORE setup, AND has local persistence enabled
if [[ "$COMPOSE_PROFILES" == *"all"* || "$COMPOSE_PROFILES" == *"es"* || "$COMPOSE_PROFILES" == *"demo"* ]] &&
   [[ "$CURRENT_HEDGE_NODE_TYPE" == *"CORE"* ]]; then
  echo -e "Waiting for Hedge Elasticsearch to be up";
  while ! ( getent hosts hedge-elasticsearch > /dev/null &&
            nc -z hedge-elasticsearch 9200 );
  do sleep 5;
    echo -e "still waiting for Hedge Elasticsearch...";
  done;
  echo -e "  >> Hedge Elasticsearch has started. Continuing ahead..";
fi
#### Validation ends ####
##########################

# contentDir - contains rules to be imported. Import demo rules only when profile=demo or profile=all
contentDir='[]'
if [[ "$COMPOSE_PROFILES" == *"all"* || "$COMPOSE_PROFILES" == *"demo"* ]]; then
  contentDir='["windTurbine","telco"]'
fi


curl --location --request POST 'http://hedge-admin:48098/api/v3/content/import' \
--header 'Content-Type: application/json' \
--data-raw '{
    "nodeType":"core",
    "contentDir":'"$contentDir"'
}'



curl --location --request POST 'http://hedge-admin:48098/api/v3/content/import' \
--header 'Content-Type: application/json' \
--data-raw '{
    "nodeType":"edge",
    "contentDir":'"$contentDir"'
}'
