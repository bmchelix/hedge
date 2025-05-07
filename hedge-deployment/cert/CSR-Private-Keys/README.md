Steps for requesting external CA Signed SSL Certificate.

1. Create CSR file using following command

> EX: openssl req -out xxx.csr -newkey rsa:2048 -nodes -keyout xxx.key -config custom_cert.cnf -days 365 -sha256

2. Validate the CSR inputs using following tool.

    https://ssltools.digicert.com/checker/views/csrChaeck.jsp

3. You might need to send the CSR file to your company's NOC or to the group administering the certificates for your company for any action that they might need to take.

4. Noc team will provide the certificate download link.

5. Follow the respective steps to apply the certificates in Hedge Core.

**Applying domain certificate on hedge-core**

1. ssh to hedge-core instance.
2. Copy xxx.pem & xxx.key to the location /var/lib/docker/volumes/edgex_nginx-tls/_data
3. Add the certificate .pem & .key path in hedge-reverse-proxy.conf file located at /var/lib/docker/volumes/edgex_nginx-config/_data/conf.d (take a backup of original file)
Under the second server block with listen 8443 ssl change ssl_certificate and ssl_certificate_key as below:

>> ssl_certificate "/etc/ssl/nginx/xxx.pem";
>> ssl_certificate_key "/etc/ssl/nginx/xxx.key";
> 

6. Restart nginx: docker restart edgex-nginx. Or exec into nginx container and run nginx -s reload


7. To apply the certificate on GCP Cloud.

          i) Login to GCP cloud.
          ii) Goto Security-Certificate Manager, select CLASSIC CERTIFICATES tab. Upload .pem & .key.
          iii) Edit LB & apply new certificate.Verify the new certificates by logging in to Hedge UI.
                    

