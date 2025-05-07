#!/bin/sh

if [ "$#" -lt 1 ]; then
  echo "Usage $0 <edge-node>"
  exit 1
fi

EDGE_NODE_ID=$1
EDGE_NODE=$2
echo "Current Node/Host being added to the DefaultNodeGroup: $EDGE_NODE (nodeid: $EDGE_NODE_ID)"

echo -e "Waiting for Hedge Admin to be up";
# 'getent' for DNS check, 'nc' for port availability check
while ! ( getent hosts hedge-admin > /dev/null &&
          nc -z hedge-admin 48098 );
do sleep 5;
  echo -e "still waiting for Hedge Admin...";
done;
echo -e "  >> Hedge Admin has started. Continuing ahead..";

#### Validation ends ####
##########################

#Node is now registered automatically by hedge-admin
# Create the edge node first
#curl --location --request POST 'http://hedge-admin:48098/api/v2/node_mgmt/node' \
#  --header 'Content-Type: application/json' \
#  --data-raw '{
#      "nodeId": "'"$EDGE_NODE"'",
#      "hostName": "'"$EDGE_NODE"'"
#  }'
#
#echo "Node addition Status: " $?

echo "CURRENT_HEDGE_NODE_TYPE:" "$CURRENT_HEDGE_NODE_TYPE"

# Only for single core-node setup do we need to create the group
if [ "$CURRENT_HEDGE_NODE_TYPE" = "CORE_NODE" ] ; then
    echo "Node groups being created for core/edge management"

    # Create top level container node for the tree
    curl --location --request POST 'http://hedge-admin:48098/api/v3/node_mgmt/group' \
    --header 'Content-Type: application/json' \
    --data-raw '{
        "name": "DefaultNodeGroup",
        "displayName" : "Nodes"
    }'

    # Attach the edge-node to the above container ( as provided in URL) and provide a display name to it
    curl --location --request POST 'http://hedge-admin:48098/api/v3/node_mgmt/group/DefaultNodeGroup' \
    --header 'Content-Type: application/json' \
    --data-raw '{
        "name": "'"$EDGE_NODE"'_Container",
        "displayName": "'"$EDGE_NODE"'",
        "node": {
            "nodeId": "'"$EDGE_NODE_ID"'"
        }
    }'
fi
