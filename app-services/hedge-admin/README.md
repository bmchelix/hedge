## content-deployment
    Hedge content deployment service is edgex app service which exposes an api to import contents for demo machines. In which the contents includes rules, nodered workflows and elastic seed data.

    1. To import the contents to target machine below is the curl command to execute. Below curl command is the post call which accepts details like nodetype,contentDir and targetMachine as the parameters if we didn't specify targetMachine parameter it will automatically take the current system logical names of each service.

        curl --location --request POST 'http://localhost:48098/api/v1/content/import' \
        --header 'Content-Type: application/json' \
        --data-raw '{
        "nodeType":"edge",
        "contentDir":["windTurbine","telco"]
        }'
    
    2. config parameter ApplicationSettings.ContentDir present in configuration.toml is the directory we need to provide which needs to be the same directory structure as "/edge-iot/contents" at least for rules, node-red and elasticsearch folders.

    3. "content-deploy.sh" is the shell script file at "/edge-iot/contents/" location, which has the curl command to deploy contents. which can be executed using "make content" command also.
