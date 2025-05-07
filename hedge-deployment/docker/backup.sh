#!/bin/sh

path="/var/lib/docker/volumes/grafana-storage/_data"
target="/grafana/data/"

if [ ! -d $target ]
then
	mkdir -p $target
else
	echo "Target folder already exists"
fi

cp $path/grafana.db $target
echo "grafana.db copied"

cp -R $path/plugins $target
echo "plugins folder copied"

cp -R $path/png $target
echo "png folder copied"

cp -R $path/resources $target
echo "resources folder copied"

