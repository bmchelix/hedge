# edge-iot 
## BMC Helix Edge IoT Platform


BMC Helix Edge IoT Platform is a new offering from BMC that extends its reach to the edge and into IoT space.
It provides a set of core services to manage devices and the real-time data collection. To collect data from devices, some of the protocol specific services(eg MQTT) is built into the platform. For others, SDK is provided to create customer services to collect data.
There is built-in rules engine and Anomaly detection as well as dashboarding capabilities in the platform.
A built-in integration with BMC Helix Operations Manager and IT Service Management is also in place. The following contains guidance for developers.


## How to set up local Development Environment
**Prerequisites**
- Go (Min verson 1.24.2)
- IDE: Goland (Preferred IDE)
- Redis
- Set the environment variable EDGEX_SECURITY_SECRET_STORE=false so it is available in IDE (For Mac add this to .zshrc)
- With new version of edgex 3.1.0 onwards, there is a common configuration that needs to be loaded. You can set the environment overide as below
  EDGEX_COMMON_CONFIG={edgex-go-dir}/cmd/core-common-config-bootstrapper/res/configuration.yaml
- To ensure TLS cipher compatibility in Go v1.23, set the environment variable GODEBUG=tlsrsakex=1 so it is available in your IDE. (For Mac users, you can add this line to your .zshrc file to apply it globally)
- Install pkg-config ( for Mac, $brew install pkg-config)


