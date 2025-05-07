#!/bin/bash

###################################################
# (1) COPY DOCKER FILE TO THE RIGHT PLACE
###################################################

# Define the file you want to copy
original_file="./cmd/ml-anomaly-inferencing-python/Dockerfile"

# Define the destination folder
destination_folder="./cmd/ml-anomaly-inferencing-python/scripts/docker_inf_test/"

# Define the file name for the copied file
copied_file="$destination_folder$(basename "$original_file")"

# Copy the file to the destination folder
cp "$original_file" "$copied_file"

echo "File copied to $copied_file"

###################################################
# (2) PERFORM FIND AND REPLACE ON DOCKERFILE
# - This is so that it can run locally
###################################################

# Define the search pattern
search="edge-ml-service/cmd/ml-anomaly-inferencing-python/"

search2="edge-ml-service/anomaly-gcp-trng/autoencoderv2/"

# Define the replacement string
replace="./"

# Use awk to perform the find and replace in the copied file
awk -v search="$search" -v replace="$replace" '{gsub(search, replace)} 1' "$copied_file" > tmpfile && mv tmpfile "$copied_file"
awk -v search2="$search2" -v replace2="$replace" '{gsub(search2, replace2)} 1' "$copied_file" > tmpfile && mv tmpfile "$copied_file"

echo "Find and replace operation completed."

###################################################
# (3) MOVE OTHER FILES
###################################################

files_to_modify=(
    "execute_inf.sh"
    "main.py"
    "requirements.txt"
    "validations.py"
)

for file in "${files_to_modify[@]}"; do
    cp -r "./cmd/ml-anomaly-inferencing-python/$file" "./cmd/ml-anomaly-inferencing-python/scripts/docker_inf_test/"
done

cp -r ./anomaly-gcp-trng/autoencoderv2/custom_logger.py ./cmd/ml-anomaly-inferencing-python/scripts/docker_inf_test/
#cd ./docker_inf_test || exit

###################################################
# (4) RUN THE Autoencoder & Inferencing

volume_name="edge-ml-service_autoencoder-volume"

# Check if the volume exists
if docker volume inspect "$volume_name" &> /dev/null; then
    # Attempt to remove the volume
    if docker volume rm "$volume_name"; then
        echo "Volume '$volume_name' removed successfully."
    else
        echo "Failed to remove volume '$volume_name'."
        # You can choose to exit here if you want to stop execution after failure
        # exit 1
    fi
else
    echo "Volume '$volume_name' does not exist."
fi

echo "COMPOSING IMAGE"

docker-compose down --rmi all  && docker-compose build --no-cache && docker-compose up -d

###################################################
# (5) Remove all copied & moved files
###################################################

echo "Deleting Un-needed files"

for file in "${files_to_modify[@]}"; do
    rm -rf "./cmd/ml-anomaly-inferencing-python/scripts/docker_inf_test/$(basename "$file")"
done

rm -rf ./cmd/ml-anomaly-inferencing-python/scripts/docker_inf_test/custom_logger.py

rm -rf ./cmd/ml-anomaly-inferencing-python/scripts/Dockerfile