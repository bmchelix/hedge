#Instructions & Guidelines to build custom ui panels(if neeeded) and deploy them along with dashboards
> config: Default grafana.ini file to be copied to conf dir of grafana installation
> resources: images that need to be copied to /usr/share/grafana/public/img/plugins
> dashboards that will be referred from provisioning/dashboard.yaml
> provisioning: Overwrite grafana's provisioning directory with this for self installation of dashboards and datasources


# Hedge Grafana

This repo builds Grafana docker image packaged together for Hedge IoT use cases.
The custom plugins have been removed from the product, however, follow instructions as per grafana documentation to build the plugins if needed


### For UI Panel
1. Install dependencies
```BASH
yarn install
```
2. Build plugin in development mode or run in watch mode
```BASH
yarn dev
```
or
```BASH
yarn watch
```
3. Build plugin in production mode
```BASH
yarn build
```

