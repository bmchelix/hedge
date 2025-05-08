#!/bin/sh
set -e
set -x

input_zip=$1
output_zip=$2

# Create a temporary directory in a writable location
temp_dir=$(mktemp -d /edgex-init/clean_zip.XXXXXX)

if [ -z "$temp_dir" ]; then
    echo "Failed to create a temporary directory"
    exit 104 # Exit code for temporary directory creation failure
fi

echo "Cleaning zip file: $input_zip"
echo "Output zip file will be: $output_zip"
echo "Temporary directory is: $temp_dir"

# Ensure the temp directory has appropriate permissions
chmod -R 755 "$temp_dir"

# Attempt to unzip the input file
unzip -q "$input_zip" -d "$temp_dir" 2>/dev/null
if [ $? -ne 0 ]; then
    echo "Unzip failed: Cannot unzip the file or permissions issue."
    ls -la "$temp_dir"
    rm -rf "$temp_dir"
    exit 100 # Exit code 100 for unzip failure
fi

# Remove unwanted files and directories
find "$temp_dir" -name "__MACOSX" -exec rm -rf {} +
find "$temp_dir" -name "._*" -exec rm -f {} +

# Ensure all files are writable (in case of permission restrictions)
find "$temp_dir" -type f -exec chmod 644 {} \; || echo "Failed to chmod files, but continuing..."
find "$temp_dir" -type d -exec chmod 755 {} \; || echo "Failed to chmod directories, but continuing..."

# Attempt to clean extended attributes (gracefully handle failures)
find "$temp_dir" -type f -exec xattr -c {} \; || echo "Warning: Failed to clean extended attributes"

# Attempt to create a new zip file
cd "$temp_dir" || exit 101 # Exit code 101 for directory navigation failure
zip -qr "$output_zip" . 2>/dev/null
if [ $? -ne 0 ]; then
    echo "Zip failed: Cannot re-create the zip file."
    rm -rf "$temp_dir"
    exit 102 # Exit code 102 for zip failure
fi

echo "Re-zipped cleaned content to $output_zip"

# Cleanup temporary directory
rm -rf "$temp_dir"
if [ $? -ne 0 ]; then
    echo "Failed to clean up temporary directory: $temp_dir"
    exit 103 # Exit code 103 for cleanup failure
fi

exit 0