## 1. Set up Redis server locally
* For Windows installation set up [wsl]([https://learn.microsoft.com/en-us/windows/wsl/install) and then follow [Redis instructions](https://redis.com/blog/install-redis-windows-11/)

## 2. Set up Edgex
Following are the steps to set up edgex environment. Make sure the checkout the edgexfoundry/edgex-go with version v3.1.0
1. Clone this Github repo on your machine. As a best practice, 
   create a workdirectory called hedge under which carry out the below steps 
    > git clone https://github.com/edgexfoundry/edgex-go
2. goto edgex-go directory
3. checkout the supported version of edgex-go for edge-iot
    > git checkout v3.1.0
4. Make sure that redis-server is running 
5. We only need to run (core-metadata & core-commands) on development environment, 
   so from the IDE. For each of the services in here, right click on cmd/main.go 
   and select create Debug or Run configuration. Thereafter, edit it to rename the configuration 
   and correct the work-directory to a path where main.go is located. 
   An example is attached in here

![Sample Golang Debug/Run Configuration](images/golangDebugConfigEg.png?raw=true "Debug Configuration")

## 3. Set up EDGE-IOT project 
1. Clone this Github repo on your machine 
    > git clone https://github.bmc.com/CTO-BIL/edge-iot.git 
   let this be outside of edgex setup, but under a common root folder eg hedge)
2. goto edge-iot directory
3. Ensure you download goland dependencies by running
   > go mod download all
4. In most cases, you might need to have a device service running so it registers the device and 
   generates the data
5. First check res/configiration.yaml so it refers to localhost, not logical names 
   that is required in docker containers 
6. Start with device-services/hedge-virtual-device, right-click cmd/main.go and click Run, 
   edit the working directory so it points to where main.go is
7. If the above is running, your dev environment is all setup
8. Some of the services depend on MQTT, you can refer to mqtt broker ( mosquitto) of your 
   shared dev environment. Same for database like victoria and elastic. 
   The current dev env is clm-pun-vvriyq
9. If you are working on Hedge UI, goto ui/admin and look for the corresponding documents in there

## Hedge Development Resources
Below are some of the relevant resources to get started on Hedge
1. If you are new to Hedge, here is where you can refer to get a good overview of edgex foundry 
   which is the underlying software, hedge uses(without modification):
> https://www.edgexfoundry.org/
> https://docs.edgexfoundry.org/3.1/getting-started/Ch-GettingStartedGoDevelopers/

2. To get to specific service API documentation, navigate to the specific service first and then 
   look for their Swagger documentation
> https://docs.edgexfoundry.org/3.1/api/core/Ch-APICoreMetadata/

2. If you are developing a north-bound service, here is the SDK reference to get started
> https://docs.edgexfoundry.org/3.1/microservices/application/ApplicationServices/

3. If you are developing a device-service to pull data from propriety protocol, here is the device SDK to get you started
> https://docs.edgexfoundry.org/3.1/microservices/device/DeviceService/

4. If you are working to secure the microservices using a secure gateway, there is where you can get started.
   edgex foundry is bundled with nginx gateway & with built-in protection using jwt token
> https://docs.edgexfoundry.org/3.1/security/Ch-APIGateway/

        
## Setting up Build Environment
**Prerequisites**
- Build Machine: The current build machine is **vl-pun-dombl145** ( root/#D3Pl0y_M3nT$)
  To make it simple, add the following alias in .zshrc or .bashrc
  >> alias hedge-build-server='sshpass -p "#D3Pl0y_M3nT$" ssh root@vl-pun-dombl145.bmc.com'

  The source code is at ~root/edge-iot, Always do a get latest ( git pull origin master) before the build

The below setup is required when first time setting up a build machine 
- Go
- Docker
- Docker-compose

**Build Steps**
1. Clone this Github repo on your machine 
    > git clone https://github.bmc.com/CTO-BIL/edge-iot.git
2. goto edge-iot directory
3. When it is a brand new machine etc, run the following from edge-iot directory where go.mod is located
    > go mode download all
4. Run a clean build of all the micro services.
    > $make clean build

    App binaries will be available inside respective microservice folders with same name as that of the microservice

    **NOTE: There may be special config instructions for individual services, make sure to check out the individual Readme. If you see issues building ml-inferencing since it needs tensorflow libraries for that env, you might want to comment that part, docker build for this still works**

5. **For Mac M1 Silicon only**: set another ENV value instead of default 'jenkins' in edge-iot/Makefile - for creating suitable images suitable for this specific platform (will pull images from the default source - Docker Hub):
    > ENV ?= jenkins

6. To build docker images add _docker_ as a target in above make command, or run a new command
    > _'make docker'_   
      or, _'make clean build docker'_   

7. To build a specific docker image, you can refer to Makefile for specific microservice name and run the make command
> eg make hedge_ui_server --> microservice name with underscore

## Push the images to docker registry
If you want to push specific image that is built to the registry, execute the below command. Note that the image name is not a fully-qualified image name
> make myimage=<ImageName> push-myimage

> eg, $make myimage=hedge-node-red push-myimage

You might get an error if you are not logged in to the registry in which case, login using the below
> docker login pun-harbor-reg1.bmc.com/iot/ (Use your adprod id and pwd)

To push all hedge images, run the below command
> make push

To push all edgex-foundry images
> make push-edgex-foundry

To push hedge infrastructure ( elastic, victoria, mosquitto, node-red), execute
> make push-hedge-infra

## Deployment
For deployment Makefile and instructions, refer to Readme.md under hedge-docker-services/hedge-docker-compose

## Troubleshooting
1. If you face build issues like below, then install zeromq for your platform from: https://zeromq.org/download/ 
    > pkg-config --cflags  -- libzmq
      pkg-config: exec: "pkg-config": executable file not found in $PATH
2. If you get such a build error, then it could be due to docker network bridge error. Try restarting the docker service
    > fetch http://dl-cdn.alpinelinux.org/alpine/v3.12/main/x86_64/APKINDEX.tar.gz
      fetch http://dl-cdn.alpinelinux.org/alpine/v3.12/community/x86_64/APKINDEX.tar.gz
      ERROR: http://dl-cdn.alpinelinux.org/alpine/v3.12/main: temporary error (try again later)
      WARNING: Ignoring APKINDEX.2c4ac24e.tar.gz: No such file or directory
      ERROR: http://dl-cdn.alpinelinux.org/alpine/v3.12/community: temporary error (try again later)
      WARNING: Ignoring APKINDEX.40a3604f.tar.gz: No such file or directory
3. If your VM goes offline (loses network connectivity) on starting up the edgeX stack (docker-compose ** up), it may be due to docker bridge network. Follow the below steps to resolve the docker bridge network issue.

        1. Uninstall docker completely from the host.
        2. Re-install Docker on VM
        3. Create a daemon.json file under /etc/docker/ with the below contents 
    
        {
            "bip": "10.104.0.1/24",
            "fixed-cidr": "10.104.0.0/24"
        }
    
        4.	Start/Restart docker service/daemon using either 'sudo systemctl start docker' or 'sudo service docker start'.
        5.	Verify if the docker0 bridge IP is changed to 10.104.0.1 from the default docker IP 172.17.0.1 using the 'ifconfig' command.
 
 **Note:**
If docker0 IP still does not change, you have to stop and start the docker services again using either of the below commands: \
        > **sudo service docker stop** \
        > **sudo systemctl stop docker** \
\
Execute the **ifconfig** command again. The IP address of 'docker0' bridge should reflect the IP address specified in the daemon.json file. \
\
The above procedure works fine if the docker containers connect to the default 'docker0' bridge. All docker containers that connect to the default docker0 bridge will \     have the IP address in the range 10.104.0.x. \
\
If you create your own custom docker network and add containers to the custom docker network, these containers will again take the IP address in the range **172.17.x.x** \ 
and not in the range **10.104.0.x**, since the containers are now connected to the custom network and not to the default docker0 network (which is configured to IP range \ **10.104.0.x**.  This can again cause issues with network connectivity and your VM will go offline.  You would need IT helpdesk to bring the system back online. \
\
To avoid this, you will have to configure your custom network to also take the IP in the range **10.104.1.x** range. To do this follow the steps below:

    1. Uninstall docker completely from the host.
    2. Re-install Docker on VM
    3. Create a daemon.json file under /etc/docker/ with the below contents 
    {
        "bip": "10.104.0.1/24",
        "fixed-cidr": "10.104.0.0/24",
        "default-address-pools":
        [
            {"base":"10.104.1.0/16","size":24}
        ]
     }

    4.  Start/Restart docker service/daemon using either 'sudo systemctl start docker' or 'sudo service docker start'.
    5.  Verify if the docker0 bridge IP is changed to 10.104.0.1 from the default docker IP 172.17.0.1 using the 'ifconfig' command.
    6.  Verify if the custom docker network IP is changed to 10.104.1.x.
    
    
# Jenkins Daily Builds
**-Note: This is still in pause mode, so need to be revived -**

Jenkins Daily Builds has been configured on 'clm-pun-ulnw8l' machine. The name of the Jenkins job is 'Daily_Checkin_Builds'.  This Jenkins job will be triggered for every code check-in that happens in GIT.  The url to access Jenkins is (http://clm-pun-ulnw8l:8080). \
\
**-Note:** Jenkins is configured using the Master – Slave concept.  Here 'clm-pun-ulnw8l' is the master and 'clm-pun-ukwsxx' is the slave machine.\
\
At the heart of the Jenkins job is the Jenkinsfile. This is a Jenkins pipeline job which contains the following stages:\
**-Start:** This stage sends an email to the configured recipients informing them that the Jenkins build has started. It stops the containers (if any), deletes the existing hedge-images on the build server (clm-pun-ulnw8l) before it starts to build the new images.\
\
**-Git Checkout:** This stage clones the latest code checked into the master branch in GIT, into the Jenkins workspace. \
\
**-Build:** This stage builds the binaries for each of the microservices. It triggers the command 'make build'. This make command invokes the 'build' target specified in the Makefile.\
\
**-Test:** This stage is in-place for unit testing. Currently there are no unit testing scripts available.\
\
**-Create Docker Images:** This stage creates the docker images of each of the hedge microservices. It triggers the command 'make docker'. This command invokes the 'docker' target specified in the Makefile.\
\
<br />
The 'post' section is configured for the below sections:\
\
**-Always:** This section will be executed always irrespective of whether the Jenkins job is successful or not. Here we clean up all the generated binaries for each of the hedge microservices that were created in the 'Build' stage.  It triggers the command 'make clean'. This command invokes the “clean” target specified in the Makefile.\
\
**-Successful:** This section will be executed if the Jenkins build was successful. This section is configured to push the new docker images to Harbor. It triggers the command 'make push'. This command invokes the 'push' target specified in the Makefile. It also sends out an email to all the configured recipients informing them about the build status.\
\
**-Failure:** This section will be executed if the Jenkins build fails. This section is configured to send out an email to all the configured recipients informing them about the build status. 

**Note:**
    There are 2 Jenkins jobs configured for Hedge IOT
        a. Daily_Checkin_Builds - This job runs every time a checkin is done in GIT.
        b. Hedge_Nightly_Build - This job is scheduled to run at 7pm IST everyday. This job is currently disabled. To enable it, click on the Jenkins job, click Configure on              the left menu options, under Build Triggers, select the "Build Periodically" check box and enter the CORN job "H 19 * * *".
    
        
#### **-To configure all the required credentials in Jenkins:**
    1.	Go to Dashboard->Manage Jenkins->Manage Credentials. Specify all the credentials that Jenkins would required to connect to the respective systems like GIT, Harbor,             Hedge servers etc, and select the appropriate credentials where ever required.
    2.	Under Stores scoped to Jenkins-> Click Jenkins link-> Click Global Credentials(unrestricted) link >Click Add Credentials link.
    3.	Specify the username, password, ID and description.
    4.	Click OK. 
    
#### **-To configure Jenkins to connect to GIT:**
    1.	For Jenkins to pull the latest source code from Git into Jenkins workspace, Jenkins would need the GIT credentials to connect to GIT.
    2.	Select a Jenkins job, click Configure. Under Build Triggers, Pipeline (for Pipeline jobs), select the below options:
            Definition: Pipeline script from SCM
            SCM: GIT
            Repositories: Specify the repository url
            Credentials: Select the required git credentials which is required for Jenkins to connect to GIT.
            Branches to build: Specify the branch which needs to be checkout.
            Additional Behaviours: Select the option “Wipe out repository and force clone”.
            Script: Specify the path to the Jenkinsfile in GIT.
    3.	Click Save.

# Sonar Qube Server Details
     
##Sonar Qube Server 
     Login URL - TBD
     Credentials - 


## Python Unit Tests

Python unit tests are running using prebuilt docker image to improve performance. When `edge-ml-service/python-code/tests/requirements.txt` is updated with new dependencies,
the image needs to be rebuilt. Follow the steps:

1. Update  `edge-ml-service/python-code/tests/requirements.txt` with new dependencies.
NOTE: It is important to use canonical names for python libraries. Pip is case-insensitive and also doesn't discern between underscore and minus sign.
2. Rebuild and push the image to registry by running `make push-python-test-coverage-base` for `edge-iot` project root folder.
NOTE: This might take about 20 mins or even more.
3. Push your changes to GitHub, create a PR and merge.
MOTE: If you create a PR before step 2 is complete python unit tests might fail because the image hasn't been pushed to registry yet.

## Swagger Documentation

Hedge Public API can be used with Swagger. Read this [page](IT-Operations-Management.Operations-Management.EDGEMASTER.Developing.Accessing-API-using-Swagger-UI.WebHome) to find out more.   
Current implementation use _swag_ generator by _swaggo_. Learn about _swaggo/swag_ [here](https://github.com/swaggo/swag?tab=readme-ov-file). 
We have several guidelines and hints for composing swagger documentation(comments).
1. Put comments over the method that only contain exactly one route definition
2. Such method should be named as `addRouteRest<...>`
3. Pay attention to `@Router` url and HTTP method (HTTP method should be in square brackets)
4. It's highly recommended to run Swagger specification file generation every time documentation editing is completed

For the reference check out `.../app-services/hedge-device-extensions/internal/router/router.go` file: 
```go
// @Summary		Update Device Extensions
// @Tags	Hedge Device Extensions - Device Extensions
// @Produce	json
// @Accept	json
// @Param			deviceName	path		string				true	"device name"
// @Param 			q 			body 		[]dto.DeviceExt 	true 	"List of the device extensions objects"
// @Success			200			{array}		[]dto.DeviceExtension 	""
// @Success			200			{array}		[]string 	"empty array"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router			/api/v3/metadata/device/extn/{deviceName} [put]
func (r Router) addRouteRestUpdateDeviceExtn() {
	...
}
```  
<br/>

#### Running Swagger specification file generation 
There are several ways to generate Swagger specification file. After completing adding/editing/removing _swag_ comments over REST API method you can either:
- Run `make generate-swagger-spec` from the project root folder
- Run in the terminal from project root folder `swag init --parseInternal=true --generalInfo=doc.go --pd=true --ot=json --output=./swagger-ui/res/swagger/`
- If in IDE, go to `doc.go` and run `go:generate` command 

You can start `swagger-ui` service in order to check generated file. Before commiting the changes to the Git, be sure to 
regenerate swagger spec file (since swagger-ui restructures swagger.json file in the runtime).

#### Swagger General Info
General info tags for Swagger (general description, license info, base path, etc.) is located in `doc.go`


