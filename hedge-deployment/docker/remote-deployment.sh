#!/bin/bash

TARGET_FOLDER=/hedge/remote-deployment


#mkdir edge-iot-$(date +"%d-%m-%Y-%M-%S")
#cd edge-iot-$(date +"%d-%m-%Y-%M-%S")
#git clone https://github.bmc.com/CTO-BIL/edge-iot.git
#cd edge-iot

echo "Enter the target machines where the files needs to be copied : " 
read target
TARGET_MACHINE=${target}
echo "Copying latest deployment files to ${TARGET_MACHINE} machine"

scp -r contents root@${TARGET_MACHINE}:${TARGET_FOLDER}
scp -r hedge-docker-services root@${TARGET_MACHINE}:${TARGET_FOLDER}
scp Makefile root@${TARGET_MACHINE}:${TARGET_FOLDER}
scp VERSION root@${TARGET_MACHINE}:${TARGET_FOLDER}

echo "All deployment files are copied to ${TARGET_MACHINE} machine"

