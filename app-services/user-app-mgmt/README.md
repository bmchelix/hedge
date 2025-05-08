# Helix IoT Edge User Application Management

User Application Management provides the APIs to support User & Role Management as well as how application menu shows up based on the user's role

## Build and Run the app with Secure Edgex services

**Steps:**

1. From the `user-app-mgmt` folder under edge-iot run:

   ```console
   make build
   ```

2. Down load default EdgeX secure compose file 

   Copy to https://github.com/edgexfoundry/edgex-compose/blob/ireland/docker-compose.yml to local folder.

3. Modify compose file so this `Secrets`example service has access to Vault for it's SecretStore 

   Add the service's service-key, `hedgedb`, to the `secretstore-setup` service's `ADD_SECRETSTORE_TOKENS` environment variable as shown below:

   ```yaml
     secretstore-setup:
       container_name: edgex-security-secretstore-setup
       depends_on:
       - security-bootstrapper
       - vault
       environment:
         ADD_SECRETSTORE_TOKENS: 'hedgedb'
   ```

   This creates a Vault back SecretStore for our service and populates it with then known `hedgedb` secret. In addition creating the SecretStore, service's can request known secrets and to be added to the API Gateway. See the [Configuring Add-on Service](https://docs.edgexfoundry.org/2.0/security/Ch-Configuring-Add-On-Services/) security documentation for complete details.

5. Run the `user-app-mgmt` service as root

   The service must run as root so that it can access it's SecretStore token. Also the environment variable `EDGEX_SECURITY_SECRET_STORE` must not be set to `false`. Either not set at all or set to `true`

   ```console
   sudo EDGEX_SECURITY_SECRET_STORE=true ./app-service
   ```

6. Follow the instructions in [Storing and Getting Secrets](#storing-and-getting-secrets) in order to test storing and retrieving secrets from the secret store.

## Storing and Getting Secrets

These tests use a collection of Postman requests, in *SecretsExample.postman_collection.json*, to store and retrieve secrets from the secret store.

1. Import the collection *SecretsExample.postman_collection.json* into Postman.

2. Execute the `Store Secrets` request in the Postman collection to push secrets to Vault. This is going through the App Service REST API, not directly to Vault. As such, the secret is exclusively for that app service instance.

3. Execute the `Get Secrets with App Service HTTP` request in the Postman collection.

   This request triggers an EdgeX event to the application service which causes execution of the pipeline function that calls the GetSecrets API.  As a result, the app service will get the exclusive secrets that were just pushed.

4. View the service's logs to verify that the secrets were retrieved. We'll view the secrets in the application's console (in production, NEVER log your application's secrets. This is done in the example service to demonstrate the functionality).

   ```console
   level=INFO ts=2021-07-19T21:29:44.2351952Z app=app-secrets source=getsecrets.go:52 msg="--- Get secrets at location /hedgedb, keys: []  ---"
   level=INFO ts=2021-07-19T21:29:44.2398679Z app=app-secrets source=getsecrets.go:59 msg="key:username, value:app-user"
   level=INFO ts=2021-07-19T21:29:44.2399432Z app=app-secrets source=getsecrets.go:59 msg="key:password, value:SuperDuperSecretPassword"
   level=INFO ts=2021-07-19T21:29:44.2399976Z app=app-secrets source=getsecrets.go:52 msg="--- Get secrets at location /hedgedb, keys: [password]  ---"
   level=INFO ts=2021-07-19T21:29:44.2400476Z app=app-secrets source=getsecrets.go:59 msg="key:password, value:SuperDuperSecretPassword"
   ```

   
